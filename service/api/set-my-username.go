package api

import (
	"errors"
	"net/http"

	"github.com/MercuriLorenzo/WASAText/service/api/reqcontext"
	"github.com/MercuriLorenzo/WASAText/service/database"
	"github.com/MercuriLorenzo/WASAText/service/schemas"
	"github.com/julienschmidt/httprouter"
)

// setMyUserName updates the username of the authenticated user
func (rt *_router) setMyUserName(w http.ResponseWriter, r *http.Request, ps httprouter.Params, ctx reqcontext.RequestContext) {
	// Get the userId that authenticate injected in the request
	userId, ok := userIdFromContext(r)
	if !ok {
		writeError(w, ctx, http.StatusUnauthorized, "user not authenticated", nil)
		return
	}

	// Parse and check the request body
	var req schemas.UsernameRequest
	if err := decodeAndValidate(r, &req); err != nil {
		writeError(w, ctx, http.StatusBadRequest, "invalid request body", err)
		return
	}

	// Query
	user, err := rt.db.SetMyUserName(userId, req.Username)
	if err != nil {
		// A username belongs to one user only: asking for a used one is a bad request
		if errors.Is(err, database.ErrUsernameTaken) {
			writeError(w, ctx, http.StatusBadRequest, "username already taken", err)
			return
		}

		writeError(w, ctx, http.StatusInternalServerError, "cannot update the username", err)
		return
	}

	// Response
	writeJSON(w, ctx, http.StatusOK, user)
}
