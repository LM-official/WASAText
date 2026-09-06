package database

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/MercuriLorenzo/WASAText/service/schemas"
)

// GetGroup reads metadata and membership in one snapshot without marking messages read.
func (db *appdbimpl) GetGroup(userId schemas.UserId, groupId schemas.ChatId) (schemas.ChatWithMembers, error) {
	// The checks and the next read must read the same snapshot
	tx, err := db.c.Begin()
	if err != nil {
		return schemas.ChatWithMembers{}, fmt.Errorf("cannot start the transaction: %w", err)
	}
	// Undo everything unless the commit below is reached
	defer func() { _ = tx.Rollback() }()

	// One row that answers both questions the call asks, and carries what the reply needs:
	// does a group own that id, and is the caller inside it
	var group schemas.ChatWithMembers
	var isMember bool
	err = tx.QueryRow(`SELECT id, chatType, name, photoId,
					   EXISTS (SELECT 1 FROM chat_members WHERE chatId = c.id AND userId = ?)
					   FROM chats AS c WHERE id = ? AND chatType = ?;`,
		userId, groupId, schemas.ChatTypeGroup).Scan(&group.Id, &group.Type, &group.Name, &group.Photo, &isMember)
	// No group owns that id: it may not exist at all, or be a private chat
	if errors.Is(err, sql.ErrNoRows) {
		return schemas.ChatWithMembers{}, ErrChatNotFound
	}
	// Error reading the group
	if err != nil {
		return schemas.ChatWithMembers{}, fmt.Errorf("cannot read group: %w", err)
	}
	// The group is there, but looking for the metadata it belongs to its members
	if !isMember {
		return schemas.ChatWithMembers{}, ErrNotAMember
	}

	// Get members ids
	rows, err := tx.Query(`SELECT userId FROM chat_members WHERE chatId = ?;`, groupId)
	if err != nil {
		return schemas.ChatWithMembers{}, fmt.Errorf("cannot read group members: %w", err)
	}
	// Close rows when done even in case of error
	defer func() { _ = rows.Close() }()

	group.Members = make(schemas.Members, 0)
	for rows.Next() {
		var id schemas.UserId
		// Error reading a row: a shorter list would be a wrong answer, not a partial one
		if err := rows.Scan(&id); err != nil {
			return schemas.ChatWithMembers{}, fmt.Errorf("cannot read member: %w", err)
		}

		group.Members = append(group.Members, id)
	}
	// Error during rows iteration
	if err := rows.Err(); err != nil {
		return schemas.ChatWithMembers{}, fmt.Errorf("cannot read group members: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return schemas.ChatWithMembers{}, fmt.Errorf("cannot commit the transaction: %w", err)
	}

	return group, nil
}
