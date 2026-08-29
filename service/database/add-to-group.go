package database

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/MercuriLorenzo/WASAText/service/globaltime"
	"github.com/MercuriLorenzo/WASAText/service/schemas"
)

// AddToGroup adds userIds to the group of the given id, and returns it with its updated members
// Joining people to a group belongs to its members, so userId must be one of them
// A user already inside is ignored, so the same request can be repeated without failing
// It returns ErrChatNotFound if no group owns that id, ErrNotAMember if the caller does not belong to it
// and ErrGroupFull if the additions take the group past schemas.GroupMaxMembers
func (db *appdbimpl) AddToGroup(userId schemas.UserId, groupId schemas.ChatId, userIds schemas.Members) (schemas.ChatWithMembers, error) {
	// The capacity is read and then written against, so both must see the same group:
	// between a count outside a transaction and its insert, another request could take the last slots
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
	// The group is there, but joining people to it belongs to its members
	if !isMember {
		return schemas.ChatWithMembers{}, ErrNotAMember
	}

	// An empty list adds nobody, and the checks above already answered for the group:
	// only the INSERT is skipped, the reply is still the group as it stands
	if len(userIds) > 0 {
		// A member joins caught up to now, and never behind the messages that were sent before it:
		// a message every member had already read must not turn back to received because somebody joined,
		// so the ones it was never sent are the ones it answers for none of
		joinDate := globaltime.Now().UTC().Truncate(time.Millisecond).Format(dateFormat)

		// One INSERT holding a tuple per member
		// Every tuple is the same text so it is repeated, one less than the members: the first one is written here
		placeholders := "(?, ?, ?)" + strings.Repeat(", (?, ?, ?)", len(userIds)-1)
		args := make([]interface{}, 0, len(userIds)*3)
		for _, member := range userIds {
			args = append(args, groupId, member, joinDate)
		}

		// OR IGNORE drops the tuples of the members already inside, which the API asks to ignore
		// It keeps the lastReadDate they already hold: re-adding a member is not a reason to mark
		// as unread what it had read, and the request asks to ignore it and not to touch it
		// It does not cover a foreign key, so an id that belongs to nobody still fails here
		_, err = tx.Exec(`INSERT OR IGNORE INTO chat_members (chatId, userId, lastReadDate) VALUES `+placeholders+`;`, args...)
		if err != nil {
			// Error inserting the members
			return schemas.ChatWithMembers{}, fmt.Errorf("cannot add the members to the group: %w", err)
		}
	}

	// Read after the INSERT, so the list holds the members just added
	// The reply carries the whole group, not only the additions,
	// so how long it gets is what the table says and not what the request asked:
	// no capacity is guessed here, append grows it
	// Empty is never the answer: the caller is a member, so it is one of these rows
	chat.Members = make(schemas.Members, 0)
	rows, err := tx.Query(`SELECT userId FROM chat_members WHERE chatId = ?;`, groupId)
	// Error reading the members
	if err != nil {
		return schemas.ChatWithMembers{}, fmt.Errorf("cannot read the group members: %w", err)
	}
	// Close rows when done even in case of error
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var member schemas.UserId
		// Error reading a row: a shorter list would be a wrong answer, not a partial one
		if err := rows.Scan(&member); err != nil {
			return schemas.ChatWithMembers{}, fmt.Errorf("cannot read a group member: %w", err)
		}

		chat.Members = append(chat.Members, member)
	}
	// Error during rows iteration
	if err := rows.Err(); err != nil {
		return schemas.ChatWithMembers{}, fmt.Errorf("cannot read the group members: %w", err)
	}

	// The list just read is the members count
	// The members already inside cost the group nothing and
	// the rollback above undoes the additions that do not fit
	// The group has no room for all the new members
	if len(chat.Members) > schemas.GroupMaxMembers {
		return schemas.ChatWithMembers{}, ErrGroupFull
	}

	// The chatType is a condition of the query above and never a value read back from it
	chat.Type = schemas.ChatTypeGroup

	if err := tx.Commit(); err != nil {
		return schemas.ChatWithMembers{}, fmt.Errorf("cannot commit the transaction: %w", err)
	}

	return chat, nil
}
