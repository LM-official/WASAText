package database

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/MercuriLorenzo/WASAText/service/globaltime"
	"github.com/MercuriLorenzo/WASAText/service/schemas"
	"github.com/gofrs/uuid"
)

// CommentMessage adds the caller's reaction to a message,
// updating the emoji of any earlier reaction of the same caller on the same message while preserving its id.
// A comment is unique per (message, user), and the method returns the message with its up-to-date comment list.
// The boolean is true when the comment already existed, so the API can answer 200 instead of 201.
//
// Commenting means having seen the chat, like sending a message into it,
// so the caller's lastReadDate is caught up too to now.
//
// It returns ErrMessageNotFound if no message owns that id,
// ErrChatNotFound if the message exists but does not belong to chatId,
// and ErrNotAMember if the caller does not belong to the message's chat.
func (db *appdbimpl) CommentMessage(userId schemas.UserId, chatId schemas.ChatId, messageId schemas.MessageId, emoji schemas.Emoji) (schemas.Message, bool, error) {
	// Since done multiple reads and writes on the db, the transaction is used to ensure atomicity and consistency, and to avoid race conditions
	tx, err := db.c.Begin()
	if err != nil {
		return schemas.Message{}, false, fmt.Errorf("cannot start the transaction: %w", err)
	}
	// Undo everything unless the commit below is reached
	defer func() { _ = tx.Rollback() }()

	// Does the message exist, does it belong to chatId, and is the caller a member of that chat?
	// messages.chatId is a FOREIGN KEY on chats(id), so a message row existing already guarantees its chat exists too
	// No separate chat-existence check is needed, same as the source message check in ForwardMessage
	// Also pull everything needed later to rebuild the schemas.Message: its author, text, photo, date
	var msgChatId schemas.ChatId
	var msgUser schemas.UserId
	var msgText, msgPhotoId sql.NullString
	var msgDateText string
	var isMember bool
	err = tx.QueryRow(`SELECT m.chatId, m.userId, m.text, m.photoId, m.date,
							  EXISTS(SELECT 1 FROM chat_members WHERE chatId = m.chatId AND userId = ?)
					   FROM messages AS m WHERE m.id = ?;`,
		userId, messageId).Scan(&msgChatId, &msgUser, &msgText, &msgPhotoId, &msgDateText, &isMember)
	// No message owns that id
	if errors.Is(err, sql.ErrNoRows) {
		return schemas.Message{}, false, ErrMessageNotFound
	}
	// Error reading the message
	if err != nil {
		return schemas.Message{}, false, fmt.Errorf("cannot read the message %q: %w", messageId, err)
	}
	// Parse before any write: a broken stored date must not commit a comment and then return an error
	date, err := globaltime.Parse(msgDateText)
	if err != nil {
		return schemas.Message{}, false, fmt.Errorf("cannot parse the date of the message %q: %w", messageId, err)
	}
	// The message exists, but not under chatId: the URL's claim about which chat it belongs to does not match reality,
	// so it is treated the same as a chat that owns nothing at this id
	if msgChatId != chatId {
		return schemas.Message{}, false, ErrChatNotFound
	}
	// The message is there and belongs to chatId, but reacting to it belongs to members of that chat
	if !isMember {
		return schemas.Message{}, false, ErrNotAMember
	}

	// There is no separate comments cap to check: at most one row belongs to each member,
	// and a chat cannot have more than schemas.GroupMaxMembers members
	// The UNIQUE(messageId, userId) constraint prevents a user from adding more than one comment to the same message,
	// and the ON CONFLICT clause updates the old comment instead of piling up

	// Generate the new UUID
	newUUID, err := uuid.NewV4()
	// Error generating the UUID
	if err != nil {
		return schemas.Message{}, false, fmt.Errorf("cannot generate the comment id: %w", err)
	}
	newId := schemas.CommentId(newUUID.String())

	// A comment is per (message, user): a second call from the same user updates its emoji without changing its identity
	// RETURNING yields newId after an insert and the existing id after a conflict, distinguishing 201 from 200 atomically
	var storedId schemas.CommentId
	err = tx.QueryRow(`INSERT INTO comments (id, messageId, userId, emoji) VALUES (?, ?, ?, ?)
				     ON CONFLICT(messageId, userId) DO UPDATE SET emoji = excluded.emoji
				     RETURNING id;`,
		newId, messageId, userId, emoji).Scan(&storedId)
	// Error writing the comment
	if err != nil {
		return schemas.Message{}, false, fmt.Errorf("cannot write the comment on the message %q: %w", messageId, err)
	}
	alreadyExisted := storedId != newId

	// Commenting means having seen the chat as of now: like SendMessage/ForwardMessage,
	// but a comment has no date of its own to reuse, so a fresh timestamp is taken here
	// The lastReadDate < guard prevents a clock rollback from moving the value backwards
	now := globaltime.Format(globaltime.Now().UTC().Truncate(time.Millisecond))
	_, err = tx.Exec(`UPDATE chat_members SET lastReadDate = ?
					  WHERE chatId = ? AND userId = ? AND lastReadDate < ?;`,
		now, chatId, userId, now)
	// Error updating the caller
	if err != nil {
		return schemas.Message{}, false, fmt.Errorf("cannot update the last read date of the commenter: %w", err)
	}

	// The state of the message is computed unlike SendMessage/ForwardMessage
	// The sender is caught up above
	var isRead bool
	err = tx.QueryRow(`SELECT NOT EXISTS (SELECT 1 FROM chat_members WHERE chatId = ? AND lastReadDate < ?);`,
		chatId, msgDateText).Scan(&isRead)
	// Error reading the state
	if err != nil {
		return schemas.Message{}, false, fmt.Errorf("cannot read the state of the message: %w", err)
	}

	// The comments list returned must reflect every reaction on the message, not just the caller's own
	rows, err := tx.Query(`SELECT id, userId, emoji FROM comments WHERE messageId = ? ORDER BY rowid;`, messageId)
	// Error reading the comments
	if err != nil {
		return schemas.Message{}, false, fmt.Errorf("cannot read the comments of the message %q: %w", messageId, err)
	}
	// Close rows when done even in case of error
	defer func() { _ = rows.Close() }()

	// Always ends with at least the caller's own just-written comment,
	// but declared like SendMessage/ForwardMessage so the field serializes as [] and never null
	comments := make(schemas.Comments, 0)
	for rows.Next() {
		var c schemas.Comment
		// Error reading a row: a shorter list would be a wrong answer, not a partial one
		if err := rows.Scan(&c.Id, &c.User, &c.Emoji); err != nil {
			return schemas.Message{}, false, fmt.Errorf("cannot read a comment of the message %q: %w", messageId, err)
		}

		comments = append(comments, c)
	}
	// Error during rows iteration
	if err := rows.Err(); err != nil {
		return schemas.Message{}, false, fmt.Errorf("cannot read the comments of the message %q: %w", messageId, err)
	}

	if err := tx.Commit(); err != nil {
		return schemas.Message{}, false, fmt.Errorf("cannot commit the transaction: %w", err)
	}

	message := schemas.Message{
		MessageBase: schemas.MessageBase{
			Id:    messageId,
			User:  msgUser,
			Date:  date,
			State: schemas.MessageStateReceived,
		},
		Content: schemas.MessageContent{
			Text:  schemas.MessageText(msgText.String),
			Photo: schemas.PhotoURL(msgPhotoId.String),
		},
		Comments: comments,
	}
	if isRead {
		message.State = schemas.MessageStateRead
	}

	return message, alreadyExisted, nil
}
