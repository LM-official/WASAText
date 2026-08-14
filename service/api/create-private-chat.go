package api

import (
	"net/http"

	"github.com/MercuriLorenzo/WASAText/service/api/reqcontext"
	"github.com/MercuriLorenzo/WASAText/service/schemas"
	"github.com/julienschmidt/httprouter"
)

// createPrivateChat creates a private chat between the authenticated user and the user of the request body,
// or just get the already existing chatId
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

	// The other member must exist
	exists, err := rt.db.UserExists(req.Id)
	if err != nil {
		writeError(w, ctx, http.StatusInternalServerError, "cannot verify the other user", err)
		return
	}
	if !exists {
		writeError(w, ctx, http.StatusNotFound, "the other user does not exist", nil)
		return
	}

	// Query
	id, found, err := rt.db.CreatePrivateChat(userId, req.Id)
	if err != nil {
		writeError(w, ctx, http.StatusInternalServerError, "cannot create the private chat", err)
		return
	}

	// An existing chat is returned, a new one is created first
	code := http.StatusCreated
	if found {
		code = http.StatusOK
	}

	// Response
	writeJSON(w, ctx, code, struct {
		Id schemas.ChatId `json:"id"`
	}{Id: id})
}
