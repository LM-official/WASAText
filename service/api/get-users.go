package api

import (
	"encoding/json"
	"net/http"

	"github.com/MercuriLorenzo/WASAText/service/api/reqcontext"
	"github.com/MercuriLorenzo/WASAText/service/schemas"
	"github.com/julienschmidt/httprouter"
)

func (rt *_router) getUsers(w http.ResponseWriter, r *http.Request, ps httprouter.Params, ctx reqcontext.RequestContext) {
	// get username from query parameters
	username := schemas.Username(r.URL.Query().Get("username"))
	if err := username.IsValid(); err != nil {
		// invalid username format
		ctx.Logger.WithError(err).Error("bad request")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// query
	users, err := rt.db.GetUsers(username)
	if err != nil {
		// other errors
		ctx.Logger.WithError(err).Error("database error during user search")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if len(users) == 0 {
		// no users found
		ctx.Logger.WithError(err).Error("user not found")
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(struct {
		Users []schemas.User `json:"users"`
	}{Users: users})
}
