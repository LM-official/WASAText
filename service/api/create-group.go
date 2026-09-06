package api

import (
	"errors"
	"net/http"

	"github.com/MercuriLorenzo/WASAText/service/api/reqcontext"
	"github.com/MercuriLorenzo/WASAText/service/schemas"
	"github.com/julienschmidt/httprouter"
)

// createGroup creates a group whose members are the users of the request plus the authenticated one
func (rt *_router) createGroup(w http.ResponseWriter, r *http.Request, ps httprouter.Params, ctx reqcontext.RequestContext) {
	// Get the userId that authenticate injected in the request
	userId, ok := userIdFromContext(r)
	if !ok {
		writeError(w, ctx, http.StatusUnauthorized, "user not authenticated", nil)
		return
	}

	// A body too large ends here as well: error 400 to everything the client got wrong, size included
	if err := parsePhotoMultipartForm(w, r); err != nil {
		writeError(w, ctx, http.StatusBadRequest, "invalid multipart body", err)
		return
	}
	// Drop the temporary files of the parser
	defer func() { _ = r.MultipartForm.RemoveAll() }()

	// The name and the members travel as a JSON part, the photo as a file part:
	// the name and the members are checked here
	var req schemas.GroupRequest
	if err := unmarshalAndValidate([]byte(r.FormValue("data")), &req); err != nil {
		writeError(w, ctx, http.StatusBadRequest, "invalid data part", err)
		return
	}

	// The request carries the other members only, so the creator cannot be one of them:
	// it would reach chat_members as the same row twice
	// This is the one rule the schema cannot hold: it is about who is asking, not about what was sent
	for _, member := range req.Members {
		if member == userId {
			writeError(w, ctx, http.StatusBadRequest, "the creator is already a member of the group", nil)
			return
		}
	}

	// Every member must already exist
	exist, err := rt.db.UsersExist(req.Members)
	if err != nil {
		writeError(w, ctx, http.StatusInternalServerError, "cannot verify the members", err)
		return
	}
	// Some members do not exist
	if !exist {
		writeError(w, ctx, http.StatusNotFound, "one or more of the members does not exist", nil)
		return
	}

	// The photo is optional: a group without one starts with a default photo,
	// so the chats CHECK still finds a photoId and the group has something to show.
	// A part that is there and broken is still a bad request
	// setGroupPhoto replaces it later, for this group alone: the default is one shared id and never a file this group owns
	file, _, err := r.FormFile("photoFile")
	hasPhoto := err == nil
	if err != nil && !errors.Is(err, http.ErrMissingFile) {
		writeError(w, ctx, http.StatusBadRequest, "invalid photoFile field", err)
		return
	}

	photoId := schemas.DefaultPhotoId
	if hasPhoto {
		defer func() { _ = file.Close() }()

		// Write the file first: an id is worth saving only once the bytes behind it exist
		// Nothing between here and the insert can fail, so a saved photo is either referenced or deleted
		photoId, err = rt.photos.Save(file)
		if err != nil {
			// What the client sent is the problem, not the server
			if isInvalidPhoto(err) {
				writeError(w, ctx, http.StatusBadRequest, "invalid photo", err)
				return
			}

			writeError(w, ctx, http.StatusInternalServerError, "cannot store the photo", err)
			return
		}
	}

	// Query
	chat, err := rt.db.CreateGroup(userId, req.Members, req.Name, photoId)
	if err != nil {
		// The new photo is on disk but no row points at it: drop it instead of leaking a file
		// Only an upload of this request is dropped: the default is shared by every user and every group that never uploaded one,
		// so a failure here must leave it exactly where it is
		if hasPhoto {
			if delErr := rt.photos.Delete(photoId); delErr != nil {
				logWarning(ctx, "cannot delete the photo of a failed group creation", delErr)
			}
		}

		writeError(w, ctx, http.StatusInternalServerError, "cannot create the group", err)
		return
	}

	// Response
	// The base is embedded, so the photo id it carries is turned into a URL through it
	chat.ChatBase = withChatPhotoURL(chat.ChatBase)
	writeJSON(w, ctx, http.StatusCreated, chat)
}
