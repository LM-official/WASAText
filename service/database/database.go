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

	"github.com/MercuriLorenzo/WASAText/service/schemas"
)

// AppDatabase is the high level interface for the DB
type AppDatabase interface {
	// My methods
	DoLogin(username schemas.Username) (schemas.UserId, bool, error)
	SetMyUserName(userId schemas.UserId, newUsername schemas.Username) (schemas.User, error)
	SetMyPhoto(userId schemas.UserId, newPhotoId schemas.PhotoId) (schemas.User, schemas.PhotoId, error)
	GetUsers(username schemas.Username) (schemas.Users, error)

	// My helpers
	UserExists(userId schemas.UserId) (bool, error)
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
	);`
	_, err := db.Exec(sqlStmt)
	if err != nil {
		return nil, fmt.Errorf("error creating users table: %w", err)
	}
	// }

	return &appdbimpl{
		c: db,
	}, nil
}

func (db *appdbimpl) Ping() error {
	return db.c.Ping()
}
