package api

import (
	"errors"
	"net/http"

	"github.com/MercuriLorenzo/WASAText/service/api/reqcontext"
	"github.com/MercuriLorenzo/WASAText/service/photos"
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

	// A group owns its photo and the chats CHECK refuses one without it, so the upload is required
	file, _, err := r.FormFile("photoFile")
	if err != nil {
		writeError(w, ctx, http.StatusBadRequest, "missing photoFile field", err)
		return
	}
	defer func() { _ = file.Close() }()

	// Write the file first: an id is worth saving only once the bytes behind it exist
	// Nothing between here and the insert can fail, so a saved photo is either referenced or deleted
	photoId, err := rt.photos.Save(file)
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
	id, err := rt.db.CreateGroup(userId, req.Members, req.Name, photoId)
	if err != nil {
		// The new photo is on disk but no row points at it: drop it instead of leaking a file
		// It is always an upload of this request, never a photo shared with something else
		if delErr := rt.photos.Delete(photoId); delErr != nil {
			logWarning(ctx, "cannot delete the photo of a failed group creation", delErr)
		}

		writeError(w, ctx, http.StatusInternalServerError, "cannot create the group", err)
		return
	}

	// Response
	writeJSON(w, ctx, http.StatusCreated, struct {
		Id schemas.ChatId `json:"id"`
	}{Id: id})
}
