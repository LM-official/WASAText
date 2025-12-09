package database

import "github.com/MercuriLorenzo/WASAText/service/schemas"

func (db *appdbimpl) SetMyUserName(userId schemas.UserId, newUsername schemas.Username) (schemas.User, error) {
	_, err := db.c.Exec("UPDATE users SET username=? WHERE id=?", newUsername, userId)
	if err != nil {
		// error during update
		return schemas.User{}, err
	}

	// get the updated user
	var user schemas.User
	err = db.c.QueryRow("SELECT username, photo FROM users WHERE id=?", userId).Scan(&user.Username, &user.Photo)
	if err != nil {
		// error fetching updated user
		return schemas.User{}, err
	}

	return user, err
}
