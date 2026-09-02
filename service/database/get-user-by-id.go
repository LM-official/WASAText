package database

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/MercuriLorenzo/WASAText/service/schemas"
)

// GetUserById returns the user that owns the given id
// It returns ErrUserNotFound if no user owns it
//
// GetUsers answers the search box, which only ever knows a username;
// this one answers everything that reads an id out of another resource:
// the sender of a message, the author of a comment, a member of a chat, where the id is all the client is given
// and the username is what it has to show
func (db *appdbimpl) GetUserById(userId schemas.UserId) (schemas.User, error) {
	var u schemas.User
	err := db.c.QueryRow(`SELECT id, username, photoId FROM users WHERE id = ?;`, userId).Scan(&u.Id, &u.Username, &u.Photo)
	// No user owns that id
	if errors.Is(err, sql.ErrNoRows) {
		return schemas.User{}, ErrUserNotFound
	}
	// Query error
	if err != nil {
		return schemas.User{}, fmt.Errorf("cannot get the user %q: %w", userId, err)
	}

	return u, nil
}
