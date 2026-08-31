package api

import (
	"errors"
	"net/http"

	"github.com/MercuriLorenzo/WASAText/service/api/reqcontext"
	"github.com/MercuriLorenzo/WASAText/service/database"
	"github.com/MercuriLorenzo/WASAText/service/schemas"
	"github.com/julienschmidt/httprouter"
)

// sendMessage writes a message in a chat the authenticated user is a member of
// A message carries text, a photo, or both
func (rt *_router) sendMessage(w http.ResponseWriter, r *http.Request, ps httprouter.Params, ctx reqcontext.RequestContext) {
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

	// A body too large ends here as well: error 400 to everything the client got wrong, size included
	if err := parsePhotoMultipartForm(w, r); err != nil {
		writeError(w, ctx, http.StatusBadRequest, "invalid multipart body", err)
		return
	}
	// Drop the temporary files of the parser
	defer func() { _ = r.MultipartForm.RemoveAll() }()

	// The text travels as a form field. Its request schema owns both its rules and normalization.
	textReq := schemas.MessageTextRequest{Text: schemas.MessageText(r.FormValue("text"))}
	if err := textReq.IsValid(); err != nil {
		writeError(w, ctx, http.StatusBadRequest, "invalid message text", err)
		return
	}
	text, hasText := textReq.Text, textReq.Text != ""

	// The photo is optional here: a part that is missing is a message without a photo and not a bad request,
	// while a part that is there and broken still is one
	file, _, err := r.FormFile("photoFile")
	hasPhoto := err == nil
	if err != nil && !errors.Is(err, http.ErrMissingFile) {
		writeError(w, ctx, http.StatusBadRequest, "invalid photoFile field", err)
		return
	}
	if hasPhoto {
		defer func() { _ = file.Close() }()
	}

	// A message must carry something
	if !hasText && !hasPhoto {
		writeError(w, ctx, http.StatusBadRequest, "empty message content: text or photo is required", nil)
		return
	}
	// Write the file first: an id is worth saving only once the bytes behind it exist
	var photoId schemas.PhotoId
	if hasPhoto {
		photoId, err = rt.photos.Save(file)
		if err != nil {
			// What the client sent is the problem, not the server
			if isInvalidPhoto(err) {
				writeError(w, ctx, http.StatusBadRequest, "invalid photo", err)
				return
			}

			writeError(w, ctx, http.StatusInternalServerError, "cannot store the photo", err)
			return
		}
	}

	// Query
	message, err := rt.db.SendMessage(userId, chatId, text, photoId)
	if err != nil {
		// Whatever the failure is, the photo is on disk and no row points at it: drop it instead of leaking a file
		// It is always an upload of this request, never a photo shared with something else
		if hasPhoto {
			if delErr := rt.photos.Delete(photoId); delErr != nil {
				logWarning(ctx, "cannot delete the photo of a failed message", delErr)
			}
		}
		// No chat owns that id
		if errors.Is(err, database.ErrChatNotFound) {
			writeError(w, ctx, http.StatusNotFound, "chat not found", nil)
			return
		}
		// The chat is there, but writing in it belongs to its members
		if errors.Is(err, database.ErrNotAMember) {
			writeError(w, ctx, http.StatusForbidden, "not a member of the chat", nil)
			return
		}
		// The chat already holds schemas.ChatMaxMessages messages:
		// what was asked cannot fit, which is about the request and not about who is asking
		if errors.Is(err, database.ErrChatFull) {
			writeError(w, ctx, http.StatusBadRequest, "the chat is full", nil)
			return
		}

		writeError(w, ctx, http.StatusInternalServerError, "cannot send the message", err)
		return
	}

	// Response
	// The message carries one photo at most, and the one without keeps the field empty
	writeJSON(w, ctx, http.StatusCreated, withMessagePhotoURL(message))
}
