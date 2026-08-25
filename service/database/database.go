/* In service/database/database.go dovrai popolare l'interfaccia AppDatabase con tutti i metodi
necessari per implementare gli operationId definiti nell'api.
Ogni metodo deve riflettere un'operazione di accesso ai dati.
*/

/*
Package database is the middleware between the app database and the code. All data (de)serialization (save/load) from a
persistent database are handled here. Database specific logic should never escape this package.

To use this package you need to apply migrations to the database if needed/wanted, connect to it (using the database
data source name from config), and then initialize an instance of AppDatabase from the DB connection.

For example, this code adds a parameter in `webapi` executable for the database data source name (add it to the
main.WebAPIConfiguration structure):

	DB struct {
		Filename string `conf:""`
	}

This is an example on how to migrate the DB and connect to it:

	// Start Database
	logger.Println("initializing database support")
	db, err := sql.Open("sqlite3", "./foo.db")
	if err != nil {
		logger.WithError(err).Error("error opening SQLite DB")
		return fmt.Errorf("opening SQLite: %w", err)
	}
	defer func() {
		logger.Debug("database stopping")
		_ = db.Close()
	}()

Then you can initialize the AppDatabase and pass it to the api package.
*/
package database

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/MercuriLorenzo/WASAText/service/schemas"
)

// dateFormat is how every date column of the schema below is written and read
// It holds no sub-second part, so every stored date has the same width and text order is chronological order,
// which is what lets a query sort a chat and find its last message without parsing anything
// Write it in UTC: an offset other than Z would sort by its own digits and not by the instant
const dateFormat = time.RFC3339

// AppDatabase is the high level interface for the DB
type AppDatabase interface {
	// My methods
	DoLogin(username schemas.Username) (schemas.UserId, bool, error)
	SetMyUserName(userId schemas.UserId, newUsername schemas.Username) (schemas.User, error)
	SetMyPhoto(userId schemas.UserId, newPhotoId schemas.PhotoId) (schemas.User, schemas.PhotoId, error)
	GetUsers(username schemas.Username) (schemas.Users, error)
	GetMyConversations(userId schemas.UserId) (schemas.Chats, error)
	CreatePrivateChat(userId1 schemas.UserId, userId2 schemas.UserId) (schemas.ChatId, bool, error)
	CreateGroup(creator schemas.UserId, userIds schemas.Members, name schemas.ChatName, photoId schemas.PhotoId) (schemas.ChatId, error)
	SetGroupName(userId schemas.UserId, groupId schemas.ChatId, newName schemas.ChatName) (schemas.ChatSummary, error)
	SetGroupPhoto(userId schemas.UserId, groupId schemas.ChatId, newPhotoId schemas.PhotoId) (schemas.ChatSummary, schemas.PhotoId, error)
	AddToGroup(userId schemas.UserId, groupId schemas.ChatId, userIds schemas.Members) (schemas.ChatWithMembers, error)
	LeaveGroup(userId schemas.UserId, groupId schemas.ChatId) (schemas.ChatWithMembers, error)

	// My helpers
	UsersExist(userIds schemas.Members) (bool, error)
	PhotoIsReferenced(photoId schemas.PhotoId) (bool, error)
	Ping() error
}

type appdbimpl struct {
	c *sql.DB
}

// New returns a new instance of AppDatabase based on the SQLite connection `db`.
// `db` is required - an error will be returned if `db` is `nil`.
func New(db *sql.DB) (AppDatabase, error) {
	if db == nil {
		return nil, errors.New("database is required when building a AppDatabase")
	}

	// The photo column holds the id of the photo, not its bytes and not its URL:
	// the bytes are a file of the photos package, and only the api layer builds the URL
	const sqlStmt = `
	CREATE TABLE IF NOT EXISTS users (
		id TEXT NOT NULL PRIMARY KEY,
		username TEXT NOT NULL UNIQUE,
		photoId TEXT NOT NULL
	);


	-- A private chat and a group are the same thing with a different chatType:
	-- a group owns its name and photo, a private chat borrows them from the other member
	-- A borrowed value is never stored: one row cannot hold both points of view, and a copy
	-- would go stale on every setMyUserName or setMyPhoto, so those columns stay NULL
	CREATE TABLE IF NOT EXISTS chats (
		id TEXT NOT NULL PRIMARY KEY,
		chatType TEXT NOT NULL,
		name TEXT,
		photoId TEXT,
		-- pairKey is the two member ids sorted and joined:
		-- UNIQUE needs one row while membership is two, so the pair is flattened, and only one chat can hold it
		-- NULL for a group: the same people may share many groups
		pairKey TEXT UNIQUE,
		CHECK (
			(chatType = 'private' AND name IS NULL AND photoId IS NULL AND pairKey IS NOT NULL)
			OR (chatType = 'group' AND name IS NOT NULL AND photoId IS NOT NULL AND pairKey IS NULL)
		)
	);

	-- chat_members is the relation itself, one row per membership:
	-- a user belongs to many chats and a chat holds many users
	CREATE TABLE IF NOT EXISTS chat_members (
		chatId TEXT NOT NULL,
		userId TEXT NOT NULL,
		-- The date this member last opened the chat, NULL while the member never opened it
		-- A message is read by a member when the member opened the chat after the message arrived,
		-- which is what makes the state of a message a value to compute and never a value to store
		lastReadDate TEXT,
		-- The composite key makes a duplicate membership unrepresentable
		PRIMARY KEY (chatId, userId),
		-- Dropping a chat drops its memberships
		-- A user still inside a chat cannot be dropped
		FOREIGN KEY (chatId) REFERENCES chats(id) ON DELETE CASCADE,
		FOREIGN KEY (userId) REFERENCES users(id)
	);

	-- Performance only:
	-- Reading the chats of a user is slow, so needs an index
	-- Reading the members of a chat is fast (the primary key is already sorted by chatId), so does not need an index
	CREATE INDEX IF NOT EXISTS idx_chat_members_userId ON chat_members(userId);

	
	-- The chatId column has no field in schemas.Message:
	-- the chat is already in the URL of every message endpoint, so it is context and not content
	-- A message carries text, photo, or both
	-- There is no state column: 'received by all members' and 'read by all members' are facts about the other members,
	-- so the state is computed from chat_members.lastReadDate when a chat is read
	-- date is a string in UTC: written that way it sorts chronologically as text,
	-- which is what lets the index below order a chat and find its last message
	CREATE TABLE IF NOT EXISTS messages (
		id TEXT NOT NULL PRIMARY KEY,
		chatId TEXT NOT NULL,
		userId TEXT NOT NULL,
		text TEXT,
		photoId TEXT,
		date TEXT NOT NULL,
		-- An empty message is not allowed
		CHECK (text IS NOT NULL OR photoId IS NOT NULL),
		-- Dropping a chat drops its messages
		FOREIGN KEY (chatId) REFERENCES chats(id) ON DELETE CASCADE,
		FOREIGN KEY (userId) REFERENCES users(id)
	);

	-- Performance only:
	-- Both the messages list of a chat and the snippet of a chat read one chat sorted by date,
	-- the snippet taking only the last row
	CREATE INDEX IF NOT EXISTS idx_messages_chatId_date ON messages(chatId, date);

	-- A comment is the reaction of a user to a message
	CREATE TABLE IF NOT EXISTS comments (
		id TEXT NOT NULL PRIMARY KEY,
		messageId TEXT NOT NULL,
		userId TEXT NOT NULL,
		emoji TEXT NOT NULL,
		-- Only one comment per user per message, so commentMessage replaces instead of piling up
		-- Is already sorted by messageId, so do not need an index
		UNIQUE (messageId, userId),
		-- Dropping a message drops its comments
		FOREIGN KEY (messageId) REFERENCES messages(id) ON DELETE CASCADE,
		FOREIGN KEY (userId) REFERENCES users(id)
	);`
	_, err := db.Exec(sqlStmt)
	if err != nil {
		return nil, fmt.Errorf("error creating database: %w", err)
	}

	return &appdbimpl{
		c: db,
	}, nil
}

func (db *appdbimpl) Ping() error {
	return db.c.Ping()
}
