package database

import (
	"errors"

	"github.com/MercuriLorenzo/WASAText/service/schemas"
	sqlite3 "github.com/mattn/go-sqlite3"
)

// SetMyUserName updates the username of the user of the given id, and returns the updated user
// It returns ErrUsernameTaken if another user already owns newUsername
//
// RETURNING hands the updated row back from the statement that wrote it: one statement instead of an
// UPDATE followed by a SELECT, so no other request can change the row between the write and the read
// and the user returned here is always the one this call stored
func (db *appdbimpl) SetMyUserName(userId schemas.UserId, newUsername schemas.Username) (schemas.User, error) {
	var user schemas.User
	err := db.c.QueryRow(`UPDATE users SET username = ? WHERE id = ?
						  RETURNING id, username, photoId;`, newUsername, userId).Scan(&user.Id, &user.Username, &user.Photo)
	if err != nil {
		var sqliteErr sqlite3.Error
		// The username column is UNIQUE: its constraint is the only expected failure,
		// and the api layer needs to answer 400 instead of 500 for it
		if errors.As(err, &sqliteErr) && sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique {
			return schemas.User{}, ErrUsernameTaken
		}

		// Error during update
		return schemas.User{}, err
	}

	return user, nil
}
