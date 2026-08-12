package api

import (
	"github.com/MercuriLorenzo/WASAText/service/api/reqcontext"
	"github.com/MercuriLorenzo/WASAText/service/schemas"
)

// releasePhoto deletes the file of oldPhotoId once no row points at it any more
//
// It must be called only after the commit that dropped the last reference:
// a crash before the call leaves a file nobody points at (harmless garbage),
// a call before the commit would leave a row pointing at a file that is gone (a broken photo)
//
// Nothing here is reported to the client:
// the response has already been written, and a photo that survives is garbage, not a failed request
func (rt *_router) releasePhoto(oldPhotoId schemas.PhotoId, ctx reqcontext.RequestContext) {
	// The default id is shared by every new user, so it has to survive even the moment when no row happens to point at it
	if oldPhotoId == "" || oldPhotoId == schemas.DefaultPhotoId {
		return
	}

	referenced, err := rt.db.PhotoIsReferenced(oldPhotoId)
	if err != nil {
		// Unknown whether the photo is still in use: keeping it
		ctx.Logger.WithError(err).Warning("cannot tell if the photo is still referenced")
		return
	}
	if referenced {
		// Another row still shows this photo
		return
	}

	if err := rt.photos.Delete(oldPhotoId); err != nil {
		ctx.Logger.WithError(err).Warning("cannot delete the unreferenced photo")
	}
}
