package service

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/gofrs/uuid"
	"github.com/rivo/uniseg"
)

// ---------- TRIM AND COUNT STRING ----------
// trims a string and counts the chars (any emoji counts one)
func CountChars(s string) int {
	// 1) trim
	// 2) count the chars (any emoji counts 1)
	return uniseg.GraphemeClusterCount(strings.TrimSpace(s))
}

// ---------- ERROR ----------
// omitted, used base net/http Error struct

// ---------- PHOTO ----------
// returns error if the Photo does not meets the rules, otherwise nil
func (p Photo) IsValid() error {
	n := len(p)

	if n < 1 || n > 31457280 { // 30 Megabyte (30 * 1024 * 1024)
		return fmt.Errorf("invalid photo size: %d bytes; must be between 1 byte and 30MB", n)
	}

	return nil
}

// returns error if the PhotoURL does not meets the rules, otherwise nil
func (p PhotoURL) IsValid() error {
	n := len(p)

	if n < 4 || n > 300 {
		return fmt.Errorf("invalid photo URL length: %d; must be between 4 and 300 characters", n)
	}

	return nil
}

// ---------- ID ----------
// returns error if the Id does not meets the rules, otherwise nil
func (i Id) IsValid() error {
	// returns an error if the UUID string format is invalid
	_, err := uuid.FromString(string(i))

	return err
}

// ---------- USER ----------
// precompiled regex at package level to avoid recompiling it on each call
var usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`)

// returns error if the Username does not meets the rules, otherwise nil
func (u Username) IsValid() error {
	// len() does not works with emojis, use rune instead
	n := len(u)
	if n < 1 || n > 30 {
		return fmt.Errorf("invalid username length: %d; must be between 1 and 30 characters", n)
	}

	if !usernameRegex.MatchString(string(u)) {
		return errors.New("username contains invalid characters")
	}

	return nil
}

// returns error if the User does not meets the rules, otherwise nil
func (u *User) IsValid() error {
	if u == nil {
		return errors.New("user is nil")
	}

	if err := u.Username.IsValid(); err != nil {
		return err
	}
	if err := u.Photo.IsValid(); err != nil {
		return err
	}

	return nil
}

// ---------- MESSAGE BASE ----------
// returns error if the MessageState does not meets the rules, otherwise nil
func (s MessageState) IsValid() error {
	switch s {
	case MessageStateReceived, MessageStateRead:
		return nil
	default:
		return fmt.Errorf("invalid message state: '%s'", s)
	}
}

// returns error if the MessageBase does not meets the rules, otherwise nil
func (m *MessageBase) IsValid() error {
	if m == nil {
		return errors.New("message base is nil")
	}

	if m.Date.IsZero() { // set date
		return errors.New("date is missing")
	}
	if err := m.User.IsValid(); err != nil {
		return err
	}
	if err := m.State.IsValid(); err != nil {
		return err
	}

	return nil
}

// ---------- MESSAGE ----------
// returns error if the MessageId does not meets the rules, otherwise nil
func (i MessageId) IsValid() error {
	// inherits Id validation rules
	if err := Id(i).IsValid(); err != nil {
		return err
	}

	// specific rules for MessageId
	return nil
}

// returns error if the MessageText does not meets the rules, otherwise nil
func (t MessageText) IsValid() error {
	n := CountChars(string(t))

	if n < 1 || n > 10000 {
		return fmt.Errorf("invalid message text length: %d; must be between 1 and 10000 characters", n)
	}

	return nil
}

// returns error if the MessageContent does not meets the rules, otherwise nil
func (m *MessageContent) IsValid() error {
	if m == nil {
		return errors.New("message content is nil")
	}

	t := CountChars(string(m.Text)) > 0 // true if text length > 0
	p := len(m.Photo) > 0

	// there must be at least one (text, photo, photo+text)
	// if there is text, it must be valid
	// if there is a photo, it must be valid
	isValid := (t || p) &&
		(!t || m.Text.IsValid() == nil) &&
		(!p || m.Photo.IsValid() == nil)
	if !isValid {
		return errors.New("invalid message content")
	}

	return nil
}

// returns error if the Message does not meets the rules, otherwise nil
func (m *Message) IsValid() error {
	if m == nil {
		return errors.New("message is nil")
	}

	if err := m.MessageBase.IsValid(); err != nil {
		return err
	}
	if err := m.Id.IsValid(); err != nil {
		return err
	}
	if err := m.Content.IsValid(); err != nil {
		return err
	}

	return nil
}

// ---------- EMOJI ----------
// returns error if the Emoji does not meets the rules, otherwise nil
func (e Emoji) IsValid() error {
	if uniseg.GraphemeClusterCount(string(e)) != 1 { // look for a single symbol ("a" = 1, "👍" = 1)
		return errors.New("emoji field must contain exactly one symbol")
	}

	// better checks for emojis are complex

	return nil
}

// ---------- SNIPPET ----------
// returns error if the SnippetId does not meets the rules, otherwise nil
func (i SnippetId) IsValid() error {
	// inherits Id validation rules
	if err := Id(i).IsValid(); err != nil {
		return err
	}

	// specific rules for SnippetId
	return nil
}

// returns error if the SnippetText does not meets the rules, otherwise nil
func (t SnippetText) IsValid() error {
	n := CountChars(string(t))

	if n < 1 || n > 50 {
		return fmt.Errorf("invalid snippet text length: %d; must be between 1 and 50 characters", n)
	}

	return nil
}

// returns error if the SnippetContent does not meets the rules, otherwise nil
func (s *SnippetContent) IsValid() error {
	if s == nil {
		return errors.New("snippet content is nil")
	}

	t := CountChars(string(s.Text)) > 0 // true if len(text) > 0
	e := len(s.Emoji) > 0               // true if a possible emoji exists (string not empty)

	// there must be at least one (text, emoji, emoji+text)
	// if there is text, it must be valid
	// if there is a emoji, it must be valid
	isValid := (t || e) &&
		(!t || s.Text.IsValid() == nil) &&
		(!e || s.Emoji.IsValid() == nil)
	if !isValid {
		return errors.New("invalid snippet content")
	}

	return nil
}

// returns error if the Snippet does not meets the rules, otherwise nil
func (s *Snippet) IsValid() error {
	if s == nil {
		return errors.New("snippet is nil")
	}
	if err := s.MessageBase.IsValid(); err != nil {
		return err
	}
	if err := s.Id.IsValid(); err != nil {
		return err
	}
	if err := s.Content.IsValid(); err != nil {
		return err
	}

	return nil
}

// ---------- COMMENT ----------
// returns error if the CommentId does not meets the rules, otherwise nil
func (i CommentId) IsValid() error {
	// inherits Id validation rules
	if err := Id(i).IsValid(); err != nil {
		return err
	}

	// specificl rules for CommentId
	return nil
}

// returns error if the Comment does not meets the rules, otherwise nil
func (c *Comment) IsValid() error {
	if c == nil {
		return errors.New("comment is nil")
	}

	if err := c.Id.IsValid(); err != nil {
		return err
	}
	if err := c.Emoji.IsValid(); err != nil {
		return err
	}
	if err := c.User.IsValid(); err != nil {
		return err
	}

	return nil
}

// ---------- CHAT BASE ----------
// returns error if the ChatId does not meets the rules, otherwise nil
func (i ChatId) IsValid() error {
	// inherits Id validation rules
	if err := Id(i).IsValid(); err != nil {
		return err
	}

	// specific rules for ChatId
	return nil
}

// returns error if the Members does not meets the rules, otherwise nil
func (m Members) IsValid() error {
	n := len(m)
	if n > 100 {
		return fmt.Errorf("invalid member numbers: %d; must less than 100", n)
	}

	// unique members check
	seen := make(map[Username]bool, n)

	for _, member := range m {
		if err := member.IsValid(); err != nil { // check each member validity
			return err
		}

		if seen[member] { // check for duplicates
			return fmt.Errorf("invalid duplicate members: '%s'", member)
		}
		seen[member] = true
	}

	return nil
}

// returns error if the ChatId does not meets the rules, otherwise nil
func (c *ChatSummary) IsValid() error {
	if c == nil {
		return errors.New("chat summary is nil")
	}

	if err := c.Id.IsValid(); err != nil {
		return err
	}
	if err := c.Members.IsValid(); err != nil {
		return err
	}
	if err := c.Snippet.IsValid(); err != nil {
		return err
	}

	return nil
}

// returns error if the Messages does not meets the rules, otherwise nil
func (m Messages) IsValid() error {
	n := len(m)
	if n > 100000 {
		return fmt.Errorf("invalid message numbers: %d; must be less than 10000", n)
	}
	for _, msg := range m {
		if err := msg.IsValid(); err != nil {
			return err
		}
	}

	return nil
}

// ---------- CHAT GROUP ----------
// returns error if the GroupName does not meets the rules, otherwise nil
func (n GroupName) IsValid() error {
	l := CountChars(string(n))

	if l < 1 || l > 100 {
		return fmt.Errorf("invalid group name length: %d; must be between 1 and 100 characters", l)
	}

	return nil
}

// returns error if the GroupSummary does not meets the rules, otherwise nil
func (g *GroupSummary) IsValid() error {
	if g == nil {
		return errors.New("group summary is nil")
	}

	if err := g.ChatSummary.IsValid(); err != nil {
		return err
	}
	if g.Type != ChatTypeGroup {
		return fmt.Errorf("invalid group type: '%s'; must be 'group'", g.Type)
	}
	if err := g.Name.IsValid(); err != nil {
		return err
	}
	if err := g.Photo.IsValid(); err != nil {
		return err
	}

	return nil
}

// returns error if the GroupDetail does not meets the rules, otherwise nil
func (g *GroupDetail) IsValid() error {
	if g == nil {
		return errors.New("group details is nil")
	}

	if err := g.GroupSummary.IsValid(); err != nil {
		return err
	}
	if err := g.Messages.IsValid(); err != nil {
		return err
	}

	return nil
}

// ---------- CHAT PRIVATE ----------
// returns error if the PrivateChatSummary does not meets the rules, otherwise nil
func (p *PrivateChatSummary) IsValid() error {
	if p == nil {
		return errors.New("private summary is nil")
	}

	if err := p.ChatSummary.IsValid(); err != nil {
		return err
	}
	if n := len(p.Members); n != 2 {
		return fmt.Errorf("invalid private chat members: %d; must be exactly 2", n)
	}
	if p.Type != ChatTypePrivate {
		return fmt.Errorf("invalid private type: '%s'; must be 'private'", p.Type)
	}
	if err := p.Name.IsValid(); err != nil {
		return err
	}
	if err := p.Photo.IsValid(); err != nil {
		return err
	}

	return nil
}

// returns error if the PrivateChatDetail does not meets the rules, otherwise nil
func (p *PrivateChatDetail) IsValid() error {
	if p == nil {
		return errors.New("private details is nil")
	}

	if err := p.PrivateChatSummary.IsValid(); err != nil {
		return err
	}
	if err := p.Messages.IsValid(); err != nil {
		return err
	}

	return nil
}
