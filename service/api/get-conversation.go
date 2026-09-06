package api

import (
	"errors"
	"net/http"

	"github.com/MercuriLorenzo/WASAText/service/api/reqcontext"
	"github.com/MercuriLorenzo/WASAText/service/database"
	"github.com/MercuriLorenzo/WASAText/service/schemas"
	"github.com/julienschmidt/httprouter"
)

// getConversation returns the chat of the given id with its members and its messages, newest first
func (rt *_router) getConversation(w http.ResponseWriter, r *http.Request, ps httprouter.Params, ctx reqcontext.RequestContext) {
	// Get the userId that authenticate injected in the request
	userId, ok := userIdFromContext(r)
	if !ok {
		writeError(w, ctx, http.StatusUnauthorized, "user not authenticated", nil)
		return
	}

	// The chatId travels in the URL and not in a body, so it is checked here:
	// it reaches the database only once it is a well formed id
	// Both kinds answer here: private chat and group
	chatId := schemas.ChatId(ps.ByName("chatId"))
	if err := chatId.IsValid(); err != nil {
		writeError(w, ctx, http.StatusBadRequest, "invalid chat id", err)
		return
	}

	// Query
	chat, err := rt.db.GetConversation(userId, chatId)
	if err != nil {
		// No chat owns that id
		if errors.Is(err, database.ErrChatNotFound) {
			writeError(w, ctx, http.StatusNotFound, "chat not found", nil)
			return
		}
		// The chat is there, but reading it belongs to its members
		if errors.Is(err, database.ErrNotAMember) {
			writeError(w, ctx, http.StatusForbidden, "not a member of the chat", nil)
			return
		}

		writeError(w, ctx, http.StatusInternalServerError, "cannot get the conversation", err)
		return
	}

	// Response
	// The base is embedded, so the photo id it carries is turned into a URL through it
	chat.ChatBase = withChatPhotoURL(chat.ChatBase)
	// The messages carry one photo each at most, and the ones without keep the field empty
	chat.Messages = withMessagePhotoURLs(chat.Messages)
	writeJSON(w, ctx, http.StatusOK, chat)
}
