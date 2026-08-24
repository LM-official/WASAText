package database

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/MercuriLorenzo/WASAText/service/schemas"
)

// LeaveGroup removes the caller from the group and returns it with its remaining members
// Leaving a group belongs to its members, so userId must be one of them
//
// A group nobody belongs to can never be reached again, so the last member out drops it:
// its messages go with it through the ON DELETE CASCADE of the messages table, while its memberships
// are already gone, the last of them being the one this call removed
// An empty members list is what tells the caller the group is gone, and its photo is the one carried by the reply
//
// It returns ErrChatNotFound if no group owns that id, and ErrNotAMember if the caller does not belong to it
func (db *appdbimpl) LeaveGroup(userId schemas.UserId, groupId schemas.ChatId) (schemas.ChatWithMembers, error) {
	// The membership is read, then removed, and the row of the group is dropped against what is left of it:
	// all three must see the same group, or the last two members leaving at once would each still find
	// the other inside and neither would drop it
	tx, err := db.c.Begin()
	if err != nil {
		return schemas.ChatWithMembers{}, fmt.Errorf("cannot start the transaction: %w", err)
	}
	// Undo everything unless the commit below is reached
	defer func() { _ = tx.Rollback() }()

	// One row that answers both questions the call asks, and carries what the reply needs:
	// does a group own that id, and is the caller inside it
	var chat schemas.ChatWithMembers
	var isMember bool
	err = tx.QueryRow(`SELECT c.id, c.name, c.photoId,
					   EXISTS (SELECT 1 FROM chat_members WHERE chatId = c.id AND userId = ?)
					   FROM chats c WHERE c.id = ? AND c.chatType = ?;`,
		userId, groupId, schemas.ChatTypeGroup,
	).Scan(&chat.Id, &chat.Name, &chat.Photo, &isMember)

	// No group owns that id: it may not exist at all, or be a private chat
	if errors.Is(err, sql.ErrNoRows) {
		return schemas.ChatWithMembers{}, ErrChatNotFound
	}
	// Error reading the group
	if err != nil {
		return schemas.ChatWithMembers{}, fmt.Errorf("cannot read the group: %w", err)
	}
	// The group is there, but leaving it belongs to its members
	if !isMember {
		return schemas.ChatWithMembers{}, ErrNotAMember
	}

	// Remove the caller from the group
	_, err = tx.Exec(`DELETE FROM chat_members WHERE chatId = ? AND userId = ?;`, groupId, userId)
	if err != nil {
		return schemas.ChatWithMembers{}, fmt.Errorf("cannot remove the caller from the group: %w", err)
	}

	// Read after the DELETE, so the list is what the caller leaves behind and never includes the caller
	// The caller may have been the last one inside,
	// so the list is built non nil to reach the client as [] and never as null
	chat.Members = make(schemas.Members, 0)
	rows, err := tx.Query(`SELECT userId FROM chat_members WHERE chatId = ?;`, groupId)
	// Error reading the members
	if err != nil {
		return schemas.ChatWithMembers{}, fmt.Errorf("cannot read the remaining members of the group: %w", err)
	}
	// Close rows when done even in case of error
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var member schemas.UserId
		// Error reading a row: a shorter list would be a wrong answer, not a partial one,
		// and here it would also drop a group somebody still belongs to
		if err := rows.Scan(&member); err != nil {
			return schemas.ChatWithMembers{}, fmt.Errorf("cannot read a remaining group member: %w", err)
		}

		chat.Members = append(chat.Members, member)
	}
	// Error during rows iteration
	if err := rows.Err(); err != nil {
		return schemas.ChatWithMembers{}, fmt.Errorf("cannot read the remaining members of the group: %w", err)
	}

	// Nobody is left inside: the group can never be reached again, so it is dropped
	// The reply still carries it as it stood, so the caller reads the photo of a group that no longer exists
	// and releases the file once no other row shows it
	if len(chat.Members) == 0 {
		_, err = tx.Exec(`DELETE FROM chats WHERE id = ? AND chatType = ?;`, groupId, schemas.ChatTypeGroup)
		if err != nil {
			return schemas.ChatWithMembers{}, fmt.Errorf("cannot delete the empty group: %w", err)
		}
	}

	// The chatType is a condition of the query above and never a value read back from it
	chat.Type = schemas.ChatTypeGroup

	if err := tx.Commit(); err != nil {
		return schemas.ChatWithMembers{}, fmt.Errorf("cannot commit the transaction: %w", err)
	}

	return chat, nil
}
