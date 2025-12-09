package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/MercuriLorenzo/WASAText/service/api/reqcontext"
	"github.com/MercuriLorenzo/WASAText/service/schemas"
	"github.com/gofrs/uuid"
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
		token := strings.TrimPrefix(authHeader, "Bearer ")

		// checks if the token is a valid UUID
		if _, err := uuid.FromString(token); err != nil {
			ctx.Logger.WithError(err).Error("invalid token format")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		userId := schemas.UserId(token)

		// check if the user exists in the database
		exists, err := rt.db.UserExists(userId)
		if err != nil {
			// error in database check
			ctx.Logger.WithError(err).Error("database user search failed during authentication")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if !exists {
			// user not found
			ctx.Logger.Warnf("user %s not found during authentication", userId)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// add userID to the context
		newCtx := context.WithValue(r.Context(), keyUserId, userId)

		// call the next handler with the new context
		next(w, r.WithContext(newCtx), ps, ctx)
	}
}
