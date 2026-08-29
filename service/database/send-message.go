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

// SendMessage writes a message in the chat of the given id and returns it
// Writing in a chat belongs to its members, so userId must be one of them
// Both kinds answer here: a private chat and a group are written the same way
//
// text empty means the message carries no text, photoId empty means it carries no photo:
// the api layer refuses the pair that is empty on both sides, and the CHECK of the table is the last guard
//
// It returns ErrChatNotFound if no chat owns that id, ErrNotAMember if the caller does not belong to it,
// and ErrChatFull if the chat already holds schemas.ChatMaxMessages messages
func (db *appdbimpl) SendMessage(userId schemas.UserId, chatId schemas.ChatId, text schemas.MessageText, photoId schemas.PhotoId) (schemas.Message, error) {
	// The count is read and then written against, and the state is read after the insert:
	// all of them must see the same chat
	tx, err := db.c.Begin()
	if err != nil {
		return schemas.Message{}, fmt.Errorf("cannot start the transaction: %w", err)
	}
	// Undo everything unless the commit below is reached
	defer func() { _ = tx.Rollback() }()

	// One row that answers both questions the call asks:
	// does a chat own that id, and is the caller inside it
	// Membership is a column and not a condition of the query:
	// a chat the caller does not belong to must still come back, or a 403 would read as a 404
	var isMember bool
	err = tx.QueryRow(`SELECT EXISTS (SELECT 1 FROM chat_members WHERE chatId = c.id AND userId = ?)
					   FROM chats AS c WHERE c.id = ?;`,
		userId, chatId).Scan(&isMember)
	// No chat owns that id
	if errors.Is(err, sql.ErrNoRows) {
		return schemas.Message{}, ErrChatNotFound
	}
	// Error reading the chat
	if err != nil {
		return schemas.Message{}, fmt.Errorf("cannot read the chat %q: %w", chatId, err)
	}
	// The chat is there, but writing in it belongs to its members
	if !isMember {
		return schemas.Message{}, ErrNotAMember
	}

	// A chat holds at most schemas.ChatMaxMessages messages
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
	dateText := date.Format(dateFormat)

	// An absent text and an absent photo reach the column as NULL and never as an empty string:
	// both NULL leave a row with nothing to preview
	_, err = tx.Exec(`INSERT INTO messages (id, chatId, userId, text, photoId, date) VALUES (?, ?, ?, ?, ?, ?);`,
		newId, chatId, userId, schemas.NullIfEmpty(string(text)), schemas.NullIfEmpty(string(photoId)), dateText)
	// Error inserting the message
	if err != nil {
		return schemas.Message{}, fmt.Errorf("cannot write the message in the chat %q: %w", chatId, err)
	}

	// Writing in a chat means having seen what is above, so the sender is caught up to its own message
	// The value is the date of the message itself
	// lastReadDate > prevents from breake time with manual set date, e.g. rollback the clock
	_, err = tx.Exec(`UPDATE chat_members SET lastReadDate = ?
					  WHERE chatId = ? AND userId = ? AND lastReadDate < ?;`,
		dateText, chatId, userId, dateText)
	// Error updating the sender
	if err != nil {
		return schemas.Message{}, fmt.Errorf("cannot update the last read date of the sender: %w", err)
	}

	// The state of the message just written, is computed like every other:
	// read once no member of the chat is left behind it
	// It not always born "received": in a chat the sender is alone in,
	// the update above already caught up the only member there is, so the message is born "read"
	var isRead bool
	err = tx.QueryRow(`SELECT NOT EXISTS (SELECT 1 FROM chat_members WHERE chatId = ? AND lastReadDate < ?);`,
		chatId, dateText).Scan(&isRead)
	// Error reading the state
	if err != nil {
		return schemas.Message{}, fmt.Errorf("cannot read the state of the message: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return schemas.Message{}, fmt.Errorf("cannot commit the transaction: %w", err)
	}

	message := schemas.Message{
		MessageBase: schemas.MessageBase{
			Id:    newId,
			User:  userId,
			Date:  date,
			State: schemas.MessageStateReceived,
		},
		Content: schemas.MessageContent{
			Text: text,
			// The photo travels as the id it is stored as, and the api layer is what turns it into a URL
			Photo: schemas.PhotoURL(photoId),
		},
		// A message is born with no reaction, and the list is answered empty and never null
		Comments: make(schemas.Comments, 0),
	}
	if isRead {
		message.State = schemas.MessageStateRead
	}

	return message, nil
}
