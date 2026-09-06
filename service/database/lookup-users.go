package database

import (
	"fmt"
	"strings"

	"github.com/MercuriLorenzo/WASAText/service/schemas"
)

// LookupUsers returns current profiles for the given IDs, omitting nonexistent users.
func (db *appdbimpl) LookupUsers(userIds schemas.Members) (schemas.Users, error) {
	users := make(schemas.Users, 0)
	// Nobody to look for
	if len(userIds) == 0 {
		return users, nil
	}

	// One placeholder per id, the same text repeated
	placeholders := "?" + strings.Repeat(", ?", len(userIds)-1)
	args := make([]interface{}, 0, len(userIds))
	for _, id := range userIds {
		args = append(args, id)
	}

	rows, err := db.c.Query(`SELECT id, username, photoId FROM users WHERE id IN (`+placeholders+`);`, args...)
	if err != nil {
		return nil, fmt.Errorf("cannot look up users: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var user schemas.User
		// Error reading a row: a shorter list would be a wrong answer, not a partial one
		if err := rows.Scan(&user.Id, &user.Username, &user.Photo); err != nil {
			return nil, fmt.Errorf("cannot read a user: %w", err)
		}

		users = append(users, user)
	}
	// Error during rows iteration
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("cannot read users: %w", err)
	}

	return users, nil
}
