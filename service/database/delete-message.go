package database

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/MercuriLorenzo/WASAText/service/globaltime"
	"github.com/MercuriLorenzo/WASAText/service/schemas"
)

// DeleteMessage removes a message from its chat, along with every comment on it
// Comments cascade away on their own: comments.messageId is a FOREIGN KEY ON DELETE CASCADE on messages(id),
// so nothing beyond the message row itself needs to be touched here
//
// It returns the photoId the deleted message carried, or "" if it carried none:
// the api layer is what decides whether to also delete the photo file, using ReleasePhoto,
// since a photo forwarded into other messages must survive the deletion of this one
//
// It returns ErrMessageNotFound if no message owns that id,
// ErrChatNotFound if the message exists but does not belong to chatId,
// ErrNotAMember if the caller does not belong to the message's chat,
// and ErrNotSender if the caller did not send the message
func (db *appdbimpl) DeleteMessage(userId schemas.UserId, chatId schemas.ChatId, messageId schemas.MessageId) (schemas.PhotoId, error) {
	// The existence/ownership/membership check and the delete must agree on the same snapshot of the world
	tx, err := db.c.Begin()
	if err != nil {
		return "", fmt.Errorf("cannot start the transaction: %w", err)
	}
	// Undo everything unless the commit below is reached
	defer func() { _ = tx.Rollback() }()

	// Does the message exist, does it belong to chatId, what photo (if any) does it carry, and is the caller a member of that chat?
	// messages.chatId is a FOREIGN KEY on chats(id), so a message row existing already guarantees its chat exists too
	// Who sent it is not read here: it belong to DELETE
	var msgChatId schemas.ChatId
	var msgPhotoId sql.NullString
	var isMember bool
	err = tx.QueryRow(`SELECT m.chatId, m.photoId,
							  EXISTS(SELECT 1 FROM chat_members WHERE chatId = m.chatId AND userId = ?)
					   FROM messages AS m WHERE m.id = ?;`,
		userId, messageId).Scan(&msgChatId, &msgPhotoId, &isMember)
	// No message owns that id
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrMessageNotFound
	}
	// Error reading the message
	if err != nil {
		return "", fmt.Errorf("cannot read the message %q: %w", messageId, err)
	}
	// The message exists, but not under chatId: treated the same as a chat that owns nothing at this id
	if msgChatId != chatId {
		return "", ErrChatNotFound
	}
	// The message is there and belongs to chatId, but deleting it still requires belonging to that chat
	if !isMember {
		return "", ErrNotAMember
	}

	// The userId = ? here is what makes deletion a sender-only action: the statement can only ever touch a row that belongs to the caller
	// Deleting the message row also drops every comment on it, through the CASCADE on comments.messageId
	res, err := tx.Exec(`DELETE FROM messages WHERE id = ? AND userId = ?;`, messageId, userId)
	// Error deleting the message
	if err != nil {
		return "", fmt.Errorf("cannot delete the message %q: %w", messageId, err)
	}
	affected, err := res.RowsAffected()
	// Error reading how many rows were removed
	if err != nil {
		return "", fmt.Errorf("cannot tell if the message was deleted: %w", err)
	}
	// The message was already found above, under chatId, with the caller a member of that chat:
	// the only reason left for the delete to touch nothing is that the caller is not who sent it
	if affected == 0 {
		return "", ErrNotSender
	}

	// Deleting means having seen the chat as of now: same as UncommentMessage,
	// delete has no date of its own to reuse, so a fresh timestamp is taken here
	// The lastReadDate < guard prevents a clock rollback from moving the value backwards
	now := globaltime.Format(globaltime.Now().UTC().Truncate(time.Millisecond))
	_, err = tx.Exec(`UPDATE chat_members SET lastReadDate = ?
					  WHERE chatId = ? AND userId = ? AND lastReadDate < ?;`,
		now, chatId, userId, now)
	// Error updating the caller
	if err != nil {
		return "", fmt.Errorf("cannot update the last read date of the caller: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("cannot commit the transaction: %w", err)
	}

	return schemas.PhotoId(msgPhotoId.String), nil
}
