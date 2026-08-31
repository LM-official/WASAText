package database

import "errors"

// The failures of this package that the api layer needs to tell apart from a generic error.
// A handler maps them to a status code with errors.Is, so no SQLite message is ever read outside:
// database specific logic never escapes this package
var (
	// ErrUsernameTaken is returned when the username is already used by another user
	ErrUsernameTaken = errors.New("username already taken")

	// ErrUserNotFound is returned when no user owns the given id
	ErrUserNotFound = errors.New("user not found")

	// ErrChatNotFound is returned when no chat owns the given id, or it is not the kind of chat the caller asked for
	// A private chat reached through a /groups/ route is not found: under that collection its id names nothing
	ErrChatNotFound = errors.New("chat not found")

	// ErrNotAMember is returned when the chat exists but the caller does not belong to it
	ErrNotAMember = errors.New("not a member of the chat")

	// ErrGroupFull is returned when the members to add take the group past schemas.GroupMaxMembers
	// The members already inside cost the group nothing: only the ones it actually gains are counted
	ErrGroupFull = errors.New("group is full")

	// ErrChatFull is returned when the chat already holds schemas.ChatMaxMessages messages
	// What was asked cannot fit, which is about the request and not about who is asking
	ErrChatFull = errors.New("chat is full")

	// ErrMessageNotFound is returned when no message owns the given id
	ErrMessageNotFound = errors.New("message not found")

	// ErrCommentNotFound is returned when the caller has no comment on the message
	ErrCommentNotFound = errors.New("comment not found")

	// ErrNotSender is returned when the caller did not send the message
	// Reacting, uncommenting and reading all belong to any member of the chat,
	// but retracting a message one did not write does not: only its sender can delete it
	ErrNotSender = errors.New("caller is not the sender of the message")
)
