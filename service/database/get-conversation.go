package database

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/MercuriLorenzo/WASAText/service/globaltime"
	"github.com/MercuriLorenzo/WASAText/service/schemas"
)

// GetConversation returns the chat of the given id with its members and its whole messages list
// Reading a chat belongs to its members, so userId must be one of them
//
// Opening a chat is what reading it means, so this marks the chat read for the caller:
// it is the only read of this package that writes, and the one that lets a member who never writes catch up with what it was sent
// The states it answers with are the ones taken before that write,
// so a message is handed over in the state it was in when the chat was asked for
//
// It returns ErrChatNotFound if no chat owns that id, and ErrNotAMember if the caller does not belong to it
func (db *appdbimpl) GetConversation(userId schemas.UserId, chatId schemas.ChatId) (schemas.ChatDetail, error) {
	// The chat, its members and its messages are three reads,
	// and a message sent between the first and the last would reach the list while
	// its sender is still missing from the members read before it:
	// one transaction is what makes the three see the same chat, and what holds the write below with them
	tx, err := db.c.Begin()
	if err != nil {
		return schemas.ChatDetail{}, fmt.Errorf("cannot start the transaction: %w", err)
	}
	// Undo everything unless the commit below is reached
	defer func() { _ = tx.Rollback() }()

	// One row that answers both questions the call asks, and carries what the reply needs:
	// does a chat own that id, and is the caller inside it
	var chat schemas.ChatDetail
	var isMember bool
	err = tx.QueryRow(`SELECT c.id, c.chatType,
							  -- The chats CHECK holds one of the two sides NULL and the other (o) not,
							  -- so this is a switch (gets the first not NULL value) on chatType and never a fallback:
							  -- a group carries the name and the photo it owns, a private chat the ones of the other member
							  COALESCE(c.name, o.username)   AS name,
							  COALESCE(c.photoId, o.photoId) AS photoId,
							  -- Membership is a column and not a condition of the query:
							  -- a chat the caller does not belong to must still come back, or a 403 would read as a 404
							  EXISTS (SELECT 1 FROM chat_members WHERE chatId = c.id AND userId = ?) AS isMember
					   FROM chats AS c
					   -- The other member (om) of a private chat, which is where its name and photo come from
					   -- The chatType is part of the condition: a group has many other members and would come back once per member,
					   -- while a pair has exactly one and comes back once
					   -- The caller is the one left out, so while it is not in the pair both members answer it and
					   -- the name and the photo are whichever of the two came first: that row is a 403 and is never read
					   LEFT JOIN chat_members AS om ON om.chatId = c.id AND c.chatType = ? AND om.userId <> ?
					   LEFT JOIN users AS o ON o.id = om.userId
					   WHERE c.id = ?;`,
		userId, schemas.ChatTypePrivate, userId, chatId,
	).Scan(&chat.Id, &chat.Type, &chat.Name, &chat.Photo, &isMember)
	// No chat owns that id
	if errors.Is(err, sql.ErrNoRows) {
		return schemas.ChatDetail{}, ErrChatNotFound
	}
	// Error reading the chat
	if err != nil {
		return schemas.ChatDetail{}, fmt.Errorf("cannot read the chat %q: %w", chatId, err)
	}
	// The chat is there, but reading it belongs to its members
	if !isMember {
		return schemas.ChatDetail{}, ErrNotAMember
	}

	// The caller is a member, so the chat has at least one member
	// Memberships carry IDs; profiles are resolved separately.
	memberRows, err := tx.Query(`SELECT cm.userId FROM chat_members AS cm
								 WHERE cm.chatId = ?;`, chatId)
	// Error reading the members
	if err != nil {
		return schemas.ChatDetail{}, fmt.Errorf("cannot read the members of the chat %q: %w", chatId, err)
	}
	// Close rows when done even in case of error
	defer func() { _ = memberRows.Close() }()

	chat.Members = make(schemas.Members, 0)
	for memberRows.Next() {
		var member schemas.UserId
		// Error reading a row: a shorter list would be a wrong answer, not a partial one
		if err := memberRows.Scan(&member); err != nil {
			return schemas.ChatDetail{}, fmt.Errorf("cannot read a member of the chat %q: %w", chatId, err)
		}

		chat.Members = append(chat.Members, member)
	}
	// Error during rows iteration
	if err := memberRows.Err(); err != nil {
		return schemas.ChatDetail{}, fmt.Errorf("cannot read the members of the chat %q: %w", chatId, err)
	}

	// A chat holds up to schemas.ChatMaxMessages and one read answers with schemas.ChatMessagesPageSize of them:
	// the client scrolls up for the older ones
	// Newest first, which is both how the page is taken and how a chat is opened:
	// the client anchors on the last message, and the page is given back in the order that chose it
	// The query walks the (chatId, date) index backwards and stops at the page,
	// so a chat of any length is read in the same bounded time,
	// and rowid breaks the tie by insertion order between two messages that share an instant
	messageRows, err := tx.Query(`SELECT m.id, m.userId, m.date, m.text, m.photoId, m.replyTo, m.forwarded,
										 -- A message is read (rm) once no member of the chat is left behind it,
										 -- which is a fact about the members and not about the caller:
										 -- the same message reads the same to everybody
										 -- The sender needs no exception here: sending catches it up to its own message,
										 -- so its date is never older than the one it wrote
										 NOT EXISTS (SELECT 1 FROM chat_members AS rm
													 WHERE rm.chatId = m.chatId AND rm.lastReadDate < m.date) AS isRead
								  FROM messages AS m WHERE m.chatId = ?
								  ORDER BY m.date DESC, m.rowid DESC LIMIT ?;`, chatId, schemas.ChatMessagesPageSize)
	// Error reading the messages
	if err != nil {
		return schemas.ChatDetail{}, fmt.Errorf("cannot read the messages of the chat %q: %w", chatId, err)
	}
	// Close rows when done even in case of error
	defer func() { _ = messageRows.Close() }()

	// Where each message sits in the list built here,
	// so that the comments read right after reach the message they belong to without walking the list once per comment
	index := make(map[schemas.MessageId]int)
	chat.Messages = make(schemas.Messages, 0)
	for messageRows.Next() {
		// A message carries text, photo, or both, so one of the two columns may be NULL but never both
		var msgDate string
		var msgText, msgPhoto, msgReplyTo sql.NullString
		var isRead bool
		var message schemas.Message

		// Error reading a row: a shorter list would be a wrong answer, not a partial one
		if err := messageRows.Scan(&message.Id, &message.User, &msgDate, &msgText, &msgPhoto, &msgReplyTo, &message.Forwarded, &isRead); err != nil {
			return schemas.ChatDetail{}, fmt.Errorf("cannot read a message of the chat %q: %w", chatId, err)
		}
		// A date the schema cannot have written: the row is broken, not the request
		message.Date, err = globaltime.Parse(msgDate)
		if err != nil {
			return schemas.ChatDetail{}, fmt.Errorf("cannot read the date of the message %q: %w", message.Id, err)
		}

		if msgText.Valid {
			message.Content.Text = schemas.MessageText(msgText.String)
		}
		// The photo travels as the id it is stored as, and the api layer is what turns it into a URL
		if msgPhoto.Valid {
			message.Content.Photo = schemas.PhotoURL(msgPhoto.String)
		}
		// NULL where the message answers none, and where the message it answered has been deleted:
		// the foreign key clears the column instead of taking this row with it
		if msgReplyTo.Valid {
			message.ReplyTo = schemas.MessageId(msgReplyTo.String)
		}

		if isRead {
			message.State = schemas.MessageStateRead
		} else {
			message.State = schemas.MessageStateReceived
		}

		// The reactions are read right below and appended here:
		// a message that carries none is answered with an empty list and never with null
		message.Comments = make(schemas.Comments, 0)

		// Slot this message is about to take
		// e.g. message 0: len = 0, so index[message.Id] = 0, then append
		// message 1: len = 1, so index[message.Id] = 1, then append
		index[message.Id] = len(chat.Messages)
		chat.Messages = append(chat.Messages, message)

	}
	// Error during rows iteration
	if err := messageRows.Err(); err != nil {
		return schemas.ChatDetail{}, fmt.Errorf("cannot read the messages of the chat %q: %w", chatId, err)
	}

	// Every comment to the page messages, each landing on the message it belongs to
	// The join repeats the page of the query above and not the whole chat:
	// a reaction on a message older than the page belongs to no message of this answer,
	// and reading it would be reading the reactions of a history that was left out
	commentRows, err := tx.Query(`SELECT cm.messageId, cm.id, cm.userId, cm.emoji
								  FROM comments AS cm
								  JOIN (SELECT m.id FROM messages AS m
										WHERE m.chatId = ?
										ORDER BY m.date DESC, m.rowid DESC LIMIT ?)
										AS m ON m.id = cm.messageId;`, chatId, schemas.ChatMessagesPageSize)
	// Error reading the comments
	if err != nil {
		return schemas.ChatDetail{}, fmt.Errorf("cannot read the comments of the chat %q: %w", chatId, err)
	}
	// Close rows when done even in case of error
	defer func() { _ = commentRows.Close() }()

	for commentRows.Next() {
		var messageId schemas.MessageId
		var comment schemas.Comment
		// Error reading a row: a message would lose a reaction it actually carries
		if err := commentRows.Scan(&messageId, &comment.Id, &comment.User, &comment.Emoji); err != nil {
			return schemas.ChatDetail{}, fmt.Errorf("cannot read a comment of the chat %q: %w", chatId, err)
		}

		// This query holds the comments on the messages of this chat and the one above holds those messages,
		// so the index answers every one of them because all the queries are inside a transaction
		i := index[messageId]
		chat.Messages[i].Comments = append(chat.Messages[i].Comments, comment)
	}
	// Error during rows iteration
	if err := commentRows.Err(); err != nil {
		return schemas.ChatDetail{}, fmt.Errorf("cannot read the comments of the chat %q: %w", chatId, err)
	}

	// Opening a chat is what reading it means, so the caller is caught up to now
	// The chat exists and the caller is a member, already checked above
	// lastReadDate > prevents from breake time with manual set date, e.g. rollback the clock
	now := globaltime.Format(globaltime.Now().UTC().Truncate(time.Millisecond))
	err = advanceLastReadDate(tx, userId, chatId, now)
	// Error updating the caller
	if err != nil {
		return schemas.ChatDetail{}, fmt.Errorf("cannot update the last read date of the caller: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return schemas.ChatDetail{}, fmt.Errorf("cannot commit the transaction: %w", err)
	}

	return chat, nil
}
