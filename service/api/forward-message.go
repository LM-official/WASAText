package api

import (
	"errors"
	"net/http"

	"github.com/MercuriLorenzo/WASAText/service/api/reqcontext"
	"github.com/MercuriLorenzo/WASAText/service/database"
	"github.com/MercuriLorenzo/WASAText/service/schemas"
	"github.com/julienschmidt/httprouter"
)

// forwardMessage writes a copy of an existing message into a chat the authenticated user is a member of
// The source message is untouched: forwarding creates a new message, with its own id, date and state,
// carrying the same text/photo as the one it echoes
// The user must be a member of both the source and destination chats, and the destination chat must not be full
func (rt *_router) forwardMessage(w http.ResponseWriter, r *http.Request, ps httprouter.Params, ctx reqcontext.RequestContext) {
	// Get the userId that authenticate injected in the request
	userId, ok := userIdFromContext(r)
	if !ok {
		writeError(w, ctx, http.StatusUnauthorized, "user not authenticated", nil)
		return
	}

	// The destination chat travels in the URL and not in the body, so it is checked here:
	// it reaches the database only once it is a well formed id
	// Both kinds answer here: private chat and group
	chatId := schemas.ChatId(ps.ByName("chatId"))
	if err := chatId.IsValid(); err != nil {
		writeError(w, ctx, http.StatusBadRequest, "invalid chat id", err)
		return
	}

	// The source message travels in the body: unlike sendMessage, there is no file to accept here,
	// so a plain JSON body is enough and a multipart parser is not needed
	var req schemas.MessageIdRequest
	if err := decodeAndValidate(r, &req); err != nil {
		writeError(w, ctx, http.StatusBadRequest, "invalid request body", err)
		return
	}

	// Query
	message, err := rt.db.ForwardMessage(userId, chatId, req.MessageId)
	if err != nil {
		// No message owns that id: checked first, since a bad messageId should never
		// be shadowed by a coincidentally-also-bad destination chatId
		if errors.Is(err, database.ErrMessageNotFound) {
			writeError(w, ctx, http.StatusNotFound, "message not found", nil)
			return
		}
		// No chat owns the destination id
		if errors.Is(err, database.ErrChatNotFound) {
			writeError(w, ctx, http.StatusNotFound, "chat not found", nil)
			return
		}
		// Forwarding out of the source chat or into the destination chat belongs to their members
		if errors.Is(err, database.ErrNotAMember) {
			writeError(w, ctx, http.StatusForbidden, "not a member of the chat", nil)
			return
		}
		// The destination chat already holds schemas.ChatMaxMessages messages:
		// what was asked cannot fit, which is about the request and not about who is asking
		if errors.Is(err, database.ErrChatFull) {
			writeError(w, ctx, http.StatusBadRequest, "the chat is full", nil)
			return
		}

		writeError(w, ctx, http.StatusInternalServerError, "cannot forward the message", err)
		return
	}

	// Response
	// The message carries one photo at most, and the one without keeps the field empty
	writeJSON(w, ctx, http.StatusCreated, withMessagePhotoURL(message))
}
