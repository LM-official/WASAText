package schemas

import "time"

// ---------- ERROR ----------
// omitted, used base net/http Error struct

// ---------- PHOTO ----------
type Photo []byte
type PhotoURL string

// ---------- ID ----------
type Id string

// ---------- USER ----------
type UserId string
type Username string
type UsernameRequest struct {
	Username Username `json:"username"`
}
type User struct {
	Id       UserId   `json:"id"`
	Username Username `json:"username"`
	Photo    PhotoURL `json:"photo"`
}

// ---------- MESSAGE BASE ----------
type MessageState string

const (
	MessageStateReceived MessageState = "received"
	MessageStateRead     MessageState = "read"
)

type MessageBase struct {
	Date  time.Time    `json:"date"`
	User  UserId       `json:"user"`
	State MessageState `json:"state"`
}

// ---------- MESSAGE ----------
type MessageId string
type MessageText string
type MessageContent struct {
	Text  MessageText `json:"text,omitempty"` // omitempty: field does not show in JSON if empty
	Photo PhotoURL    `json:"photo,omitempty"`
}
type Message struct {
	MessageBase
	Id      MessageId      `json:"id"`
	Content MessageContent `json:"content"`
}

// ---------- EMOJI ----------
type Emoji string

// ---------- SNIPPET ----------
type SnippetId string
type SnippetText string
type SnippetContent struct {
	Text  SnippetText `json:"text,omitempty"`
	Emoji Emoji       `json:"emoji,omitempty"`
}
type Snippet struct {
	MessageBase
	Id      SnippetId      `json:"id"`
	Content SnippetContent `json:"content"`
}

// ---------- COMMENT ----------
type CommentId string
type Comment struct {
	Id    CommentId `json:"id"`
	Emoji Emoji     `json:"emoji"`
	User  UserId    `json:"user"`
}

// ---------- CHAT BASE ----------
type ChatId string
type Members []UserId

const (
	ChatTypeGroup   string = "group"
	ChatTypePrivate string = "private"
)

type ChatSummary struct {
	Id      ChatId  `json:"id"`
	Members Members `json:"members"`
	Snippet `json:"snippet"`
}
type Messages []Message

// ---------- CHAT GROUP ----------
type GroupName string
type GroupSummary struct {
	ChatSummary
	Type  string    `json:"chatType"`
	Name  GroupName `json:"name"`
	Photo PhotoURL  `json:"photo"`
}
type GroupDetail struct {
	GroupSummary
	Messages Messages `json:"messages"`
}

// ---------- CHAT PRIVATE ----------
type PrivateChatSummary struct {
	ChatSummary
	Type  string   `json:"chatType"`
	Name  Username `json:"name"`
	Photo PhotoURL `json:"photo"`
}
type PrivateChatDetail struct {
	PrivateChatSummary
	Messages Messages `json:"messages"`
}
