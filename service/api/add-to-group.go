package api

import (
	"errors"
	"net/http"

	"github.com/MercuriLorenzo/WASAText/service/api/reqcontext"
	"github.com/MercuriLorenzo/WASAText/service/database"
	"github.com/MercuriLorenzo/WASAText/service/schemas"
	"github.com/julienschmidt/httprouter"
)

func (rt *_router) addToGroup(w http.ResponseWriter, r *http.Request, ps httprouter.Params, ctx reqcontext.RequestContext) {
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
	var req schemas.MembersRequest
	if err := decodeAndValidate(r, &req); err != nil {
		writeError(w, ctx, http.StatusBadRequest, "invalid request body", err)
		return
	}

	// Every new member must already exist
	exist, err := rt.db.UsersExist(req.Members)
	if err != nil {
		writeError(w, ctx, http.StatusInternalServerError, "cannot verify the members", err)
		return
	}
	// Some members do not exist
	if !exist {
		writeError(w, ctx, http.StatusNotFound, "one or more of the members does not exist", nil)
		return
	}

	// Query
	chat, err := rt.db.AddToGroup(userId, groupId, req.Members)
	if err != nil {
		// No group owns that id: it may not exist at all, or be a private chat
		if errors.Is(err, database.ErrChatNotFound) {
			writeError(w, ctx, http.StatusNotFound, "group not found", nil)
			return
		}

		// The group is there, but joining people to it belongs to its members
		if errors.Is(err, database.ErrNotAMember) {
			writeError(w, ctx, http.StatusForbidden, "not a member of the group", nil)
			return
		}

		// The additions would take the group past schemas.GroupMaxMembers:
		// what was asked cannot fit, which is about the request and not about who is asking
		if errors.Is(err, database.ErrGroupFull) {
			writeError(w, ctx, http.StatusBadRequest, "the group is full", nil)
			return
		}

		writeError(w, ctx, http.StatusInternalServerError, "cannot add members to the group", err)
		return
	}

	// Response
	// The base is embedded, so the photo id it carries is turned into a URL through it
	chat.ChatBase = withChatPhotoURL(chat.ChatBase)
	writeJSON(w, ctx, http.StatusOK, chat)
}
