package api

import (
	"errors"
	"net/http"

	"github.com/MercuriLorenzo/WASAText/service/api/reqcontext"
	"github.com/MercuriLorenzo/WASAText/service/database"
	"github.com/MercuriLorenzo/WASAText/service/schemas"
	"github.com/julienschmidt/httprouter"
)

// uncommentMessage removes the authenticated user's own reaction from a message
// The user must be a member of the chat named in the URL, the message must actually belong to it
// and the message must have the user comment
func (rt *_router) uncommentMessage(w http.ResponseWriter, r *http.Request, ps httprouter.Params, ctx reqcontext.RequestContext) {
	// Get the userId that authenticate injected in the request
	userId, ok := userIdFromContext(r)
	if !ok {
		writeError(w, ctx, http.StatusUnauthorized, "user not authenticated", nil)
		return
	}

	// The chat travels in the URL and not in a body, so it is checked here:
	// it reaches the database only once it is a well formed id
	// Both kinds answer here: private chat and group
	chatId := schemas.ChatId(ps.ByName("chatId"))
	if err := chatId.IsValid(); err != nil {
		writeError(w, ctx, http.StatusBadRequest, "invalid chat id", err)
		return
	}

	// The message travels in the URL as well: it names what is being un-reacted to
	messageId := schemas.MessageId(ps.ByName("messageId"))
	if err := messageId.IsValid(); err != nil {
		writeError(w, ctx, http.StatusBadRequest, "invalid message id", err)
		return
	}

	// Query
	err := rt.db.UncommentMessage(userId, chatId, messageId)
	if err != nil {
		// No message owns that id
		if errors.Is(err, database.ErrMessageNotFound) {
			writeError(w, ctx, http.StatusNotFound, "message not found", nil)
			return
		}
		// The message exists but not under this chat: nothing answers at this URL either way
		if errors.Is(err, database.ErrChatNotFound) {
			writeError(w, ctx, http.StatusNotFound, "chat not found", nil)
			return
		}
		// The message belongs to this chat, but reacting (or un-reacting) to it belongs to its members
		if errors.Is(err, database.ErrNotAMember) {
			writeError(w, ctx, http.StatusForbidden, "not a member of the chat", nil)
			return
		}
		// The caller never left a comment on this message: there is nothing to remove
		if errors.Is(err, database.ErrCommentNotFound) {
			writeError(w, ctx, http.StatusNotFound, "comment not found", nil)
			return
		}

		writeError(w, ctx, http.StatusInternalServerError, "cannot uncomment the message", err)
		return
	}

	// Response
	// A DELETE answers with no body: this call removes the caller's own comment from the message
	// Whoever reads the message afterwards simply finds one fewer comment in its list
	w.WriteHeader(http.StatusNoContent)
}
