package database

import (
	"fmt"
	"strings"

	"github.com/MercuriLorenzo/WASAText/service/schemas"
)

// `%` and `_` are LIKE wildcards, and `_` is a legal username character:
// without escaping, the prefix "a_b" would also match "axb"
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// GetUsers returns the users whose username starts with the given prefix
func (db *appdbimpl) GetUsers(username schemas.Username) (schemas.Users, error) {
	prefix := likeEscaper.Replace(string(username)) + "%"
	rows, err := db.c.Query(`SELECT id, username, photoId FROM users WHERE username LIKE ? ESCAPE '\' LIMIT 20;`, prefix)
	// Query error
	if err != nil {
		return schemas.Users{}, fmt.Errorf("cannot get the users whose username starts with %q: %w", username, err)
	}
	// Close rows when done even in case of error
	defer func() { _ = rows.Close() }()

	results := make(schemas.Users, 0)
	for rows.Next() {
		var u schemas.User
		// Error reading a row: a shorter list would be a wrong answer, not a partial one
		if err := rows.Scan(&u.Id, &u.Username, &u.Photo); err != nil {
			return schemas.Users{}, fmt.Errorf("cannot read a user whose username starts with %q: %w", username, err)
		}

		// Append to results
		results = append(results, u)
	}
	// Error during rows iteration
	if err := rows.Err(); err != nil {
		return schemas.Users{}, fmt.Errorf("cannot read the users whose username starts with %q: %w", username, err)
	}

	return results, nil
}
