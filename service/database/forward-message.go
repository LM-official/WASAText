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

func (db *appdbimpl) ForwardMessage(userId schemas.UserId, chatId schemas.ChatId, messageId schemas.MessageId) (schemas.Message, error) {
	// The source message should be read then re-write to the destination chat
	// all of them must see the same chat
	tx, err := db.c.Begin()
	if err != nil {
		return schemas.Message{}, fmt.Errorf("cannot start the transaction: %w", err)
	}
	// Undo everything unless the commit below is reached
	defer func() { _ = tx.Rollback() }()

	// 1. does the source message exist, and is the caller a member of its chat
	// A plain WHERE is fine here, a missing message legitimately means "zero rows",
	// which matches the ErrNoRows pattern already used for the chat lookup in SendMessage
	// Also get the content of the source message
	// messages.chatId is a FOREIGN KEY on chats(id), so a message row existing already guarantees its chat exists too
	var srcChatId schemas.ChatId
	var srcMember bool
	var srcText, srcPhotoId sql.NullString
	err = tx.QueryRow(`SELECT m.chatId, m.text, m.photoId,
							  EXISTS(SELECT 1 FROM chat_members WHERE chatId = m.chatId AND userId = ?)
					   FROM messages AS m WHERE m.id = ?;`,
		userId, messageId).Scan(&srcChatId, &srcText, &srcPhotoId, &srcMember)
	// No message owns that id
	if errors.Is(err, sql.ErrNoRows) {
		return schemas.Message{}, ErrMessageNotFound
	}
	// Error reading the source message
	if err != nil {
		return schemas.Message{}, fmt.Errorf("cannot read the source message %q: %w", messageId, err)
	}
	// The message is there, but forwarding out of its chat belongs to its members
	if !srcMember {
		return schemas.Message{}, ErrNotAMember
	}

	// 2. does the destination chat exists, and is the caller a member of it
	var dstExists, dstMember bool
	err = tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM chats WHERE id = ?),
						  EXISTS(SELECT 1 FROM chat_members WHERE chatId = ? AND userId = ?);`,
		chatId, chatId, userId).Scan(&dstExists, &dstMember)
	// Error reading the destination chat
	if err != nil {
		return schemas.Message{}, fmt.Errorf("cannot read the destination chat %q: %w", chatId, err)
	}
	// No chat owns the destination id
	if !dstExists {
		return schemas.Message{}, ErrChatNotFound
	}
	// The chat is there, but forwarding into it belongs to its members
	if !dstMember {
		return schemas.Message{}, ErrNotAMember
	}

	// 3. a chat holds at most schemas.ChatMaxMessages messages
	// Counted inside the transaction and before the insert, so the row that would pass the cap is never written
	var held int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM messages WHERE chatId = ?;`, chatId).Scan(&held); err != nil {
		return schemas.Message{}, fmt.Errorf("cannot count the messages of the chat %q: %w", chatId, err)
	}
	// Chat is full, and the message cannot be written
	// ">" defends against db or ChatMaxMessages modifications
	if held >= schemas.ChatMaxMessages {
		return schemas.Message{}, ErrChatFull
	}

	// Generate the new UUID
	newUUID, err := uuid.NewV4()
	// Error generating the UUID
	if err != nil {
		return schemas.Message{}, fmt.Errorf("cannot generate the message id: %w", err)
	}
	newId := schemas.MessageId(newUUID.String())

	// The date is written into the message and into the lastReadDate of its sender, so it is taken once here:
	// the two must be the same string for message state consistency
	date := globaltime.Now().UTC().Truncate(time.Millisecond)
	dateText := globaltime.Format(date)

	// The forwarded message is a new row in the destination chat, carrying the same content as the source
	_, err = tx.Exec(`INSERT INTO messages (id, chatId, userId, text, photoId, date, forwarded) VALUES (?, ?, ?, ?, ?, ?, 1);`,
		newId, chatId, userId, srcText, srcPhotoId, dateText)
	// Error inserting the message
	if err != nil {
		return schemas.Message{}, fmt.Errorf("cannot write the forwarded message in the chat %q: %w", chatId, err)
	}

	// Writing in a chat means having seen what is above, so the sender is caught up to its own message
	// The value is the date of the message itself
	// The lastReadDate < guard prevents a clock rollback from moving the value backwards
	err = advanceLastReadDate(tx, userId, chatId, dateText)
	// Error updating the sender
	if err != nil {
		return schemas.Message{}, fmt.Errorf("cannot update the last read date of the sender: %w", err)
	}

	// The state of the message just written, is computed like every other:
	// read once no member of the chat is left behind it
	// It not always born "received": in a chat the sender is alone in,
	// the update above already caught up the only member there is, so the message is born "read"
	state, err := readMessageState(tx, chatId, dateText)
	// Error reading the state
	if err != nil {
		return schemas.Message{}, fmt.Errorf("cannot read the state of the message: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return schemas.Message{}, fmt.Errorf("cannot commit the transaction: %w", err)
	}

	message := schemas.Message{
		MessageBase: schemas.MessageBase{
			Id:        newId,
			User:      userId,
			Date:      date,
			State:     state,
			Forwarded: true,
		},
		Content: schemas.MessageContent{
			Text: schemas.MessageText(srcText.String),
			// The photo travels as the id it is stored as, and the api layer is what turns it into a URL
			Photo: schemas.PhotoURL(srcPhotoId.String),
		},
		// A message is born with no reaction, and the list is answered empty and never null
		Comments: make(schemas.Comments, 0),
	}
	return message, nil
}
