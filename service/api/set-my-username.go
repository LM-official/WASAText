package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/MercuriLorenzo/WASAText/service/api/reqcontext"
	"github.com/MercuriLorenzo/WASAText/service/schemas"
	"github.com/julienschmidt/httprouter"
)

func (rt *_router) setMyUserName(w http.ResponseWriter, r *http.Request, ps httprouter.Params, ctx reqcontext.RequestContext) {
	// get the userId from the context
	userId, ok := r.Context().Value(keyUserId).(schemas.UserId)
	if !ok {
		// user not authenticated
		ctx.Logger.Error("user not authenticated")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// parsing request body
	var req schemas.UsernameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ctx.Logger.WithError(err).Error("bad request body")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := req.IsValid(); err != nil {
		// invalid input
		ctx.Logger.WithError(err).Error("bad request body")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// query
	user, err := rt.db.SetMyUserName(userId, req.Username)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			// UNIQUE constraint violated
			ctx.Logger.WithError(err).Error("database failed to update username: unique constraint violated")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// other error
		ctx.Logger.WithError(err).Error("database failed to update username")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(user)
}
