package api

import "github.com/MercuriLorenzo/WASAText/service/schemas"

// The database stores the id of a photo, doc/api.yaml answers with its URL:
// these helpers are the only place that turns one into the other, and they must be called on
// every user that leaves this package, or a raw id is served in place of a URL

// withPhotoURL turns the stored photo id of a user into the URL the API returns
func withPhotoURL(u schemas.User) schemas.User {
	u.Photo = u.Photo.Id().URL()
	return u
}

// withPhotoURLs is withPhotoURL over a list of users
func withPhotoURLs(us schemas.Users) schemas.Users {
	for i := range us {
		us[i] = withPhotoURL(us[i])
	}
	return us
}

// withChatPhotoURL turns the stored photo id of a chat into the URL the API returns
// It takes the base every chat shape is built on, so one call covers the summary,
// the chat with its members and the opened chat alike
func withChatPhotoURL(c schemas.ChatBase) schemas.ChatBase {
	c.Photo = c.Photo.Id().URL()
	return c
}

// withChatPhotoURLs is withChatPhotoURL over a list of chats
func withChatPhotoURLs(cs schemas.ChatSummaries) schemas.ChatSummaries {
	for i := range cs {
		cs[i].ChatBase = withChatPhotoURL(cs[i].ChatBase)
	}
	return cs
}

// withMessagePhotoURL turns the stored photo id of a message into the URL the API returns
// A user and a chat always have a photo, a message carries text, photo, or both:
// the one without a photo is left alone, or the prefix would go out by itself as the URL of a photo that does not exist
func withMessagePhotoURL(m schemas.Message) schemas.Message {
	if m.Content.Photo == "" {
		return m
	}
	m.Content.Photo = m.Content.Photo.Id().URL()
	return m
}

// withMessagePhotoURLs is withMessagePhotoURL over a list of messages
func withMessagePhotoURLs(ms schemas.Messages) schemas.Messages {
	for i := range ms {
		ms[i] = withMessagePhotoURL(ms[i])
	}
	return ms
}
