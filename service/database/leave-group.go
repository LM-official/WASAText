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
// It gives back the photos to release and never the group itself: the endpoint answers with no body
// The list is read only when the caller was the last member out and contains the group photo plus
// every distinct photo carried by its messages. While the group stands there is nothing to release
//
// It returns ErrChatNotFound if no group owns that id, and ErrNotAMember if the caller does not belong to it
func (db *appdbimpl) LeaveGroup(userId schemas.UserId, groupId schemas.ChatId) ([]schemas.PhotoId, error) {
	// The membership is read, then removed, and the row of the group is dropped against what is left of it:
	// all three must see the same group, or the last two members leaving at once would each still find
	// the other inside and neither would drop it
	tx, err := db.c.Begin()
	if err != nil {
		return nil, fmt.Errorf("cannot start the transaction: %w", err)
	}
	// Undo everything unless the commit below is reached
	defer func() { _ = tx.Rollback() }()

	// One row that answers both questions the call asks:
	// does a group own that id, and is the caller inside it
	var isMember bool
	err = tx.QueryRow(`SELECT EXISTS (SELECT 1 FROM chat_members WHERE chatId = c.id AND userId = ?)
						   FROM chats c WHERE c.id = ? AND c.chatType = ?;`,
		userId, groupId, schemas.ChatTypeGroup,
	).Scan(&isMember)
	// No group owns that id: it may not exist at all, or be a private chat
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrChatNotFound
	}
	// Error reading the group
	if err != nil {
		return nil, fmt.Errorf("cannot read the group: %w", err)
	}
	// The group is there, but leaving it belongs to its members
	if !isMember {
		return nil, ErrNotAMember
	}

	// Remove the caller from the group
	_, err = tx.Exec(`DELETE FROM chat_members WHERE chatId = ? AND userId = ?;`, groupId, userId)
	if err != nil {
		return nil, fmt.Errorf("cannot remove the caller from the group: %w", err)
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
		return nil, fmt.Errorf("cannot tell if the group still has members: %w", err)
	}

	var photosToRelease []schemas.PhotoId
	// Nobody is left inside: the group can never be reached again, so it is dropped,
	// and every photo reference that disappears with it becomes a candidate for release
	// This is deliberately inside the rare last-member branch: a normal leave does not read photos
	if !anyoneLeft {
		rows, err := tx.Query(`SELECT photoId FROM chats WHERE id = ?
						  	   UNION
						  	   SELECT photoId FROM messages WHERE chatId = ? AND photoId IS NOT NULL;`,
			groupId, groupId)
		// Error reading the photos of the empty group
		if err != nil {
			return nil, fmt.Errorf("cannot read the photos of the empty group: %w", err)
		}

		photosToRelease = make([]schemas.PhotoId, 0)
		for rows.Next() {
			var photoId schemas.PhotoId
			if err := rows.Scan(&photoId); err != nil {
				_ = rows.Close()
				return nil, fmt.Errorf("cannot read a photo of the empty group: %w", err)
			}

			photosToRelease = append(photosToRelease, photoId)
		}
		// Error reading the photos of the empty group
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("cannot read the photos of the empty group: %w", err)
		}

		// No need to check chatType: the first query let only a real group reach this point.
		// The delete cascades to messages and comments after their photo ids have been collected above.
		_, err = tx.Exec(`DELETE FROM chats WHERE id = ?;`, groupId)
		if err != nil {
			return nil, fmt.Errorf("cannot delete the empty group: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("cannot commit the transaction: %w", err)
	}

	return photosToRelease, nil
}
