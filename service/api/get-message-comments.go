package api

import (
	"errors"
	"net/http"

	"github.com/MercuriLorenzo/WASAText/service/api/reqcontext"
	"github.com/MercuriLorenzo/WASAText/service/database"
	"github.com/MercuriLorenzo/WASAText/service/schemas"
	"github.com/julienschmidt/httprouter"
)

// getMessageComments returns every reaction left on a message
// The user must be a member of the chat named in the URL, and the message must actually belong to it
func (rt *_router) getMessageComments(w http.ResponseWriter, r *http.Request, ps httprouter.Params, ctx reqcontext.RequestContext) {
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

	// The message travels in the URL as well: it names whose reactions are being read
	messageId := schemas.MessageId(ps.ByName("messageId"))
	if err := messageId.IsValid(); err != nil {
		writeError(w, ctx, http.StatusBadRequest, "invalid message id", err)
		return
	}

	// Query
	comments, err := rt.db.GetMessageComments(userId, chatId, messageId)
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
		// The message belongs to this chat, but reading its reactions belongs to its members
		if errors.Is(err, database.ErrNotAMember) {
			writeError(w, ctx, http.StatusForbidden, "not a member of the chat", nil)
			return
		}
		// There is no separate comments cap to check: at most one row belongs to each member,
		// and a chat cannot have more than schemas.GroupMaxMembers members
		// The UNIQUE(messageId, userId) constraint prevents a user from adding more than one comment to the same message,
		// and the ON CONFLICT clause updates the old comment instead of piling up

		writeError(w, ctx, http.StatusInternalServerError, "cannot get the comments", err)
		return
	}

	if len(comments) == 0 {
		writeError(w, ctx, http.StatusNotFound, "no comments found", nil)
		return
	}

	// Response
	writeJSON(w, ctx, http.StatusOK, struct {
		Comments schemas.Comments `json:"comments"`
	}{Comments: comments})
}
