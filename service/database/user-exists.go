package database

import (
	"database/sql"
	"errors"

	"github.com/MercuriLorenzo/WASAText/service/schemas"
)

// UserExists reports whether a user is registered under the given id
// It returns true if the user exists, false if it does not exist, and an error if the check could not be performed
func (db *appdbimpl) UserExists(userId schemas.UserId) (bool, error) {
	// Ignore return value, just keep the error
	err := db.c.QueryRow(`SELECT 1 FROM users WHERE id = ?;`, userId).Scan(new(int))
	// User does not exist
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}

	// Other error
	if err != nil {
		return false, err
	}

	// User found
	return true, nil
}
