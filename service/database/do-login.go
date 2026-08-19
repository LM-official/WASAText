package database

import (
	"github.com/MercuriLorenzo/WASAText/service/schemas"
	"github.com/gofrs/uuid"
)

// DoLogin returns the id of the user of the given username, registering the user if the username is new
// The boolean is true when the user was already registered, so that the api layer can answer 200 instead of 201
func (db *appdbimpl) DoLogin(username schemas.Username) (schemas.UserId, bool, error) {
	// Generate the new UUID
	newUUID, err := uuid.NewV4()
	// Error generating the UUID
	if err != nil {
		return schemas.UserId(""), false, err
	}
	newId := schemas.UserId(newUUID.String())

	// A new user starts with the default photo id
	// DO NOTHING leaves the username to the UNIQUE on that column:
	// a username that is already registered writes no row and raises no error,
	// so a second request asking for the same new username is a login and never a failure
	res, err := db.c.Exec(`INSERT INTO users (id, username, photoId) VALUES (?, ?, ?)
						   ON CONFLICT(username) DO NOTHING;`, newId, username, schemas.DefaultPhotoId)
	// Error inserting the new user
	if err != nil {
		return schemas.UserId(""), false, err
	}

	written, err := res.RowsAffected()
	if err != nil {
		return schemas.UserId(""), false, err
	}
	// No row written: the username is already taken by a user, and that user is the one logging in
	if written == 0 {
		var id schemas.UserId
		// Error searching for the user
		if err := db.c.QueryRow(`SELECT id FROM users WHERE username = ?;`, username).Scan(&id); err != nil {
			return schemas.UserId(""), false, err
		}
		// User found, login
		return id, true, nil
	}

	// User registered
	return newId, false, nil
}
