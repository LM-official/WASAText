package api

import (
	"errors"
	"net/http"

	"github.com/MercuriLorenzo/WASAText/service/api/reqcontext"
	"github.com/MercuriLorenzo/WASAText/service/photos"
	"github.com/MercuriLorenzo/WASAText/service/schemas"
	"github.com/julienschmidt/httprouter"
)

// multipartMemory is how much of an upload is kept in memory:
// the rest goes to a temporarym file of the system, so a big photo does not become a big allocation
const multipartMemory = 8 << 20 // 8 Megabyte

// multipartOverhead is the room left to the multipart framing (boundaries, part headers) on top of the photo itself,
// so that a photo of exactly MaxPhotoBytes still fits in the body
const multipartOverhead = 1 << 20 // 1 Megabyte

// setMyPhoto replaces the profile picture of the authenticated user
func (rt *_router) setMyPhoto(w http.ResponseWriter, r *http.Request, ps httprouter.Params, ctx reqcontext.RequestContext) {
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
	user, oldPhotoId, err := rt.db.SetMyPhoto(userId, newPhotoId)
	if err != nil {
		// The new photo is on disk but no row points at it: drop it instead of leaking a file
		if delErr := rt.photos.Delete(newPhotoId); delErr != nil {
			ctx.Logger.WithError(delErr).Warning("cannot delete the photo of a failed update")
		}

		writeError(w, ctx, http.StatusInternalServerError, "cannot update the photo", err)
		return
	}

	// Response
	writeJSON(w, ctx, http.StatusOK, withPhotoURL(user))

	// The replaced photo is garbage now, unless something else still shows it
	rt.releasePhoto(oldPhotoId, ctx)
}
