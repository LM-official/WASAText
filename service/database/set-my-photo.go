package database

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/MercuriLorenzo/WASAText/service/schemas"
)

// SetMyPhoto sets newPhotoId as the photo of the user of the given id,
// and returns the updated user together with the id of the photo it replaced
// The old id is what lets the caller drop the file of that photo once nothing points at it
func (db *appdbimpl) SetMyPhoto(userId schemas.UserId, newPhotoId schemas.PhotoId) (schemas.User, schemas.PhotoId, error) {
	// Reading the old id and writing the new one must be one single step:
	// between the two, a concurrent update would make the caller delete the file of a photo that is now in use
	// so everything done in a transaction
	tx, err := db.c.Begin()
	if err != nil {
		return schemas.User{}, "", fmt.Errorf("cannot start the transaction: %w", err)
	}
	// Undo everything unless the commit below is reached
	defer func() { _ = tx.Rollback() }()

	// Get the photo that is being replaced
	var oldPhotoId schemas.PhotoId
	err = tx.QueryRow(`SELECT photoId FROM users WHERE id = ?;`, userId).Scan(&oldPhotoId)
	// No user owns that id
	if errors.Is(err, sql.ErrNoRows) {
		return schemas.User{}, "", ErrUserNotFound
	}
	// Error fetching the current photo
	if err != nil {
		return schemas.User{}, "", fmt.Errorf("cannot read the user %q: %w", userId, err)
	}

	_, err = tx.Exec(`UPDATE users SET photoId = ? WHERE id = ?;`, newPhotoId, userId)
	// Error during update
	if err != nil {
		return schemas.User{}, "", fmt.Errorf("error updating user photo: %w", err)
	}

	// Get the updated user
	var user schemas.User
	err = tx.QueryRow(`SELECT id, username, photoId FROM users WHERE id = ?;`, userId).Scan(&user.Id, &user.Username, &user.Photo)
	// Error fetching updated user
	if err != nil {
		return schemas.User{}, "", fmt.Errorf("cannot read the updated user %q: %w", userId, err)
	}

	if err := tx.Commit(); err != nil {
		return schemas.User{}, "", fmt.Errorf("cannot commit the transaction: %w", err)
	}

	return user, oldPhotoId, nil
}
