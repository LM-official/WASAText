package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/MercuriLorenzo/WASAText/service/api/reqcontext"
	"github.com/MercuriLorenzo/WASAText/service/schemas"
	"github.com/julienschmidt/httprouter"
)

// Private key type, so that no other package can read or overwrite the value stored under it
type contextKey string

const keyUserId contextKey = "UserId"

// Authenticate checks the token of the request and injects the userId in the standard context of the request
// The token is the userId itself: this project is about the API design, not about security
func (rt *_router) authenticate(next httpRouterHandler) httpRouterHandler {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params, ctx reqcontext.RequestContext) {
		// Get the token from the Authorization header
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			writeError(w, ctx, http.StatusUnauthorized, "missing bearer token", nil)
			return
		}
		userId := schemas.UserId(strings.TrimPrefix(authHeader, "Bearer "))
		if err := userId.IsValid(); err != nil {
			// Invalid userId format
			writeError(w, ctx, http.StatusUnauthorized, "invalid token format", err)
			return
		}

		exists, err := rt.db.UsersExist(schemas.Members{userId})
		if err != nil {
			// Error in database check
			writeError(w, ctx, http.StatusInternalServerError, "cannot verify the token", err)
			return
		}
		if !exists {
			// No user owns this token
			writeError(w, ctx, http.StatusUnauthorized, "unknown token", nil)
			return
		}

		// Add the userId to the context, and call the next handler with the new context
		newCtx := context.WithValue(r.Context(), keyUserId, userId)
		next(w, r.WithContext(newCtx), ps, ctx)
	}
}

// userIdFromContext returns the userId that authenticate injected in the request
// The second value is false when the handler was registered without authenticate
func userIdFromContext(r *http.Request) (schemas.UserId, bool) {
	userId, ok := r.Context().Value(keyUserId).(schemas.UserId)
	return userId, ok
}
