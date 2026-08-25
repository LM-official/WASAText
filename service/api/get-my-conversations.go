package api

import (
	"net/http"

	"github.com/MercuriLorenzo/WASAText/service/api/reqcontext"
	"github.com/MercuriLorenzo/WASAText/service/schemas"
	"github.com/julienschmidt/httprouter"
)

// getMyConversations returns all the chats (private and groups) the user belongs to, the most recently active first
func (rt *_router) getMyConversations(w http.ResponseWriter, r *http.Request, ps httprouter.Params, ctx reqcontext.RequestContext) {
	// Get the userId that authenticate injected in the request
	userId, ok := userIdFromContext(r)
	if !ok {
		writeError(w, ctx, http.StatusUnauthorized, "user not authenticated", nil)
		return
	}

	// Query
	chats, err := rt.db.GetMyConversations(userId)
	if err != nil {
		writeError(w, ctx, http.StatusInternalServerError, "cannot get my conversations", err)
		return
	}

	if len(chats) == 0 {
		writeError(w, ctx, http.StatusNotFound, "no conversations found", nil)
		return
	}

	// Response
	writeJSON(w, ctx, http.StatusOK, struct {
		Chats schemas.Chats `json:"chats"`
	}{Chats: withChatPhotoURLs(chats)})
}
