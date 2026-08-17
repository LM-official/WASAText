package database

import (
	"fmt"
	"strings"

	"github.com/MercuriLorenzo/WASAText/service/schemas"
	"github.com/gofrs/uuid"
)

// CreateGroup creates a group named name and showing photoId, whose members are userIds plus creator
// The request carries the other members only: the creator belongs to the group it opens,
// so it is added here and never travels in the body
func (db *appdbimpl) CreateGroup(creator schemas.UserId, userIds schemas.Members, name schemas.ChatName, photoId schemas.PhotoId) (schemas.ChatId, error) {
	// Generate the new UUID
	newUUID, err := uuid.NewV4()
	if err != nil {
		// Error generating the UUID
		return schemas.ChatId(""), err
	}
	newId := schemas.ChatId(newUUID.String())

	// One INSERT holding a tuple per member
	// The creator opens the group and belongs to it, but the request does not carry it:
	// it is the first tuple, so the list is never empty and the statement never ends on VALUES
	// Every tuple is the same text so it is repeated
	placeholders := "(?, ?)" + strings.Repeat(", (?, ?)", len(userIds))
	args := make([]interface{}, 0, (len(userIds)+1)*2)
	args = append(args, newId, creator)
	for _, member := range userIds {
		args = append(args, newId, member)
	}

	// The chat row and its memberships must appear together or not at all:
	// a chat without members would be unreachable
	// and members without a chat would break their foreign key
	tx, err := db.c.Begin()
	if err != nil {
		return schemas.ChatId(""), fmt.Errorf("cannot start the transaction: %w", err)
	}
	// Undo everything unless the commit below is reached
	defer func() { _ = tx.Rollback() }()

	// A group chat owns its name and photo
	_, err = tx.Exec(`INSERT INTO chats (id, chatType, name, photoId, pairKey) VALUES (?, 'group', ?, ?, NULL);`, newId, name, photoId)
	if err != nil {
		// Error inserting the new chat
		return schemas.ChatId(""), err
	}

	// New chat created, update the memberships
	_, err = tx.Exec(`INSERT INTO chat_members (chatId, userId) VALUES `+placeholders+`;`, args...)
	if err != nil {
		// Error inserting the members
		return schemas.ChatId(""), err
	}

	if err := tx.Commit(); err != nil {
		return schemas.ChatId(""), fmt.Errorf("cannot commit the transaction: %w", err)
	}

	return newId, nil
}
