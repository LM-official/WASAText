package api

import (
	"net/http"

	"github.com/MercuriLorenzo/WASAText/service/api/reqcontext"
	"github.com/julienschmidt/httprouter"
)

// setMyPhoto replaces the profile picture of the authenticated user
func (rt *_router) setMyPhoto(w http.ResponseWriter, r *http.Request, ps httprouter.Params, ctx reqcontext.RequestContext) {
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
		if isInvalidPhoto(err) {
			writeError(w, ctx, http.StatusBadRequest, "invalid photo", err)
			return
		}

		writeError(w, ctx, http.StatusInternalServerError, "cannot store the photo", err)
		return
	}

	// Query
	user, oldPhotoId, err := rt.db.SetMyPhoto(userId, newPhotoId)
	if err != nil {
		// The new photo is on disk but no row points at it: drop it instead of leaking a file
		if delErr := rt.photos.Delete(newPhotoId); delErr != nil {
			logWarning(ctx, "cannot delete the photo of a failed update", delErr)
		}

		writeError(w, ctx, http.StatusInternalServerError, "cannot update the photo", err)
		return
	}

	// The replaced photo is garbage now, unless something else still shows it
	rt.releasePhoto(oldPhotoId, ctx)

	// Response
	writeJSON(w, ctx, http.StatusOK, withPhotoURL(user))
}
