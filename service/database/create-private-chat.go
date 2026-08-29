package database

import (
	"fmt"
	"time"

	"github.com/MercuriLorenzo/WASAText/service/globaltime"
	"github.com/MercuriLorenzo/WASAText/service/schemas"
	"github.com/gofrs/uuid"
)

// pairKey builds the value of the chats.pairKey column:
// the two member ids sorted and joined
// Sorting is what makes the key of A and B the same key as the one of B and A,
// so the UNIQUE on that column sees the two requests as the same pair
func pairKey(userId1 schemas.UserId, userId2 schemas.UserId) string {
	if userId1 > userId2 {
		userId1, userId2 = userId2, userId1
	}
	return string(userId1) + "|" + string(userId2)
}

// CreatePrivateChat returns the private chat of the two given users, creating it if it does not exist yet
// The boolean is true when the chat was already there, so that the api layer can answer 200 instead of 201
func (db *appdbimpl) CreatePrivateChat(userId1 schemas.UserId, userId2 schemas.UserId) (schemas.ChatId, bool, error) {
	key := pairKey(userId1, userId2)

	// Generate the new UUID
	newUUID, err := uuid.NewV4()
	// Error generating the UUID
	if err != nil {
		return schemas.ChatId(""), false, fmt.Errorf("cannot generate a new UUID: %w", err)
	}
	newId := schemas.ChatId(newUUID.String())

	// The chat row and its two memberships must appear together or not at all:
	// a chat without members would be unreachable
	// and members without a chat would break their foreign key
	tx, err := db.c.Begin()
	if err != nil {
		return schemas.ChatId(""), false, fmt.Errorf("cannot start the transaction: %w", err)
	}
	// Undo everything unless the commit below is reached
	defer func() { _ = tx.Rollback() }()

	// A private chat owns no name and no photo: it borrows both from the other member
	// DO NOTHING leaves the pair to the UNIQUE on pairKey:
	// a chat that is already there writes no row and raises no error
	res, err := tx.Exec(`INSERT INTO chats (id, chatType, name, photoId, pairKey) VALUES (?, ?, NULL, NULL, ?)
						 ON CONFLICT(pairKey) DO NOTHING;`, newId, schemas.ChatTypePrivate, key)
	// Error inserting the new chat
	if err != nil {
		return schemas.ChatId(""), false, fmt.Errorf("cannot insert the new chat %q: %w", newId, err)
	}

	written, err := res.RowsAffected()
	if err != nil {
		return schemas.ChatId(""), false, fmt.Errorf("cannot read the number of rows written for the new chat %q: %w", newId, err)
	}
	// No row written: this pair already owns a chat
	if written == 0 {
		var id schemas.ChatId
		if err := tx.QueryRow(`SELECT id FROM chats WHERE pairKey = ?;`, key).Scan(&id); err != nil {
			return schemas.ChatId(""), false, fmt.Errorf("cannot read the id of the chat already owned by the pair %q: %w", key, err)
		}
		// Chat already exists
		return id, true, nil
	}

	// New chat created, update the memberships
	// A membership is born caught up to now and never holds NULL:
	// the chat carries no message yet, so there is nothing either of them could be behind
	joinDate := globaltime.Now().UTC().Truncate(time.Millisecond).Format(dateFormat)
	_, err = tx.Exec(`INSERT INTO chat_members (chatId, userId, lastReadDate) VALUES (?, ?, ?), (?, ?, ?);`,
		newId, userId1, joinDate, newId, userId2, joinDate)
	// Error inserting the members
	if err != nil {
		return schemas.ChatId(""), false, fmt.Errorf("cannot insert the members of the new chat %q: %w", newId, err)
	}

	if err := tx.Commit(); err != nil {
		return schemas.ChatId(""), false, fmt.Errorf("cannot commit the transaction: %w", err)
	}

	return newId, false, nil
}
