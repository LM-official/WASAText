package api

import (
	"errors"
	"net/http"

	"github.com/MercuriLorenzo/WASAText/service/api/reqcontext"
	"github.com/MercuriLorenzo/WASAText/service/database"
	"github.com/MercuriLorenzo/WASAText/service/schemas"
	"github.com/julienschmidt/httprouter"
)

// commentMessage adds the authenticated user's reaction to a message,
// updating the emoji of any earlier reaction of the same user on the same message
// The user must be a member of the chat named in the URL, and the message must actually belong to it
func (rt *_router) commentMessage(w http.ResponseWriter, r *http.Request, ps httprouter.Params, ctx reqcontext.RequestContext) {
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

	// The message travels in the URL as well: it names what is being reacted to
	messageId := schemas.MessageId(ps.ByName("messageId"))
	if err := messageId.IsValid(); err != nil {
		writeError(w, ctx, http.StatusBadRequest, "invalid message id", err)
		return
	}

	// The emoji travels in the body: unlike sendMessage, there is no file to accept here,
	// so a plain JSON body is enough and a multipart parser is not needed
	var req schemas.EmojiRequest
	if err := decodeAndValidate(r, &req); err != nil {
		writeError(w, ctx, http.StatusBadRequest, "invalid request body", err)
		return
	}

	// Query
	message, alreadyExisted, err := rt.db.CommentMessage(userId, chatId, messageId, req.Emoji)
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

		// The message belongs to this chat, but reacting to it belongs to its members
		if errors.Is(err, database.ErrNotAMember) {
			writeError(w, ctx, http.StatusForbidden, "not a member of the chat", nil)
			return
		}

		// There is no separate comments cap to check: at most one row belongs to each member,
		// and a chat cannot have more than schemas.GroupMaxMembers members
		// The UNIQUE(messageId, userId) constraint prevents a user from adding more than one comment to the same message,
		// and the ON CONFLICT clause updates the old comment instead of piling up

		writeError(w, ctx, http.StatusInternalServerError, "cannot comment the message", err)
		return
	}

	// Response: creating the caller's comment is 201; updating its emoji is 200
	// The message carries one photo at most, and the one without keeps the field empty
	code := http.StatusCreated
	if alreadyExisted {
		code = http.StatusOK
	}
	writeJSON(w, ctx, code, withMessagePhotoURL(message))
}
