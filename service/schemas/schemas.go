package schemas

// Every type here is the Go form of a schema of doc/api.yaml
// Each section says which SQL table stores it:
// a field without a column is derived at query time and never written,
// a column without a field is context, not content

import (
	"strings"
	"time"
)

// ---------- ERROR ----------
// No table: an error is built by the api layer when a request fails, and never stored
// It is the body of every 4xx and 5xx response of doc/api.yaml
type Error struct {
	Code    int    `json:"code"`    // The HTTP status code of the response
	Message string `json:"message"` // Why the request failed
}

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
// MaxPhotoBytes is the size limit of any uploaded photo
const MaxPhotoBytes = 30 << 20 // 31457280 (30 * 1024 * 1024)

// PhotoFile is what the client uploads
// PhotoURL is what the API returns
// Only the api layer turns a PhotoId into a PhotoURL
type PhotoFile []byte
type PhotoURL string

// DefaultPhotoId is the photo id a new user starts with
const DefaultPhotoId PhotoId = "00000000-0000-4000-8000-000000000000"

// PhotoPathPrefix is what a PhotoURL holds before the id of the photo
const PhotoPathPrefix = "/photos/"

// The URL is relative: the client already knows the host it is talking to,
// while the server behind a proxy or on another port does not know its own
func (i PhotoId) URL() PhotoURL {
	return PhotoURL(PhotoPathPrefix + i)
}

// Id gives back the PhotoId the PhotoURL points at
func (p PhotoURL) Id() PhotoId {
	return PhotoId(strings.TrimPrefix(string(p), PhotoPathPrefix))
}

// ---------- USER ----------
// Table: users (id, username, photoId)
type Username string
type User struct {
	Id       UserId   `json:"id"`
	Username Username `json:"username"`
	Photo    PhotoURL `json:"photo"`
}
type Users []User

// ---------- CHAT ----------
// tables: chats (id, chatType, name, photoId, pairKey) + chat_members (chatId, userId, lastReadDate)
// A private chat and a group are the same thing with a different chatType,
// exactly like the chats table: a group owns its name and photo, a private chat borrows them from the other member
type ChatType string

const (
	ChatTypePrivate ChatType = "private"
	ChatTypeGroup   ChatType = "group"
)

type ChatName string // The group name, or the username of the other member in a private chat

// GroupMaxMembers is how many members a group can hold at once
// A private chat is always the two of its pair
const GroupMaxMembers = 100

// Members is a list of ids: what a request names, and what a lookup asks about
// createGroup and addToGroup are given ids, and UsersExist checks ids
type Members []UserId

// ChatMembers is a list of users: what a chat answers with
// Every read hands out ids, but the interface has to draw a username and a photo,
// so the members of a chat travel with both and the client never has to ask who they are.
// A sender who left the chat is not here, and its name is still read one id at a time:
// this list is the membership, not everybody the answer mentions
type ChatMembers []User

// UserChatsPageSize is how many chats one read of the chats of a user gives back:
// the most recently active ones, which is where the homepage opens, while the older ones are what the client asks for by scrolling
// There is no cap on how many chats a user belongs to, so this page is the only bound of that read
const UserChatsPageSize = 500

// ChatBase is what a chat carries whichever way it is asked for, and the root every shape below grows from
type ChatBase struct {
	Id    ChatId   `json:"id"`
	Type  ChatType `json:"chatType"`
	Name  ChatName `json:"name"`
	Photo PhotoURL `json:"photo"`
}

// ChatSummary is one element of the chats list of the homepage: the base + the preview of its last message
// The snippet is only here and not in any other Chat* because the list is the only place that draws a preview
type ChatSummary struct {
	ChatBase
	Snippet *Snippet `json:"snippet,omitempty"` // Absent while the chat has no messages
}

type ChatSummaries []ChatSummary

// ChatWithMembers is the base + who belongs to the chat
type ChatWithMembers struct {
	ChatBase
	Members ChatMembers `json:"members"`
}

// ChatDetail is an opened chat: the base + the members and the full messages list
type ChatDetail struct {
	ChatWithMembers
	Messages Messages `json:"messages"`
}

// ---------- MESSAGE ----------
// Table: messages (id, chatId, userId, text, photoId, date)
// The chatId column has no field here: the chat is already in the URL of every message endpoint
// State has no column: it is computed from chat_members.lastReadDate at every read,
// because it is a fact about who belongs to the chat now and that set changes under messages nobody touched
type MessageState string

const (
	MessageStateReceived MessageState = "received"
	MessageStateRead     MessageState = "read"
)

// ChatMaxMessages is how many messages a chat can hold at once
const ChatMaxMessages = 10000

// ChatMessagesPageSize is how many of them one read of a chat gives back:
// the newest ones, which is where a chat is opened, while the older ones are
// what the client asks for by scrolling up
const ChatMessagesPageSize = 500

// MessageBase is everything a message and its snippet have in common
type MessageBase struct {
	Id    MessageId    `json:"id"`
	User  UserId       `json:"user"`
	Date  time.Time    `json:"date"`
	State MessageState `json:"state"`
}

// MessageTextMaxChars is the maximum length of a message text
const MessageTextMaxChars = 5000

type MessageText string
type MessageContent struct {
	Text  MessageText `json:"text,omitempty"` // omitempty: field does not show in JSON if empty
	Photo PhotoURL    `json:"photo,omitempty"`
}

type Message struct {
	MessageBase
	Content MessageContent `json:"content"`
	// The message this one answers, absent when it answers none
	// It is always a message of the same chat, so the client finds it in the page it already holds
	// and never has to ask for it;
	// a quote whose message was deleted comes back absent, because retracting a message must not retract the answer somebody else wrote to it
	ReplyTo  MessageId `json:"replyTo,omitempty"`
	Comments Comments  `json:"comments"` // The reactions on this message, empty list if none
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

// SnippetMaxChars is how much of the text of a message reaches its snippet
const SnippetMaxChars = 50

// SnippetPhotoEmoji stands for the photo of a message in its snippet:
// the preview carries a symbol where the opened chat carries the actual picture
const SnippetPhotoEmoji Emoji = "📷"

type SnippetText string
type SnippetContent struct {
	Text  SnippetText `json:"text,omitempty"`
	Emoji Emoji       `json:"emoji,omitempty"` // Stands for the media of the message (e.g. 📷)
}
type Snippet struct {
	MessageBase                // The id is message this snippet previews
	Content     SnippetContent `json:"content"`
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
type MessageTextRequest struct {
	Text MessageText `json:"text"` // sendMessage, the text part of the multipart body
}
type EmojiRequest struct {
	Emoji Emoji `json:"emoji"` // commentMessage
}
