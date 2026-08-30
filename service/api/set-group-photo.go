package api

import (
	"errors"
	"net/http"

	"github.com/MercuriLorenzo/WASAText/service/api/reqcontext"
	"github.com/MercuriLorenzo/WASAText/service/database"
	"github.com/MercuriLorenzo/WASAText/service/photos"
	"github.com/MercuriLorenzo/WASAText/service/schemas"
	"github.com/julienschmidt/httprouter"
)

// setGroupPhoto replaces the picture of a group the authenticated user is a member of
func (rt *_router) setGroupPhoto(w http.ResponseWriter, r *http.Request, ps httprouter.Params, ctx reqcontext.RequestContext) {
	// Get the userId that authenticate injected in the request
	userId, ok := userIdFromContext(r)
	if !ok {
		writeError(w, ctx, http.StatusUnauthorized, "user not authenticated", nil)
		return
	}

	// The group travels in the URL and not in a body, so it is checked here:
	// it reaches the database only once it is a well formed id
	// ByName returns a string, and an assignment needs one of the two types to be unnamed:
	// string and ChatId are both named, so the conversion is what carries the id across
	groupId := schemas.ChatId(ps.ByName("groupId"))
	if err := groupId.IsValid(); err != nil {
		writeError(w, ctx, http.StatusBadRequest, "invalid group id", err)
		return
	}

	// Bound the body before anything reads it:
	// without this a client could stream any number of bytes and the multipart parser would follow along
	r.Body = http.MaxBytesReader(w, r.Body, schemas.MaxPhotoBytes+multipartOverhead)

	// A body too large ends here as well: error 400 to everything the client got wrong, size included
	if err := r.ParseMultipartForm(multipartMemory); err != nil {
		writeError(w, ctx, http.StatusBadRequest, "invalid multipart body", err)
		return
	}
	// Drop the temporary files of the parser
	defer func() { _ = r.MultipartForm.RemoveAll() }()

	// The photo is the whole request: the group travels in the URL, so there is no other part to read
	file, _, err := r.FormFile("photoFile")
	if err != nil {
		writeError(w, ctx, http.StatusBadRequest, "missing photoFile field", err)
		return
	}
	defer func() { _ = file.Close() }()

	// Write the file first: an id is worth saving only once the bytes behind it exist
	newPhotoId, err := rt.photos.Save(file)
	if err != nil {
		// What the client sent is the problem, not the server
		if errors.Is(err, photos.ErrUnsupportedType) || errors.Is(err, photos.ErrPhotoTooLarge) || errors.Is(err, photos.ErrEmptyPhoto) {
			writeError(w, ctx, http.StatusBadRequest, "invalid photo", err)
			return
		}

		writeError(w, ctx, http.StatusInternalServerError, "cannot store the photo", err)
		return
	}

	// Query
	chat, oldPhotoId, err := rt.db.SetGroupPhoto(userId, groupId, newPhotoId)
	if err != nil {
		// Whatever the failure is, the new photo is on disk and no row points at it: drop it instead of leaking a file
		if delErr := rt.photos.Delete(newPhotoId); delErr != nil {
			logWarning(ctx, "cannot delete the photo of a failed group photo update", delErr)
		}
		// No group owns that id: it may not exist at all, or be a private chat,
		// which borrows its photo from the other member and owns none to update
		if errors.Is(err, database.ErrChatNotFound) {
			writeError(w, ctx, http.StatusNotFound, "group not found", nil)
			return
		}
		// The group is there, but changing its photo belongs to its members
		if errors.Is(err, database.ErrNotAMember) {
			writeError(w, ctx, http.StatusForbidden, "not a member of the group", nil)
			return
		}

		writeError(w, ctx, http.StatusInternalServerError, "cannot update the group photo", err)
		return
	}

	// Response
	writeJSON(w, ctx, http.StatusOK, withChatPhotoURL(chat))

	// The replaced photo is garbage now, unless something else still shows it
	rt.releasePhoto(oldPhotoId, ctx)
}
