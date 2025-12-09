package database

import (
	"database/sql"
	"errors"

	"github.com/MercuriLorenzo/WASAText/service/schemas"
)

func (db *appdbimpl) UserExists(userId schemas.UserId) (bool, error) {
	var temp int

	err := db.c.QueryRow("SELECT 1 FROM users WHERE id=?", userId).Scan(&temp)

	// no row found -> user does not exist
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}

	// some other error
	if err != nil {
		return false, err
	}

	// user exists and found
	return true, nil
}
