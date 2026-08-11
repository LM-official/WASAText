package api

import (
	"net/http"

	"github.com/MercuriLorenzo/WASAText/service/api/reqcontext"
	"github.com/MercuriLorenzo/WASAText/service/schemas"
	"github.com/julienschmidt/httprouter"
)

// getUsers returns the users whose username starts with the `username` query parameter
func (rt *_router) getUsers(w http.ResponseWriter, r *http.Request, ps httprouter.Params, ctx reqcontext.RequestContext) {
	// Get and check the username prefix from the query parameters
	username := schemas.Username(r.URL.Query().Get("username"))
	if err := username.IsValid(); err != nil {
		writeError(w, ctx, http.StatusBadRequest, "invalid username", err)
		return
	}

	// Query
	users, err := rt.db.GetUsers(username)
	if err != nil {
		writeError(w, ctx, http.StatusInternalServerError, "user search failed", err)
		return
	}

	if len(users) == 0 {
		writeError(w, ctx, http.StatusNotFound, "no user matches the given username", nil)
		return
	}

	// Response
	writeJSON(w, ctx, http.StatusOK, struct {
		Users schemas.Users `json:"users"`
	}{Users: users})
}
