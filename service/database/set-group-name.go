package database

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/MercuriLorenzo/WASAText/service/schemas"
)

// SetGroupName updates the name of the group of the given id, and returns its updated summary
// Renaming a group belongs to its members, so userId must be one of them
// It returns ErrChatNotFound if no group owns that id, and ErrNotAMember if the caller does not belong to it
func (db *appdbimpl) SetGroupName(userId schemas.UserId, groupId schemas.ChatId, newName schemas.ChatName) (schemas.ChatBase, error) {
	var chat schemas.ChatBase
	err := db.c.QueryRow(`UPDATE chats SET name = ? WHERE id = ? AND chatType = ?
						  AND EXISTS (SELECT 1 FROM chat_members WHERE chatId = ? AND userId = ?)
						  RETURNING id, name, photoId;`,
		newName, groupId, schemas.ChatTypeGroup, groupId, userId,
	).Scan(&chat.Id, &chat.Name, &chat.Photo)

	// Nothing was updated: either no group owns that id, or the caller is not one of its members
	// No transaction needed because this SELECT touches only immutable columns
	if errors.Is(err, sql.ErrNoRows) {
		err = db.c.QueryRow(`SELECT 1 FROM chats WHERE id = ? AND chatType = ?;`, groupId, schemas.ChatTypeGroup).Scan(new(int))
		// No group owns that id
		if errors.Is(err, sql.ErrNoRows) {
			return schemas.ChatBase{}, ErrChatNotFound
		}
		// Error reading the group
		if err != nil {
			return schemas.ChatBase{}, fmt.Errorf("error checking if group exists: %w", err)
		}

		// The caller is not in the group
		return schemas.ChatBase{}, ErrNotAMember
	}
	// Error during update
	if err != nil {
		return schemas.ChatBase{}, fmt.Errorf("error updating group name: %w", err)
	}

	// The chatType is a condition of the update above and never a value read back from it
	chat.Type = schemas.ChatTypeGroup

	return chat, nil
}
