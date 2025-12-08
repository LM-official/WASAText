package database

import (
	"github.com/MercuriLorenzo/WASAText/service/schemas"
)

// DoLogin checks if a user with the given username exists in the database.
// It returns the UserId, a boolean indicating if the user was found, and an error if any occurred.
func (db *appdbimpl) DoLogin(username schemas.Username) (schemas.UserId, bool, error) {
	var id schemas.UserId

	// search for the user in the database
	err := db.c.QueryRow(`SELECT id FROM users WHERE username = ?;`, string(username)).Scan(&id)

	if err == nil {
		// user found, login (HTTP 200)
		return id, true, nil
	}

	// user not found, create the new user (HTTP 201)
	// generates the new UUID
	newUUID, err := schemas.CreateUUID("UserId")
	if err != nil {
		return schemas.UserId(""), false, err
	}

	// insert the new user in the database
	_, err = db.c.Exec("INSERT INTO users (id, username, photo) VALUES (?, ?, ?)", newUUID, string(username), "db/defaultPhoto.png")
	if err != nil {
		return schemas.UserId(""), false, err
	}

	return schemas.UserId(newUUID), false, nil
}
