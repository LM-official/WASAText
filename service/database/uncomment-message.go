package database

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/MercuriLorenzo/WASAText/service/globaltime"
	"github.com/MercuriLorenzo/WASAText/service/schemas"
)

// UncommentMessage removes the caller's own reaction from a message
// Uncommenting belongs to whoever left the comment, so only the caller's own row is ever touched:
// UNIQUE(messageId, userId) guarantees there is at most one to remove
//
// Uncommenting means having seen the message and its own comment, like CommentMessage,
// so the caller's lastReadDate is caught up too to now
//
// It returns ErrMessageNotFound if no message owns that id,
// ErrChatNotFound if the message exists but does not belong to chatId,
// ErrNotAMember if the caller does not belong to the message's chat,
// and ErrCommentNotFound if the caller has no comment on that message
func (db *appdbimpl) UncommentMessage(userId schemas.UserId, chatId schemas.ChatId, messageId schemas.MessageId) error {
	// Both the existence/membership check and the delete must agree on the same snapshot of the world
	tx, err := db.c.Begin()
	if err != nil {
		return fmt.Errorf("cannot start the transaction: %w", err)
	}
	// Undo everything unless the commit below is reached
	defer func() { _ = tx.Rollback() }()

	// Does the message exist, does it belong to chatId, and is the caller a member of that chat?
	// messages.chatId is a FOREIGN KEY on chats(id), so a message row existing already guarantees its chat exists too
	// No separate chat-existence check is needed
	var msgChatId schemas.ChatId
	var isMember bool
	err = tx.QueryRow(`SELECT m.chatId,
							  EXISTS(SELECT 1 FROM chat_members WHERE chatId = m.chatId AND userId = ?)
					   FROM messages AS m WHERE m.id = ?;`,
		userId, messageId).Scan(&msgChatId, &isMember)
	// No message owns that id
	if errors.Is(err, sql.ErrNoRows) {
		return ErrMessageNotFound
	}
	// Error reading the message
	if err != nil {
		return fmt.Errorf("cannot read the message %q: %w", messageId, err)
	}
	// The message exists, but not under chatId: treated the same as a chat that owns nothing at this id
	if msgChatId != chatId {
		return ErrChatNotFound
	}
	// The message is there and belongs to chatId, but reacting (or un-reacting) to it belongs to members of that chat
	if !isMember {
		return ErrNotAMember
	}

	// Only the caller's own comment on this message is a candidate:
	// UNIQUE(messageId, userId) means this can delete at most one row, so RowsAffected alone tells whether one existed
	res, err := tx.Exec(`DELETE FROM comments WHERE messageId = ? AND userId = ?;`, messageId, userId)
	// Error removing the comment
	if err != nil {
		return fmt.Errorf("cannot remove the comment from the message %q: %w", messageId, err)
	}
	affected, err := res.RowsAffected()
	// Error reading how many rows were removed
	if err != nil {
		return fmt.Errorf("cannot tell if the comment was removed: %w", err)
	}
	// The caller never left a comment on this message, so there is nothing to uncomment
	if affected == 0 {
		return ErrCommentNotFound
	}

	// Uncommenting means having seen the chat as of now: same as CommentMessage,
	// but a comment removal has no date of its own to reuse, so a fresh timestamp is taken here
	// The lastReadDate < guard prevents a clock rollback from moving the value backwards
	now := globaltime.Format(globaltime.Now().UTC().Truncate(time.Millisecond))
	err = advanceLastReadDate(tx, userId, chatId, now)
	// Error updating the caller
	if err != nil {
		return fmt.Errorf("cannot update the last read date of the caller: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("cannot commit the transaction: %w", err)
	}

	return nil
}
