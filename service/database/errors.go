package database

import "errors"

// The failures of this package that the api layer needs to tell apart from a generic error.
// A handler maps them to a status code with errors.Is, so no SQLite message is ever read outside:
// database specific logic never escapes this package
var (
	// ErrUsernameTaken is returned when the username is already used by another user
	ErrUsernameTaken = errors.New("username already taken")
)
