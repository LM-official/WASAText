package database

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/MercuriLorenzo/WASAText/service/schemas"
)

// SetGroupPhoto sets newPhotoId as the photo of the group of the given id,
// and returns its updated summary together with the id of the photo it replaced
// The old id is what lets the caller drop the file of that photo once nothing points at it
// Changing the photo of a group belongs to its members, so userId must be one of them
// It returns ErrChatNotFound if no group owns that id, and ErrNotAMember if the caller does not belong to it
func (db *appdbimpl) SetGroupPhoto(userId schemas.UserId, groupId schemas.ChatId, newPhotoId schemas.PhotoId) (schemas.ChatSummary, schemas.PhotoId, error) {
	// Reading the old id and writing the new one must be one single step:
	// between the two, a concurrent update would make the caller delete the file of a photo that is now in use
	tx, err := db.c.Begin()
	if err != nil {
		return schemas.ChatSummary{}, "", fmt.Errorf("cannot start the transaction: %w", err)
	}
	// Undo everything unless the commit below is reached
	defer func() { _ = tx.Rollback() }()

	// Get the photo that is being replaced
	// It also says whether a group owns that id, so the update below is left with the membership alone to fail on
	// The CHECK of the chats table gives every group a photo, so this column is never NULL here
	var oldPhotoId schemas.PhotoId
	err = tx.QueryRow(`SELECT photoId FROM chats WHERE id = ? AND chatType = ?;`, groupId, schemas.ChatTypeGroup).Scan(&oldPhotoId)
	// No group owns that id
	if errors.Is(err, sql.ErrNoRows) {
		return schemas.ChatSummary{}, "", ErrChatNotFound
	}
	// Error fetching the current photo
	if err != nil {
		return schemas.ChatSummary{}, "", err
	}

	var chat schemas.ChatSummary
	err = tx.QueryRow(`UPDATE chats SET photoId = ? WHERE id = ? AND chatType = ?
					   AND EXISTS (SELECT 1 FROM chat_members WHERE chatId = ? AND userId = ?)
					   RETURNING id, name, photoId;`,
		newPhotoId, groupId, schemas.ChatTypeGroup, groupId, userId,
	).Scan(&chat.Id, &chat.Name, &chat.Photo)

	// Nothing was updated, and the SELECT above already found the group:
	// the condition left to fail is the one on the members, so the caller is not in the group
	if errors.Is(err, sql.ErrNoRows) {
		return schemas.ChatSummary{}, "", ErrNotAMember
	}
	// Error during update
	if err != nil {
		return schemas.ChatSummary{}, "", err
	}

	if err := tx.Commit(); err != nil {
		return schemas.ChatSummary{}, "", fmt.Errorf("cannot commit the transaction: %w", err)
	}

	// The chatType is a condition of the update above and never a value read back from it
	chat.Type = schemas.ChatTypeGroup

	// The snippet is left empty: it is derived from the messages of the chat, which this call never reads
	return chat, oldPhotoId, nil
}
