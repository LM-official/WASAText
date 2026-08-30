package database

import (
	"database/sql"
	"fmt"

	"github.com/MercuriLorenzo/WASAText/service/globaltime"
	"github.com/MercuriLorenzo/WASAText/service/schemas"
)

// GetMyConversations returns the chats the user belongs to, the most recently active first,
// and at most schemas.UserChatsPageSize of them: the client scrolls down for the older ones
//
// A group owns its name and photo, a private chat borrows them from the other member:
// the row of a private chat holds NULL in both columns, so the pair is read from that member
// and never stored, or it would go stale on every setMyUserName and setMyPhoto
//
// Each chat carries the snippet of its last message, and none while it holds no message
func (db *appdbimpl) GetMyConversations(userId schemas.UserId) (schemas.ChatSummaries, error) {
	// 1. id and type, every chat stores them in a row
	// 2. name and photo, a group owns them, a private chat borrows them from the other member
	// 3. The last message, or none while the chat holds no message
	// 4. Message is read?
	// 5. Order by last message date, newest first, and id to break the tie
	rows, err := db.c.Query(`SELECT c.id, c.chatType,
									-- The chats CHECK holds one of the two sides NULL and the other (o) not,
									-- so this is a switch (gets the first not NULL value) on chatType and never a fallback:
									-- a group carries the pair it owns, a private chat the pair of the other member
									COALESCE(c.name, o.username)   AS name,
									COALESCE(c.photoId, o.photoId) AS photoId,
									-- The columns the snippet is built from, taken whole from the last message (lm)
									-- They are NULL together while the chat holds no message,
									-- which is what tells the caller apart from a message whose own text or photo is missing
									lm.id, lm.userId, lm.date, lm.text, lm.photoId,
									-- A message is read (rm) once no member of the chat is left behind it
									-- The sender needs no exception here: sending catches it up to its own message,
									-- so its date is never older than the one it wrote
									NOT EXISTS (SELECT 1 FROM chat_members AS rm
												WHERE rm.chatId = c.id AND rm.lastReadDate < lm.date) AS isRead
							 FROM chats AS c JOIN chat_members AS m ON m.chatId = c.id
							 -- The other member (om) of a private chat, which is where its name and photo come from
							 -- The chatType is part of the condition: a group has many other members and would come back once per member,
							 -- while a pair has exactly one and comes back once
							 LEFT JOIN chat_members AS om ON om.chatId = c.id AND c.chatType = ? AND om.userId <> ?
							 LEFT JOIN users AS o ON o.id = om.userId
							 -- The last message (lm) of the chat, and no row while the chat holds none
							 -- Two messages can share a second, so rowid breaks the tie by insertion order
							 LEFT JOIN messages AS lm ON lm.id = (SELECT id FROM messages WHERE chatId = c.id
																  ORDER BY date DESC, rowid DESC LIMIT 1)
							 WHERE m.userId = ?
							 -- Newest first: a chat with no message has no date, and SQLite sorts NULL last in DESC,
							 -- so it lands at the bottom of the list
							 -- The id breaks the tie, so the same chats come back in the same order every time
							 ORDER BY lm.date DESC, c.id
							 -- One page, and the list is already in the order the page is taken from:
							 -- the most recently active chats are the ones this read gives back
							 LIMIT ?;`,
		schemas.ChatTypePrivate, userId, userId, schemas.UserChatsPageSize,
	)
	// Query error
	if err != nil {
		return schemas.ChatSummaries{}, fmt.Errorf("cannot get the chats of user %q: %w", userId, err)
	}
	// Close rows when done even in case of error
	defer func() { _ = rows.Close() }()

	results := make(schemas.ChatSummaries, 0)
	for rows.Next() {
		var chat schemas.ChatSummary
		// The columns of the last message are NULL together while the chat holds no message
		var msgId, msgUser, msgDate, msgText, msgPhoto sql.NullString
		var isRead bool

		// Error reading a row: a shorter list would be a wrong answer, not a partial one
		if err := rows.Scan(&chat.Id, &chat.Type, &chat.Name, &chat.Photo,
			&msgId, &msgUser, &msgDate, &msgText, &msgPhoto, &isRead); err != nil {
			return schemas.ChatSummaries{}, fmt.Errorf("cannot read a chat of user %q: %w", userId, err)
		}
		// Invalid last message, snippet is nil by default
		if !msgId.Valid {
			results = append(results, chat)
			continue
		}
		// user and date mimic id so if the id is valid also the other two are
		// Valid last message
		date, err := globaltime.Parse(msgDate.String)
		// A date the schema cannot have written: the row is broken, not the request
		if err != nil {
			return schemas.ChatSummaries{}, fmt.Errorf("cannot read the date of the last message: %w", err)
		}

		// A message carries text, photo, or both, and so does its snippet:
		// the text is cut to a maximum fixed length and the photo stands as a symbol
		content := schemas.SnippetContent{}
		if msgText.Valid {
			content.Text = schemas.SnippetText(schemas.TruncateChars(msgText.String, schemas.SnippetMaxChars))
		}
		if msgPhoto.Valid {
			content.Emoji = schemas.SnippetPhotoEmoji
		}
		// No snippet content means no snippet
		if content.Text == "" && content.Emoji == "" {
			results = append(results, chat)
			continue
		}

		// Create the snippet from last message
		snippet := schemas.Snippet{
			MessageBase: schemas.MessageBase{
				Id:    schemas.MessageId(msgId.String),
				User:  schemas.UserId(msgUser.String),
				Date:  date,
				State: schemas.MessageStateReceived,
			},
			Content: content,
		}
		if isRead {
			snippet.State = schemas.MessageStateRead
		}

		chat.Snippet = &snippet
		results = append(results, chat)
	}
	// Error during rows iteration
	if err := rows.Err(); err != nil {
		return schemas.ChatSummaries{}, fmt.Errorf("cannot read the chats of user %q: %w", userId, err)
	}

	return results, nil
}
