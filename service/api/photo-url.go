package api

import "github.com/MercuriLorenzo/WASAText/service/schemas"

// The database stores the id of a photo, doc/api.yaml answers with its URL:
// these helpers are the only place that turns one into the other, and they must be called on
// every user that leaves this package, or a raw id is served in place of a URL

// withPhotoURL turns the stored photo id of a user into the URL the API returns
func withPhotoURL(u schemas.User) schemas.User {
	u.Photo = schemas.PhotoId(u.Photo).URL()
	return u
}

// withPhotoURLs is withPhotoURL over a list of users
func withPhotoURLs(us schemas.Users) schemas.Users {
	for i := range us {
		us[i] = withPhotoURL(us[i])
	}
	return us
}
