package database

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/MercuriLorenzo/WASAText/service/schemas"
)

// LeaveGroup removes the caller from the group
// Leaving a group belongs to its members, so userId must be one of them
//
// A group nobody belongs to can never be reached again, so the last member out drops it:
// its messages go with it through the ON DELETE CASCADE of the messages table
//
// It gives back the photo to release and never the group itself: the endpoint answers with no body
// That photo is the one the group showed, and only when the caller was the last one out:
// while the group stands its own row still points at it, so there is never anything to release
//
// It returns ErrChatNotFound if no group owns that id, and ErrNotAMember if the caller does not belong to it
func (db *appdbimpl) LeaveGroup(userId schemas.UserId, groupId schemas.ChatId) (schemas.PhotoId, error) {
	// The membership is read, then removed, and the row of the group is dropped against what is left of it:
	// all three must see the same group, or the last two members leaving at once would each still find
	// the other inside and neither would drop it
	tx, err := db.c.Begin()
	if err != nil {
		return "", fmt.Errorf("cannot start the transaction: %w", err)
	}
	// Undo everything unless the commit below is reached
	defer func() { _ = tx.Rollback() }()

	// One row that answers both questions the call asks, and carries the value the caller needs:
	// does a group own that id, is the caller inside it, and which photo the group shows
	var photoId schemas.PhotoId
	var isMember bool
	err = tx.QueryRow(`SELECT c.photoId,
					   EXISTS (SELECT 1 FROM chat_members WHERE chatId = c.id AND userId = ?)
					   FROM chats c WHERE c.id = ? AND c.chatType = ?;`,
		userId, groupId, schemas.ChatTypeGroup,
	).Scan(&photoId, &isMember)

	// No group owns that id: it may not exist at all, or be a private chat
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrChatNotFound
	}
	// Error reading the group
	if err != nil {
		return "", fmt.Errorf("cannot read the group: %w", err)
	}
	// The group is there, but leaving it belongs to its members
	if !isMember {
		return "", ErrNotAMember
	}

	// Remove the caller from the group
	_, err = tx.Exec(`DELETE FROM chat_members WHERE chatId = ? AND userId = ?;`, groupId, userId)
	if err != nil {
		return "", fmt.Errorf("cannot remove the caller from the group: %w", err)
	}

	// Whether anybody is left is the whole question:
	// asked after the DELETE, so it is about what the caller leaves behind and never counts the caller
	// One membership answers it, so LIMIT 1 stops the scan at the first one
	var anyoneLeft = true
	err = tx.QueryRow(`SELECT 1 FROM chat_members WHERE chatId = ? LIMIT 1;`, groupId).Scan(new(int))
	// No row is the answer of this read and not a failure of it: nobody is left inside
	if errors.Is(err, sql.ErrNoRows) {
		anyoneLeft = false
	} else if err != nil {
		return "", fmt.Errorf("cannot tell if the group still has members: %w", err)
	}

	// Nobody is left inside: the group can never be reached again, so it is dropped,
	// and the photo it showed becomes a candidate for release
	if !anyoneLeft {
		// Do not need to check chatType because the groupId is used above and pass trought only if it is a real groupId
		_, err = tx.Exec(`DELETE FROM chats WHERE id = ?;`, groupId)
		if err != nil {
			return "", fmt.Errorf("cannot delete the empty group: %w", err)
		}
	} else {
		// The group stands, and its own row still points at that photo: nothing could release it,
		// whatever else may share the same file, so the caller is given nothing to try
		photoId = ""
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("cannot commit the transaction: %w", err)
	}

	return photoId, nil
}
