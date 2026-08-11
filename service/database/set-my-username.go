package database

import (
	"errors"

	"github.com/MercuriLorenzo/WASAText/service/schemas"
	sqlite3 "github.com/mattn/go-sqlite3"
)

// SetMyUserName updates the username of the user of the given id, and returns the updated user
// It returns ErrUsernameTaken if another user already owns newUsername
func (db *appdbimpl) SetMyUserName(userId schemas.UserId, newUsername schemas.Username) (schemas.User, error) {
	_, err := db.c.Exec(`UPDATE users SET username = ? WHERE id = ?`, newUsername, userId)
	if err != nil {
		// The username column is UNIQUE: its constraint is the only expected failure,
		// and the api layer needs to answer 400 instead of 500 for it
		var sqliteErr sqlite3.Error
		if errors.As(err, &sqliteErr) && sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique {
			return schemas.User{}, ErrUsernameTaken
		}

		// Error during update
		return schemas.User{}, err
	}

	// Get the updated user
	var user schemas.User
	err = db.c.QueryRow(`SELECT id, username, photo FROM users WHERE id = ?`, userId).Scan(&user.Id, &user.Username, &user.Photo)
	if err != nil {
		// Error fetching updated user
		return schemas.User{}, err
	}

	return user, nil
}
