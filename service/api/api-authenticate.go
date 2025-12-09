package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/MercuriLorenzo/WASAText/service/api/reqcontext"
	"github.com/MercuriLorenzo/WASAText/service/schemas"
	"github.com/julienschmidt/httprouter"
)

// private type for httprouter handler with context
type contextKey string

const keyUserId contextKey = "UserId"

// authenticate verifica il token e lo inietta nel context standard della request
func (rt *_router) authenticate(next httpRouterHandler) httpRouterHandler {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params, ctx reqcontext.RequestContext) {
		// get the token from the Authorization header
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		userId := schemas.UserId(strings.TrimPrefix(authHeader, "Bearer "))

		if err := userId.IsValid(); err != nil {
			// invalid userId format
			ctx.Logger.WithError(err).Error("invalid token format")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		exists, err := rt.db.UserExists(userId)
		if err != nil {
			// error in database check
			ctx.Logger.WithError(err).Error("database user search failed during authentication")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if !exists {
			// user not found
			ctx.Logger.WithError(err).Error("user not found during authentication")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// add userID to the context
		newCtx := context.WithValue(r.Context(), keyUserId, userId)

		// call the next handler with the new context
		next(w, r.WithContext(newCtx), ps, ctx)
	}
}
