package database

import (
	"fmt"
	"strings"
	"time"

	"github.com/MercuriLorenzo/WASAText/service/globaltime"
	"github.com/MercuriLorenzo/WASAText/service/schemas"
	"github.com/gofrs/uuid"
)

// CreateGroup creates a group named name and showing photoId, whose members are userIds plus creator
// The request carries the other members only: the creator belongs to the group it opens,
// so it is added here and never travels in the body
func (db *appdbimpl) CreateGroup(creator schemas.UserId, userIds schemas.Members, name schemas.ChatName, photoId schemas.PhotoId) (schemas.ChatId, error) {
	// Generate the new UUID
	newUUID, err := uuid.NewV4()
	// Error generating the UUID
	if err != nil {
		return schemas.ChatId(""), fmt.Errorf("cannot generate a new UUID: %w", err)
	}
	newId := schemas.ChatId(newUUID.String())

	// A membership is born caught up to now and never holds NULL:
	// the group carries no message yet, so there is nothing any of them could be behind
	joinDate := globaltime.Format(globaltime.Now().UTC().Truncate(time.Millisecond))

	// One INSERT holding a tuple per member
	// The creator opens the group and belongs to it, but the request does not carry it:
	// it is the first tuple, so the list is never empty and the statement never ends on VALUES
	// Every tuple is the same text so it is repeated
	placeholders := "(?, ?, ?)" + strings.Repeat(", (?, ?, ?)", len(userIds))
	args := make([]interface{}, 0, (len(userIds)+1)*3)
	args = append(args, newId, creator, joinDate)
	for _, member := range userIds {
		args = append(args, newId, member, joinDate)
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
	_, err = tx.Exec(`INSERT INTO chats (id, chatType, name, photoId, pairKey) VALUES (?, ?, ?, ?, NULL);`, newId, schemas.ChatTypeGroup, name, photoId)
	// Error inserting the new chat
	if err != nil {
		return schemas.ChatId(""), fmt.Errorf("cannot insert the new chat %q: %w", newId, err)
	}

	// New chat created, update the memberships
	_, err = tx.Exec(`INSERT INTO chat_members (chatId, userId, lastReadDate) VALUES `+placeholders+`;`, args...)
	// Error inserting the members
	if err != nil {
		return schemas.ChatId(""), fmt.Errorf("cannot insert the members of the new chat %q: %w", newId, err)
	}

	if err := tx.Commit(); err != nil {
		return schemas.ChatId(""), fmt.Errorf("cannot commit the transaction: %w", err)
	}

	return newId, nil
}
