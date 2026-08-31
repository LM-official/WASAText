package database

import (
	"database/sql"

	"github.com/MercuriLorenzo/WASAText/service/schemas"
)

// readMessageState computes a message's state from the members' progress in its chat
func readMessageState(tx *sql.Tx, chatId schemas.ChatId, messageDate string) (schemas.MessageState, error) {
	var isRead bool
	if err := tx.QueryRow(`SELECT NOT EXISTS (SELECT 1 FROM chat_members WHERE chatId = ? AND lastReadDate < ?);`,
		chatId, messageDate).Scan(&isRead); err != nil {
		return "", err
	}

	if isRead {
		return schemas.MessageStateRead, nil
	}
	return schemas.MessageStateReceived, nil
}

// advanceLastReadDate moves a member's progress forward without ever moving it backwards
// A missing member or an equal/older date is a successful no-op
func advanceLastReadDate(tx *sql.Tx, userId schemas.UserId, chatId schemas.ChatId, dateText string) error {
	_, err := tx.Exec(`UPDATE chat_members SET lastReadDate = ? WHERE chatId = ? AND userId = ? AND lastReadDate < ?;`,
		dateText, chatId, userId, dateText)
	return err
}
