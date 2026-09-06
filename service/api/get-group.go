package api

import (
	"errors"
	"net/http"

	"github.com/MercuriLorenzo/WASAText/service/api/reqcontext"
	"github.com/MercuriLorenzo/WASAText/service/database"
	"github.com/MercuriLorenzo/WASAText/service/schemas"
	"github.com/julienschmidt/httprouter"
)

// getGroup returns the group with the given ID, if the caller is a member of it
func (rt *_router) getGroup(w http.ResponseWriter, r *http.Request, ps httprouter.Params, ctx reqcontext.RequestContext) {
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

	// Query
	group, err := rt.db.GetGroup(userId, groupId)
	if err != nil {
		// No chat owns that id
		if errors.Is(err, database.ErrChatNotFound) {
			writeError(w, ctx, http.StatusNotFound, "group not found", nil)
		}
		// The chat is there, but reading it belongs to its members
		if errors.Is(err, database.ErrNotAMember) {
			writeError(w, ctx, http.StatusForbidden, "not a member of the group", nil)
		}

		writeError(w, ctx, http.StatusInternalServerError, "cannot get the group", err)
		return
	}

	// Response
	// The base is embedded, so the photo id it carries is turned into a URL through it
	group.ChatBase = withChatPhotoURL(group.ChatBase)
	writeJSON(w, ctx, http.StatusOK, group)
}
