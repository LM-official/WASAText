package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/MercuriLorenzo/WASAText/service/api/reqcontext"
	"github.com/MercuriLorenzo/WASAText/service/photos"
	"github.com/MercuriLorenzo/WASAText/service/schemas"
	"github.com/julienschmidt/httprouter"
)

// getPhoto serves the bytes of a photo
// It asks for a token, and any logged in user may read any photo: the same rule getUsers
// follows for the users. Since the reply is behind a token, the frontend cannot render it with
// a plain <img> tag, which sends no Authorization header: it has to fetch the bytes and build
// an object URL out of them
func (rt *_router) getPhoto(w http.ResponseWriter, r *http.Request, ps httprouter.Params, ctx reqcontext.RequestContext) {
	photoId := schemas.PhotoId(ps.ByName("photoId"))

	file, contentType, err := rt.photos.Open(photoId)
	if err != nil {
		if errors.Is(err, photos.ErrInvalidPhotoId) {
			writeError(w, ctx, http.StatusBadRequest, "invalid photo id", err)
			return
		}
		if errors.Is(err, photos.ErrPhotoNotFound) {
			writeError(w, ctx, http.StatusNotFound, "photo not found", nil)
			return
		}

		writeError(w, ctx, http.StatusInternalServerError, "cannot read the photo", err)
		return
	}
	defer func() { _ = file.Close() }()

	// The type is the one read from the bytes
	// nosniff stops the browser from guessing another one,
	// so an uploaded file can never be served as anything but the image it is
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")

	// The bytes of a photo never change:
	// a new upload gets a new id, so this URL, once fetched, never has to be fetched again
	// `private`, not `public`: the reply is behind a token, so only the browser that asked for it may keep a copy
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")

	// ServeContent keeps the Content-Type set above and adds range requests and revalidation
	var modTime time.Time
	if info, err := file.Stat(); err == nil {
		modTime = info.ModTime()
	}
	http.ServeContent(w, r, "", modTime, file)
}
