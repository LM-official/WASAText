package database

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/MercuriLorenzo/WASAText/service/globaltime"
	"github.com/MercuriLorenzo/WASAText/service/schemas"
)

// GetMessageComments returns every reaction left on a message
// Reading the comments of a message belongs to the members of its chat, so userId must be one of them
// Reading a message comments means having seen the chat as of now, so the caller's lastReadDate is caught up too to now
//
// It returns ErrMessageNotFound if no message owns that id,
// ErrChatNotFound if the message exists but does not belong to chatId,
// and ErrNotAMember if the caller does not belong to the message's chat.
func (db *appdbimpl) GetMessageComments(userId schemas.UserId, chatId schemas.ChatId, messageId schemas.MessageId) (schemas.Comments, error) {
	// The existence/membership check and the comments read must agree on the same snapshot of the world
	tx, err := db.c.Begin()
	if err != nil {
		return schemas.Comments{}, fmt.Errorf("cannot start the transaction: %w", err)
	}
	// Undo everything unless the commit below is reached
	defer func() { _ = tx.Rollback() }()

	// Does the message exists, does it belong to chatId, and is the caller a member of that chat?
	// messages.chatId is a FOREIGN KEY on chats(id), so a message row existing already guarantees its chat exists too
	// No separate chat-existence check is needed, same as CommentMessage
	var msgChatId schemas.ChatId
	var isMember bool
	err = tx.QueryRow(`SELECT m.chatId,
							  EXISTS(SELECT 1 FROM chat_members WHERE chatId = m.chatId AND userId = ?)
					   FROM messages AS m WHERE m.id = ?;`,
		userId, messageId).Scan(&msgChatId, &isMember)
	// No message owns that id
	if errors.Is(err, sql.ErrNoRows) {
		return schemas.Comments{}, ErrMessageNotFound
	}
	// Error reading the message
	if err != nil {
		return schemas.Comments{}, fmt.Errorf("cannot read the message %q: %w", messageId, err)
	}
	// The message exists, but not under chatId: treated the same as a chat that owns nothing at this id
	if msgChatId != chatId {
		return schemas.Comments{}, ErrChatNotFound
	}
	// The message is there and belongs to chatId, but reading its reactions belongs to members of that chat
	if !isMember {
		return schemas.Comments{}, ErrNotAMember
	}

	// Every reaction on the message, newest insertion first. Updating an emoji preserves its rowid and position;
	// deleting and recreating a comment gives it a new rowid and moves it to the front.
	// There is no separate comments cap or LIMIT to check: at most one row belongs to each member,
	// and a chat cannot have more than schemas.GroupMaxMembers members.
	// The UNIQUE(messageId, userId) constraint prevents a user from adding more than one comment to the same message,
	// and the ON CONFLICT clause updates the old comment instead of piling up
	rows, err := tx.Query(`SELECT id, userId, emoji FROM comments WHERE messageId = ? ORDER BY rowid DESC;`, messageId)
	// Error reading the comments
	if err != nil {
		return schemas.Comments{}, fmt.Errorf("cannot read the comments of the message %q: %w", messageId, err)
	}
	// Close rows when done even in case of error
	defer func() { _ = rows.Close() }()

	// An empty result is represented internally as [] rather than null; the API maps it to its collection-empty response.
	comments := make(schemas.Comments, 0)
	for rows.Next() {
		var c schemas.Comment
		// Error reading a row: a shorter list would be a wrong answer, not a partial one
		if err := rows.Scan(&c.Id, &c.User, &c.Emoji); err != nil {
			return schemas.Comments{}, fmt.Errorf("cannot read a comment of the message %q: %w", messageId, err)
		}

		comments = append(comments, c)
	}
	// Error during rows iteration
	if err := rows.Err(); err != nil {
		return schemas.Comments{}, fmt.Errorf("cannot read the comments of the message %q: %w", messageId, err)
	}

	// Reading a message comments means having seen the chat as of now: like commentMessage,
	// but has no date of its own to reuse, so a fresh timestamp is taken here
	// The lastReadDate < guard prevents a clock rollback from moving the value backwards
	now := globaltime.Format(globaltime.Now().UTC().Truncate(time.Millisecond))
	_, err = tx.Exec(`UPDATE chat_members SET lastReadDate = ?
					  WHERE chatId = ? AND userId = ? AND lastReadDate < ?;`,
		now, chatId, userId, now)
	// Error updating the caller
	if err != nil {
		return schemas.Comments{}, fmt.Errorf("cannot update the last read date of the caller: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return schemas.Comments{}, fmt.Errorf("cannot commit the transaction: %w", err)
	}

	return comments, nil
}
