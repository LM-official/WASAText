package database

import (
	"github.com/MercuriLorenzo/WASAText/service/schemas"
)

func (db *appdbimpl) SearchUsers(username schemas.Username) ([]schemas.User, error) {
	results := make([]schemas.User, 0)

	rows, err := db.c.Query("SELECT username, photo FROM users WHERE username LIKE ? LIMIT 20", username+"%")
	if err != nil {
		// query error
		return results, err
	}
	// close rows when done even in case of error
	defer rows.Close()

	for rows.Next() {
		var u schemas.User
		if err := rows.Scan(&u.Username, &u.Photo); err == nil {
			// append to results
			results = append(results, u)
		}
		// error reading row, row skipped
	}
	if err = rows.Err(); err != nil {
		// error during rows iteration
		return results, err
	}

	return results, nil
}
