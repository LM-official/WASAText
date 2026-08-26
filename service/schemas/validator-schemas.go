package schemas

// One IsValid() for each type of schemas.go: it returns an error describing the broken rule, or nil
// The api layer calls it on everything that arrives from a client, so a handler never checks a field by hand

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/gofrs/uuid"
	"github.com/rivo/uniseg"
)

// ---------- ERROR ----------
// Returns error if the Error does not meets the rules, otherwise nil
func (e *Error) IsValid() error {
	if e == nil {
		return errors.New("error is nil")
	}

	if e.Code < 100 || e.Code > 599 {
		return fmt.Errorf("invalid error code: %d; must be between 100 and 599", e.Code)
	}

	// An empty message is allowed: the status code alone already says what happened
	if n := CountChars(e.Message); n > 500 {
		return fmt.Errorf("invalid error message length: %d; must be at most 500 characters", n)
	}

	return nil
}

// ---------- BASE TYPES ----------
// ---------- ID ----------
// Returns error if the Id is not a well formed UUID, otherwise nil
func (i Id) IsValid() error {
	u, err := uuid.FromString(string(i))
	if err != nil {
		return err
	}

	// uuid.FromString also accepts the braced, the urn and the unhyphenated form of the same UUID:
	// only the canonical 36 chars form is a valid Id, so one id always has one spelling
	if u.String() != string(i) {
		return fmt.Errorf("invalid id format: '%s'; must be a canonical lowercase UUID", i)
	}

	return nil
}

// Every id inherits the Id rules: a specific rule of one entity goes in its own method
func (i PhotoId) IsValid() error   { return Id(i).IsValid() }
func (i UserId) IsValid() error    { return Id(i).IsValid() }
func (i ChatId) IsValid() error    { return Id(i).IsValid() }
func (i MessageId) IsValid() error { return Id(i).IsValid() }
func (i CommentId) IsValid() error { return Id(i).IsValid() }

// ---------- EMOJI ----------
// returns error if the Emoji does not meets the rules, otherwise nil
func (e Emoji) IsValid() error {
	if uniseg.GraphemeClusterCount(string(e)) != 1 { // Look for a single symbol ("a" = 1, "👍" = 1)
		return errors.New("emoji field must contain exactly one symbol")
	}

	// One symbol has no length limit: an "a" plus 20 combining accents is still one symbol, so cap
	if n := utf8.RuneCountInString(string(e)); n > 16 {
		return fmt.Errorf("invalid emoji length: %d code points; must be at most 16", n)
	}

	// Better checks for emojis are complex
	return nil
}

// ---------- TRIM AND COUNT STRING ----------
// Trims a string and counts the chars (any emoji counts one)
func CountChars(s string) int {
	// 1) trim
	// 2) count the chars (any emoji counts 1)
	return uniseg.GraphemeClusterCount(strings.TrimSpace(s))
}

// TruncateChars trims a string and keeps its first n chars, counted as CountChars counts them
// Cutting on bytes or on runes would split an emoji in half and leave a broken symbol behind,
// so the cut falls on the boundary between two chars and never inside one
func TruncateChars(s string, n int) string {
	// No chars
	if n <= 0 {
		return ""
	}

	s = strings.TrimSpace(s)

	state := -1
	rest := s
	// Iterate one char at a time
	for i := 0; i < n; i++ {
		if len(rest) == 0 {
			return s
		}

		_, rest, _, state = uniseg.FirstGraphemeClusterInString(rest, state)
	}

	// rest is what comes after the nth char
	return s[:len(s)-len(rest)]
}

// ---------- PHOTO ----------
// Returns error if the uploaded file does not meets the rules, otherwise nil
func (p PhotoFile) IsValid() error {
	n := len(p)
	if n < 1 || n > MaxPhotoBytes {
		return fmt.Errorf("invalid photo size: %d bytes; must be between 1 byte and %d", n, MaxPhotoBytes)
	}
	return nil
}

// Returns error if the PhotoURL does not meets the rules, otherwise nil
func (p PhotoURL) IsValid() error {
	n := len(p)
	if n < 4 || n > 100 {
		return fmt.Errorf("invalid photo URL length: %d; must be between 4 and 100 characters", n)
	}
	return nil
}

// ---------- USER ----------
// Precompiled regex at package level to avoid recompiling it on each call
var usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`)

// Returns error if the Username does not meets the rules, otherwise nil
func (u Username) IsValid() error {
	// len() does not works with emojis, count the chars instead
	n := CountChars(string(u))
	if n < 1 || n > 30 {
		return fmt.Errorf("invalid username length: %d; must be between 1 and 30 characters", n)
	}

	if !usernameRegex.MatchString(string(u)) {
		return errors.New("username contains invalid characters")
	}

	return nil
}

// Returns error if the User does not meets the rules, otherwise nil
func (u *User) IsValid() error {
	if u == nil {
		return errors.New("user is nil")
	}

	if err := u.Id.IsValid(); err != nil {
		return err
	}

	if err := u.Username.IsValid(); err != nil {
		return err
	}

	return u.Photo.IsValid()
}

// ---------- CHAT ----------
// Returns error if the ChatType does not meets the rules, otherwise nil
func (t ChatType) IsValid() error {
	switch t {
	case ChatTypePrivate, ChatTypeGroup:
		return nil

	default:
		return fmt.Errorf("invalid chat type: '%s'", t)
	}
}

// Returns error if the ChatName does not meets the rules, otherwise nil
func (n ChatName) IsValid() error {
	l := CountChars(string(n))
	if l < 1 || l > 100 {
		return fmt.Errorf("invalid chat name length: %d; must be between 1 and 100 characters", l)
	}
	return nil
}

// Returns error if the ChatBase does not meets the rules, otherwise nil
func (c *ChatBase) IsValid() error {
	if c == nil {
		return errors.New("chat base is nil")
	}

	if err := c.Id.IsValid(); err != nil {
		return err
	}

	if err := c.Type.IsValid(); err != nil {
		return err
	}

	if err := c.Photo.IsValid(); err != nil {
		return err
	}

	// The name depends on the type of the chat
	switch c.Type {
	case ChatTypePrivate:
		// The name of a private chat is the username of the other member
		if err := Username(c.Name).IsValid(); err != nil {
			return err
		}

	case ChatTypeGroup:
		if err := c.Name.IsValid(); err != nil {
			return err
		}
	}

	return nil
}

// Returns error if the ChatSummary does not meets the rules, otherwise nil
func (c *ChatSummary) IsValid() error {
	if c == nil {
		return errors.New("chat summary is nil")
	}

	if err := c.ChatBase.IsValid(); err != nil {
		return err
	}

	// The snippet is optional: a chat without messages has no preview
	if c.Snippet != nil {
		return c.Snippet.IsValid()
	}

	return nil
}

// Returns error if the Members list does not meets the rules, otherwise nil
func (m Members) IsValid() error {
	n := len(m)
	if n > GroupMaxMembers {
		return fmt.Errorf("invalid members number: %d; must be at most %d", n, GroupMaxMembers)
	}

	// Ensure that each member appears only once in the list
	seen := make(map[UserId]bool, n)
	for _, member := range m {
		if err := member.IsValid(); err != nil { // Check each member validity
			return err
		}

		if seen[member] { // Check for duplicates
			return fmt.Errorf("invalid duplicate member: '%s'", member)
		}
		seen[member] = true
	}

	return nil
}

// Returns error if the ChatWithMembers does not meets the rules, otherwise nil
func (c *ChatWithMembers) IsValid() error {
	if c == nil {
		return errors.New("chat with members is nil")
	}

	if err := c.ChatBase.IsValid(); err != nil {
		return err
	}

	// The number of members depends on the type of the chat
	// A chat is answered with its members only to a member of it, so it always holds at least the caller:
	// a group that empties is dropped in the same transaction that empties it
	switch c.Type {
	case ChatTypePrivate:
		// A private chat is always the two of its pair, and is left by neither
		if n := len(c.Members); n != 2 {
			return fmt.Errorf("invalid private chat members: %d; must be exactly 2", n)
		}

	case ChatTypeGroup:
		// A group chat has always at least a member
		if n := len(c.Members); n < 1 {
			return fmt.Errorf("invalid group members: %d; must be at least 1", n)
		}
	}

	return c.Members.IsValid()
}

// Returns error if the ChatDetail does not meets the rules, otherwise nil
func (c *ChatDetail) IsValid() error {
	if c == nil {
		return errors.New("chat detail is nil")
	}

	if err := c.ChatWithMembers.IsValid(); err != nil {
		return err
	}

	return c.Messages.IsValid()
}

// ---------- MESSAGE ----------
// Returns error if the MessageState does not meets the rules, otherwise nil
func (s MessageState) IsValid() error {
	switch s {
	case MessageStateReceived, MessageStateRead:
		return nil

	default:
		return fmt.Errorf("invalid message state: '%s'", s)
	}
}

// Returns error if the MessageBase does not meets the rules, otherwise nil
func (m *MessageBase) IsValid() error {
	if m == nil {
		return errors.New("message base is nil")
	}

	if err := m.Id.IsValid(); err != nil {
		return err
	}

	if err := m.User.IsValid(); err != nil {
		return err
	}

	if m.Date.IsZero() { // Set date
		return errors.New("date is missing")
	}

	return m.State.IsValid()
}

// Returns error if the MessageText does not meets the rules, otherwise nil
func (t MessageText) IsValid() error {
	n := CountChars(string(t))
	if n < 1 || n > 10000 {
		return fmt.Errorf("invalid message text length: %d; must be between 1 and 10000 characters", n)
	}
	return nil
}

// Returns error if the MessageContent does not meets the rules, otherwise nil
func (m *MessageContent) IsValid() error {
	if m == nil {
		return errors.New("message content is nil")
	}

	t := CountChars(string(m.Text)) > 0 // True if text length > 0
	p := len(m.Photo) > 0
	// There must be at least one (text, photo, photo+text), and each one present must be valid
	if !t && !p {
		return errors.New("empty message content: text or photo is required")
	}
	if t {
		if err := m.Text.IsValid(); err != nil {
			return err
		}
	}
	if p {
		if err := m.Photo.IsValid(); err != nil {
			return err
		}
	}

	return nil
}

// Returns error if the Message does not meets the rules, otherwise nil
func (m *Message) IsValid() error {
	if m == nil {
		return errors.New("message is nil")
	}

	if err := m.MessageBase.IsValid(); err != nil {
		return err
	}

	if err := m.Content.IsValid(); err != nil {
		return err
	}

	return m.Comments.IsValid()
}

// Returns error if the Messages list does not meets the rules, otherwise nil
func (m Messages) IsValid() error {
	n := len(m)
	if n > 100000 {
		return fmt.Errorf("invalid messages number: %d; must be less than 100000", n)
	}

	for i := range m {
		if err := m[i].IsValid(); err != nil { // Check each message validity
			return err
		}
	}

	return nil
}

// ---------- COMMENT ----------
// Returns error if the Comment does not meets the rules, otherwise nil
func (c *Comment) IsValid() error {
	if c == nil {
		return errors.New("comment is nil")
	}

	if err := c.Id.IsValid(); err != nil {
		return err
	}

	if err := c.User.IsValid(); err != nil {
		return err
	}

	return c.Emoji.IsValid()
}

// Returns error if the Comments list does not meets the rules, otherwise nil
func (c Comments) IsValid() error {
	n := len(c)
	if n > 1000 {
		return fmt.Errorf("invalid comments number: %d; must be less than 1000", n)
	}

	// a user can comment the same message only once, and every comment is a different row
	seen := make(map[UserId]bool, n)
	seenId := make(map[CommentId]bool, n)
	for i := range c {
		if err := c[i].IsValid(); err != nil { // Check each comment validity
			return err
		}

		if seen[c[i].User] { // Check for duplicates users
			return fmt.Errorf("invalid duplicate comment of the user: '%s'", c[i].User)
		}
		seen[c[i].User] = true

		if seenId[c[i].Id] { // Check for duplicated ids
			return fmt.Errorf("invalid duplicate comment id: '%s'", c[i].Id)
		}
		seenId[c[i].Id] = true
	}

	return nil
}

// ---------- SNIPPET ----------
// Returns error if the SnippetText does not meets the rules, otherwise nil
func (t SnippetText) IsValid() error {
	n := CountChars(string(t))
	if n < 1 || n > SnippetMaxChars {
		return fmt.Errorf("invalid snippet text length: %d; must be between 1 and %d characters", n, SnippetMaxChars)
	}
	return nil
}

// Returns error if the SnippetContent does not meets the rules, otherwise nil
func (s *SnippetContent) IsValid() error {
	if s == nil {
		return errors.New("snippet content is nil")
	}

	t := CountChars(string(s.Text)) > 0 // True if text length > 0
	e := len(s.Emoji) > 0               // True if a possible emoji exists (string not empty)
	// There must be at least one (text, emoji, emoji+text), and each one present must be valid
	if !t && !e {
		return errors.New("empty snippet content: text or emoji is required")
	}
	if t {
		if err := s.Text.IsValid(); err != nil {
			return err
		}
	}
	if e {
		if err := s.Emoji.IsValid(); err != nil {
			return err
		}
	}

	return nil
}

// Returns error if the Snippet does not meets the rules, otherwise nil
func (s *Snippet) IsValid() error {
	if s == nil {
		return errors.New("snippet is nil")
	}

	if err := s.MessageBase.IsValid(); err != nil {
		return err
	}

	// The id of the previewed message

	return s.Content.IsValid()
}

// ---------- REQUESTS ----------
// Returns error if the UsernameRequest does not meets the rules, otherwise nil
func (u *UsernameRequest) IsValid() error {
	if u == nil {
		return errors.New("username request is nil")
	}
	return u.Username.IsValid()
}

// Returns error if the UserIdRequest does not meets the rules, otherwise nil
func (u *UserIdRequest) IsValid() error {
	if u == nil {
		return errors.New("user id request is nil")
	}
	return u.Id.IsValid()
}

// Returns error if the GroupNameRequest does not meets the rules, otherwise nil
func (g *GroupNameRequest) IsValid() error {
	if g == nil {
		return errors.New("group name request is nil")
	}
	return g.Name.IsValid()
}

// Returns error if the GroupRequest does not meets the rules, otherwise nil
func (g *GroupRequest) IsValid() error {
	if g == nil {
		return errors.New("group request is nil")
	}

	if err := g.Name.IsValid(); err != nil {
		return err
	}

	// The request carries the other members only, the creator is added by the server:
	// at least one other member, and one place less than the whole group, which the creator takes
	if n := len(g.Members); n < 1 || n > GroupMaxMembers-1 {
		return fmt.Errorf("invalid group members: %d; must be between 1 and %d", n, GroupMaxMembers-1)
	}

	return g.Members.IsValid()
}

// Returns error if the MembersRequest does not meets the rules, otherwise nil
func (m *MembersRequest) IsValid() error {
	if m == nil {
		return errors.New("members request is nil")
	}

	return m.Members.IsValid()
}

// Returns error if the MessageIdRequest does not meets the rules, otherwise nil
func (m *MessageIdRequest) IsValid() error {
	if m == nil {
		return errors.New("message id request is nil")
	}
	return m.MessageId.IsValid()
}

// Returns error if the EmojiRequest does not meets the rules, otherwise nil
func (e *EmojiRequest) IsValid() error {
	if e == nil {
		return errors.New("emoji request is nil")
	}
	return e.Emoji.IsValid()
}
