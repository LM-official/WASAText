package api

import (
	"net/http"

	"github.com/MercuriLorenzo/WASAText/service/api/reqcontext"
	"github.com/MercuriLorenzo/WASAText/service/schemas"
	"github.com/julienschmidt/httprouter"
)

// DoLogin logs in the user of the given username, registering it first if the username is new
func (rt *_router) doLogin(w http.ResponseWriter, r *http.Request, ps httprouter.Params, ctx reqcontext.RequestContext) {
	// Parse and check the request body
	var req schemas.UsernameRequest
	if err := decodeAndValidate(r, &req); err != nil {
		writeError(w, ctx, http.StatusBadRequest, "invalid request body", err)
		return
	}

	// !uery
	id, found, err := rt.db.DoLogin(req.Username)
	if err != nil {
		writeError(w, ctx, http.StatusInternalServerError, "login failed", err)
		return
	}

	// An existing user is only logged in, a new one is registered first
	code := http.StatusCreated
	if found {
		code = http.StatusOK
	}

	// Response
	writeJSON(w, ctx, code, struct {
		Id schemas.UserId `json:"id"`
	}{Id: id})
}
