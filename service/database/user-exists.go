package database

import (
	"database/sql"
	"errors"

	"github.com/MercuriLorenzo/WASAText/service/schemas"
)

func (db *appdbimpl) UserExists(userId schemas.UserId) (bool, error) {
	if err := userId.IsValid(); err != nil {
		// invalid userId format
		return false, err
	}

	err := db.c.QueryRow("SELECT 1 FROM users WHERE id=?", userId).Scan(new(int))
	// user does not exist
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}

	// other error
	if err != nil {
		return false, err
	}

	// user found
	return true, nil
}
