package database

import (
	"database/sql"
	"errors"

	"github.com/MercuriLorenzo/WASAText/service/schemas"
	"github.com/gofrs/uuid"
)

// DoLogin checks if a user with the given username exists in the database.
// It returns the UserId, a boolean indicating if the user was found, and an error if any occurred.
func (db *appdbimpl) DoLogin(username schemas.Username) (schemas.UserId, bool, error) {
	var id schemas.UserId

	// search for the user in the database
	err := db.c.QueryRow(`SELECT id FROM users WHERE username = ?;`, string(username)).Scan(&id)

	if err == nil {
		// user found, login
		return id, true, nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		// error searching for the user (error != no row found, eg. connection issue)
		return schemas.UserId(""), false, err
	}

	// user not found (missing row), create new user
	// generate the new UUID
	newUUID, err := uuid.NewV4()
	if err != nil {
		// error generating the UUID
		return schemas.UserId(""), false, err
	}

	// insert the new user in the database
	_, err = db.c.Exec("INSERT INTO users (id, username, photo) VALUES (?, ?, ?)", newUUID.String(), string(username), "./db/defaultPhoto.png")
	if err != nil {
		// error inserting the new user
		return schemas.UserId(""), false, err
	}

	return schemas.UserId(newUUID.String()), false, nil
}
