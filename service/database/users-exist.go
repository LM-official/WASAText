package database

import (
	"strings"

	"github.com/MercuriLorenzo/WASAText/service/schemas"
)

// UsersExist reports whether every given user exists, one user or many
// One query and not one per user: the list is already free of duplicates,
// so counting the rows that match tells that none of them is missing without naming which one is
//
// An empty list is true: there is no user in it that fails to exist
// A caller handing over a list that comes from a request must refuse the empty one first,
// or it reads this true as a membership it never checked
func (db *appdbimpl) UsersExist(userIds schemas.Members) (bool, error) {
	// Nobody to look for
	if len(userIds) == 0 {
		return true, nil
	}

	// One placeholder per id, the same text repeated
	placeholders := "?" + strings.Repeat(", ?", len(userIds)-1)
	args := make([]interface{}, 0, len(userIds))
	for _, userId := range userIds {
		args = append(args, userId)
	}

	var found int
	// Error during the search
	if err := db.c.QueryRow(`SELECT COUNT(*) FROM users WHERE id IN (`+placeholders+`);`, args...).Scan(&found); err != nil {
		return false, err
	}

	return found == len(userIds), nil
}
