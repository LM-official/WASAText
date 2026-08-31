package api

import (
	"errors"
	"net/http"

	"github.com/MercuriLorenzo/WASAText/service/api/reqcontext"
	"github.com/MercuriLorenzo/WASAText/service/database"
	"github.com/MercuriLorenzo/WASAText/service/schemas"
	"github.com/julienschmidt/httprouter"
)

// leaveGroup removes the authenticated user from a group it is a member of
func (rt *_router) leaveGroup(w http.ResponseWriter, r *http.Request, ps httprouter.Params, ctx reqcontext.RequestContext) {
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
	photosToRelease, err := rt.db.LeaveGroup(userId, groupId)
	if err != nil {
		// No group owns that id: it may not exist at all, or be a private chat,
		// which borrows its two members from its pair and is left by neither
		if errors.Is(err, database.ErrChatNotFound) {
			writeError(w, ctx, http.StatusNotFound, "group not found", nil)
			return
		}
		// The group is there, but leaving it belongs to its members
		if errors.Is(err, database.ErrNotAMember) {
			writeError(w, ctx, http.StatusForbidden, "not a member of the group", nil)
			return
		}

		writeError(w, ctx, http.StatusInternalServerError, "cannot leave the group", err)
		return
	}

	// Only the last member receives candidates: the group photo and the distinct photos of its deleted messages
	// A forwarded photo may still be used elsewhere, so releasePhoto checks every candidate after the commit
	for _, photoId := range photosToRelease {
		rt.releasePhoto(photoId, ctx)
	}

	// Response
	// A DELETE answers with no body: this call remove the caller membership to the group
	// Who is left inside is read by the members of that group, and the caller has just stopped being one
	w.WriteHeader(http.StatusNoContent)
}
