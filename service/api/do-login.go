package api

import (
	"encoding/json"
	"net/http"

	"github.com/MercuriLorenzo/WASAText/service/api/reqcontext"
	"github.com/MercuriLorenzo/WASAText/service/schemas"
	"github.com/julienschmidt/httprouter"
)

func (rt *_router) doLogin(w http.ResponseWriter, r *http.Request, ps httprouter.Params, ctx reqcontext.RequestContext) {
	// parse request body
	var req schemas.UsernameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ctx.Logger.WithError(err).Error("bad request body")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// validate input
	if err := req.IsValid(); err != nil {
		ctx.Logger.WithError(err).Error("bad request body")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// query
	id, found, err := rt.db.DoLogin(req.Username)
	if err != nil {
		ctx.Logger.WithError(err).Error("login failed")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// response
	w.Header().Set("Content-Type", "application/json")

	if found {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusCreated)
	}

	_ = json.NewEncoder(w).Encode(id)
}
