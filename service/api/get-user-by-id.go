package api

import (
	"errors"
	"net/http"

	"github.com/MercuriLorenzo/WASAText/service/api/reqcontext"
	"github.com/MercuriLorenzo/WASAText/service/database"
	"github.com/MercuriLorenzo/WASAText/service/schemas"
	"github.com/julienschmidt/httprouter"
)

// getUserById returns the user of the given id
//
// Every other read hands the client ids and not users:
// the sender of a message, the author of a comment and the members of a chat are all a UserId,
// while what the interface has to draw is a username and a photo
func (rt *_router) getUserById(w http.ResponseWriter, r *http.Request, ps httprouter.Params, ctx reqcontext.RequestContext) {
	// Get the userId that authenticate injected in the request
	if _, ok := userIdFromContext(r); !ok {
		writeError(w, ctx, http.StatusUnauthorized, "user not authenticated", nil)
		return
	}

	// The userId travels in the URL and not in a body, so it is checked here:
	// it reaches the database only once it is a well formed id
	userId := schemas.UserId(ps.ByName("userId"))
	if err := userId.IsValid(); err != nil {
		writeError(w, ctx, http.StatusBadRequest, "invalid user id", err)
		return
	}

	// Query
	user, err := rt.db.GetUserById(userId)
	if err != nil {
		// No user owns that id
		if errors.Is(err, database.ErrUserNotFound) {
			writeError(w, ctx, http.StatusNotFound, "user not found", nil)
			return
		}

		writeError(w, ctx, http.StatusInternalServerError, "cannot get the user", err)
		return
	}

	// Response
	writeJSON(w, ctx, http.StatusOK, withPhotoURL(user))
}
