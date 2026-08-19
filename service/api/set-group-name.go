package api

import (
	"errors"
	"net/http"

	"github.com/MercuriLorenzo/WASAText/service/api/reqcontext"
	"github.com/MercuriLorenzo/WASAText/service/database"
	"github.com/MercuriLorenzo/WASAText/service/schemas"
	"github.com/julienschmidt/httprouter"
)

// setGroupName updates the name of a group the authenticated user is a member of
func (rt *_router) setGroupName(w http.ResponseWriter, r *http.Request, ps httprouter.Params, ctx reqcontext.RequestContext) {
	// Get the userId that authenticate injected in the request
	userId, ok := userIdFromContext(r)
	if !ok {
		writeError(w, ctx, http.StatusUnauthorized, "user not authenticated", nil)
		return
	}

	// The group travels in the URL and not in a body, so it is checked here:
	// it reaches the database only once it is a well formed id
	// ByName returns a string, and an assignment needs one of the two types to be unnamed:
	// string and ChatId are both named, so the conversion is what carries the id across
	groupId := schemas.ChatId(ps.ByName("groupId"))
	if err := groupId.IsValid(); err != nil {
		writeError(w, ctx, http.StatusBadRequest, "invalid group id", err)
		return
	}

	// Parse and check the request body
	var req schemas.GroupNameRequest
	if err := decodeAndValidate(r, &req); err != nil {
		writeError(w, ctx, http.StatusBadRequest, "invalid request body", err)
		return
	}

	// Query
	chat, err := rt.db.SetGroupName(userId, groupId, req.Name)
	if err != nil {
		// No group owns that id: it may not exist at all, or be a private chat,
		// which borrows its name from the other member and owns none to update
		if errors.Is(err, database.ErrChatNotFound) {
			writeError(w, ctx, http.StatusNotFound, "group not found", nil)
			return
		}

		// The group is there, but renaming it belongs to its members
		if errors.Is(err, database.ErrNotAMember) {
			writeError(w, ctx, http.StatusForbidden, "not a member of the group", nil)
			return
		}

		writeError(w, ctx, http.StatusInternalServerError, "cannot update the group name", err)
		return
	}

	// Response
	writeJSON(w, ctx, http.StatusOK, withChatPhotoURL(chat))
}
