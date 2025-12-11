package schemas

// Every type here is the Go form of a schema of doc/api.yaml
// Each section says which SQL table stores it:
// a field without a column is derived at query time and never written,
// a column without a field is context, not content

import "time"

// ---------- ERROR ----------
// Omitted, used base net/http Error struct

// ---------- BASE TYPES ----------
// ---------- EMOJI ----------
type Emoji string

// ---------- ID ----------
// Every id is a UUID: one named type for each entity, so two ids can never be swapped by mistake
type Id string
type PhotoId string
type UserId string
type ChatId string
type MessageId string
type CommentId string

// ---------- PHOTO ----------
// The same photo has three forms:
// PhotoFile is what the client uploads
// PhotoURL is what the API returns
// Only the api layer turns a PhotoId into a PhotoURL
type PhotoFile []byte
type PhotoURL string

// ---------- USER ----------
// Table: users (id, username, photo)
type Username string
type User struct {
	Id       UserId   `json:"id,omitempty"` // only doLogin returns the Id
	Username Username `json:"username"`
	Photo    PhotoURL `json:"photo"`
}
type Users []User

// ---------- CHAT ----------
// tables: chats (id, chatType, name, photo) + chat_members (chatId, userId)
// A private chat and a group are the same thing with a different chatType,
// exactly like the chats table: a group owns its name and photo, a private chat borrows them from the other member
type ChatType string

const (
	ChatTypePrivate ChatType = "private"
	ChatTypeGroup   ChatType = "group"
)

type ChatName string // The group name, or the username of the other member in a private chat
type Members []UserId

// ChatSummary is one element of the chats list: the preview of a chat
// The members are not here: the homepage list only draws name, photo and snippet
// and a private chat already borrows name and photo from the other member
type ChatSummary struct {
	Id      ChatId   `json:"id"`
	Type    ChatType `json:"chatType"`
	Name    ChatName `json:"name"`
	Photo   PhotoURL `json:"photo"`
	Snippet *Snippet `json:"snippet,omitempty"` // Absent while the chat has no messages
}

// ChatDetail is an opened chat: the same summary + the members and the full messages list
type ChatDetail struct {
	ChatSummary
	Members  Members  `json:"members"`
	Messages Messages `json:"messages"`
}

// ---------- MESSAGE ----------
// Table: messages (id, chatId, userId, text, photo, date, state)
// The chatId column has no field here: the chat is already in the URL of every message endpoint
type MessageState string

const (
	MessageStateReceived MessageState = "received"
	MessageStateRead     MessageState = "read"
)

// MessageBase is everything a message and its snippet have in common
type MessageBase struct {
	Id    MessageId    `json:"id"`
	User  UserId       `json:"user"`
	Date  time.Time    `json:"date"`
	State MessageState `json:"state"`
}

type MessageText string
type MessageContent struct {
	Text  MessageText `json:"text,omitempty"` // omitempty: field does not show in JSON if empty
	Photo PhotoURL    `json:"photo,omitempty"`
}

type Message struct {
	MessageBase
	Content  MessageContent `json:"content"`
	Comments Comments       `json:"comments"` // The reactions on this message, empty list if none
}
type Messages []Message

// ---------- COMMENT ----------
// Table: comments (id, messageId, userId, emoji)
// A comment is the reaction of one user to one message: only one comment per user per message
type Comment struct {
	Id    CommentId `json:"id"`
	User  UserId    `json:"user"`
	Emoji Emoji     `json:"emoji"`
}
type Comments []Comment

// ---------- SNIPPET ----------
// No table: a snippet is always derived from the last message of a chat
// It is the preview shown in the chats list of the homepage
type SnippetText string
type SnippetContent struct {
	Text  SnippetText `json:"text,omitempty"`
	Emoji Emoji       `json:"emoji,omitempty"` // Stands for the media of the message (e.g. 📷)
}
type Snippet struct {
	MessageBase // The id is message this snippet previews
	Content SnippetContent `json:"content"`
}

// ---------- REQUESTS ----------
// One type for each request body, so that a handler never validates raw fields
type UsernameRequest struct {
	Username Username `json:"username"` // doLogin, setMyUserName
}
type UserIdRequest struct {
	Id UserId `json:"id"` // createPrivateChat
}
type GroupNameRequest struct {
	Name ChatName `json:"name"` // setGroupName
}
type GroupRequest struct {
	Name    ChatName `json:"name"` // createGroup, the 'data' part of the multipart body
	Members Members  `json:"members"`
}
type MembersRequest struct {
	Members Members `json:"members"` // addToGroup
}
type MessageIdRequest struct {
	MessageId MessageId `json:"messageId"` // forwardMessage
}
type EmojiRequest struct {
	Emoji Emoji `json:"emoji"` // commentMessage
}
