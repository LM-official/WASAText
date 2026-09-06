package api

import (
	"net/http"

	"github.com/MercuriLorenzo/WASAText/service/api/reqcontext"
	"github.com/MercuriLorenzo/WASAText/service/schemas"
	"github.com/julienschmidt/httprouter"
)

// lookupUsers returns the profiles of the given users, omitting nonexistent ones
func (rt *_router) lookupUsers(w http.ResponseWriter, r *http.Request, ps httprouter.Params, ctx reqcontext.RequestContext) {
	// Only authenticated users may look up profiles; the caller's ID is not a query filter.
	if _, ok := userIdFromContext(r); !ok {
		writeError(w, ctx, http.StatusUnauthorized, "user not authenticated", nil)
		return
	}

	// Parse and validate the body
	var req schemas.LookupUsersRequest
	if err := decodeAndValidate(r, &req); err != nil {
		writeError(w, ctx, http.StatusBadRequest, "invalid request body", err)
		return
	}

	// Query
	users, err := rt.db.LookupUsers(req.Ids)
	if err != nil {
		writeError(w, ctx, http.StatusInternalServerError, "cannot look up users", err)
		return
	}

	// Response
	writeJSON(w, ctx, http.StatusOK, struct {
		Users schemas.Users `json:"users"`
	}{Users: withPhotoURLs(users)})
}
