package database

import (
	"strings"

	"github.com/MercuriLorenzo/WASAText/service/schemas"
)

// `%` and `_` are LIKE wildcards, and `_` is a legal username character:
// without escaping, the prefix "a_b" would also match "axb"
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// GetUsers returns the users whose username starts with the given prefix
func (db *appdbimpl) GetUsers(username schemas.Username) (schemas.Users, error) {
	results := make(schemas.Users, 0)

	prefix := likeEscaper.Replace(string(username)) + "%"
	rows, err := db.c.Query(`SELECT id, username, photo FROM users WHERE username LIKE ? ESCAPE '\' LIMIT 20`, prefix)
	if err != nil {
		// Query error
		return nil, err
	}
	// Close rows when done even in case of error
	defer rows.Close()

	for rows.Next() {
		var u schemas.User
		if err := rows.Scan(&u.Id, &u.Username, &u.Photo); err != nil {
			// Error reading a row: a shorter list would be a wrong answer, not a partial one
			return nil, err
		}

		// Append to results
		results = append(results, u)
	}
	if err := rows.Err(); err != nil {
		// Error during rows iteration
		return nil, err
	}

	return results, nil
}
