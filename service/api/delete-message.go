package api

import (
	"errors"
	"net/http"

	"github.com/MercuriLorenzo/WASAText/service/api/reqcontext"
	"github.com/MercuriLorenzo/WASAText/service/database"
	"github.com/MercuriLorenzo/WASAText/service/schemas"
	"github.com/julienschmidt/httprouter"
)

// deleteMessage removes a message the authenticated user sent, along with every comment on it
// The user must be a member of the chat named in the URL, the message must actually belong to it,
// and the user must be who sent it
func (rt *_router) deleteMessage(w http.ResponseWriter, r *http.Request, ps httprouter.Params, ctx reqcontext.RequestContext) {
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

	// The message travels in the URL as well: it names what is being deleted
	messageId := schemas.MessageId(ps.ByName("messageId"))
	if err := messageId.IsValid(); err != nil {
		writeError(w, ctx, http.StatusBadRequest, "invalid message id", err)
		return
	}

	// Query
	oldPhotoId, err := rt.db.DeleteMessage(userId, chatId, messageId)
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
		// The message belongs to this chat, but deleting it still requires belonging to that chat
		if errors.Is(err, database.ErrNotAMember) {
			writeError(w, ctx, http.StatusForbidden, "not a member of the chat", nil)
			return
		}
		// The caller is a member and the message is real, but it was not the caller who sent it
		if errors.Is(err, database.ErrNotSender) {
			writeError(w, ctx, http.StatusForbidden, "not the sender of the message", nil)
			return
		}

		writeError(w, ctx, http.StatusInternalServerError, "cannot delete the message", err)
		return
	}

	// The message may have carried a photo, and deleting it is the last thing pointing at that file:
	// DeleteMessage gives back that photo only when the deleted message carried one,
	// and an empty id otherwise, which releasePhoto takes as nothing to do
	// releasePhoto itself checks PhotoIsReferenced, so a photo forwarded into other messages survives this one's deletion
	rt.releasePhoto(oldPhotoId, ctx)

	// Response
	// A DELETE answers with no body: this call removes the message and every comment on it
	// Whoever reads the chat afterwards simply finds one fewer message in its list
	w.WriteHeader(http.StatusNoContent)
}
