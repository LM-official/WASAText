package database

import "github.com/MercuriLorenzo/WASAText/service/schemas"

// PhotoIsReferenced reports whether any row still points at photoId.
// It is the rule that decides when a photo file may be deleted: a photo is dropped only when this returns false,
// so the same file can be shared by many rows (a forwarded message keeps the photo id of the original one)
// and none of them can delete it while another still needs it
//
// Every table with a photo column must appear in this query and be check here and nowhere else
// A table missing from this list means deleting photos that are still on screen
func (db *appdbimpl) PhotoIsReferenced(photoId schemas.PhotoId) (bool, error) {
	// Search for a profile picture, a group picture or the photo of a message
	var referenced bool
	err := db.c.QueryRow(`SELECT EXISTS (SELECT 1 FROM users WHERE photoId = ?)
						  OR EXISTS (SELECT 1 FROM chats WHERE photoId = ?)
						  OR EXISTS (SELECT 1 FROM messages WHERE photoId = ?);`,
		photoId, photoId, photoId).Scan(&referenced)
	if err != nil {
		// Error during the search
		return false, err
	}
	return referenced, nil
}
