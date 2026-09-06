package api

import (
	"errors"
	"net/http"

	"github.com/MercuriLorenzo/WASAText/service/api/reqcontext"
	"github.com/MercuriLorenzo/WASAText/service/database"
	"github.com/MercuriLorenzo/WASAText/service/schemas"
	"github.com/julienschmidt/httprouter"
)

// createPrivateChat creates a private chat between the authenticated user and the user of the request body,
// or returns the existing chat, with metadata and both member IDs
// Returns 201 when the chat is created, 200 when it was already there
func (rt *_router) createPrivateChat(w http.ResponseWriter, r *http.Request, ps httprouter.Params, ctx reqcontext.RequestContext) {
	// Get the userId that authenticate injected in the request
	userId, ok := userIdFromContext(r)
	if !ok {
		writeError(w, ctx, http.StatusUnauthorized, "user not authenticated", nil)
		return
	}

	// Parse and check the request body
	var req schemas.UserIdRequest
	if err := decodeAndValidate(r, &req); err != nil {
		writeError(w, ctx, http.StatusBadRequest, "invalid request body", err)
		return
	}

	// A private chat must hold two different members
	// one user alone would reach chat_members as the same row twice,
	// so the pair is refused here and never becomes a failed insert
	if req.Id == userId {
		writeError(w, ctx, http.StatusBadRequest, "cannot open a private chat with yourself", nil)
		return
	}

	// The other member must exist: CreatePrivateChat trusts the id it is given and only reads
	// this user's profile for the name/photo it borrows, so an unchecked id would surface as a
	// generic 500 instead of a 404. Checked here, once, before touching the chat at all.
	if _, err := rt.db.GetUserById(req.Id); err != nil {
		// No user owns that id
		if errors.Is(err, database.ErrUserNotFound) {
			writeError(w, ctx, http.StatusNotFound, "the other user does not exist", nil)
			return
		}

		writeError(w, ctx, http.StatusInternalServerError, "cannot verify the other user", err)
		return
	}

	// Query
	chat, found, err := rt.db.CreatePrivateChat(userId, req.Id)
	if err != nil {
		writeError(w, ctx, http.StatusInternalServerError, "cannot create the private chat", err)
		return
	}

	// An existing chat is returned, a new one is created first
	code := http.StatusCreated
	if found {
		code = http.StatusOK
	}

	// The base is embedded, so the photo id it carries is turned into a URL through it
	chat.ChatBase = withChatPhotoURL(chat.ChatBase)
	writeJSON(w, ctx, code, chat)
}
