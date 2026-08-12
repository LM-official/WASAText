package database

import (
	"database/sql"
	"errors"

	"github.com/MercuriLorenzo/WASAText/service/schemas"
	"github.com/gofrs/uuid"
)

// DoLogin checks if a user with the given username exists in the database
// It returns the UserId, a boolean indicating if the user was found, and an error if any occurred
func (db *appdbimpl) DoLogin(username schemas.Username) (schemas.UserId, bool, error) {
	var id schemas.UserId

	// Search for the user in the database
	err := db.c.QueryRow(`SELECT id FROM users WHERE username = ?;`, username).Scan(&id)

	if err == nil {
		// User found, login
		return id, true, nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		// Error searching for the user (error != no row found, e.g. connection issue)
		return schemas.UserId(""), false, err
	}

	// User not found (missing row), create new user
	// Generate the new UUID
	newUUID, err := uuid.NewV4()
	if err != nil {
		// Error generating the UUID
		return schemas.UserId(""), false, err
	}
	newId := schemas.UserId(newUUID.String())

	// Insert the new user in the database
	// A new user starts with the default photo id
	_, err = db.c.Exec(`INSERT INTO users (id, username, photoId) VALUES (?, ?, ?);`, newId, username, schemas.DefaultPhotoId)
	if err != nil {
		// Error inserting the new user
		return schemas.UserId(""), false, err
	}

	return newId, false, nil
}
