package schema

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	// Packages
	uuid "github.com/google/uuid"
	pg "github.com/mutablelogic/go-pg"
	types "github.com/mutablelogic/go-server/pkg/types"
)

////////////////////////////////////////////////////////////////////////////////
// CONSTANTS

// DefaultMaxIterations is the default maximum number of tool-calling iterations
// per chat turn.
const DefaultMaxIterations = 10

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Conversation is a sequence of messages exchanged with an LLM
type Conversation []*Message

// Session represents a stored conversation with an LLM.
type Session struct {
	ID uuid.UUID `json:"id"`
	SessionInsert
	Overhead   uint         `json:"overhead,omitempty" help:"Constant token cost per turn (tools, system prompt)"`
	Messages   Conversation `json:"messages" help:"Messages in the session conversation"`
	CreatedAt  time.Time    `json:"created_at" help:"Creation timestamp"`
	ModifiedAt *time.Time   `json:"modified_at,omitempty" help:"Last modification timestamp" optional:""`
}

// SessionMeta represents the metadata for a session.
type SessionMeta struct {
	GeneratorMeta
	Title string   `json:"title,omitempty" help:"Session title" optional:""`
	Tags  []string `json:"tags,omitempty" help:"User-defined tags" optional:""`
}

type SessionInsert struct {
	Parent uuid.UUID `json:"parent,omitempty" help:"Parent session for threading" optional:""`
	User   uuid.UUID `json:"user" help:"User owning the session"`
	SessionMeta
}

// SessionListRequest represents a request to list sessions.
type SessionListRequest struct {
	pg.OffsetLimit
	Parent *uuid.UUID `json:"parent,omitempty" help:"Filter by parent session ID" optional:""`
	User   *uuid.UUID `json:"user,omitempty" help:"Filter by user ID" optional:""`
	Title  *string    `json:"title,omitempty" help:"Filter by session title (partial match)" optional:""`
	Tags   []string   `json:"tags,omitempty" help:"Filter by tags (sessions must contain all specified tags)" optional:""`
}

// SessionList represents a response containing a list of sessions.
type SessionList struct {
	SessionListRequest
	Count uint       `json:"count"`
	Body  []*Session `json:"body,omitzero"`
}

// SessionIDSelector selects a session by ID for get, update, and delete operations.
type SessionIDSelector uuid.UUID

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	SessionListMax uint64 = 100
)

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - STRINGIFY

func (s Session) String() string {
	return types.Stringify(s)
}

func (s SessionMeta) String() string {
	return types.Stringify(s)
}

func (s SessionInsert) String() string {
	return types.Stringify(s)
}

func (s SessionListRequest) String() string {
	return types.Stringify(s)
}

func (s SessionList) String() string {
	return types.Stringify(s)
}

func (c Conversation) String() string {
	return types.Stringify(c)
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - CONVERSATION

// Append adds a message to the conversation
func (s *Conversation) Append(message Message) {
	*s = append(*s, &message)
}

// AppendWithOutput adds a message to the conversation, attributing token
// counts to individual messages. The last message in the conversation
// (typically the just-appended user message) receives an estimated token
// count based on its content rather than absorbing overhead such as tool
// schemas and system prompts. The response message receives the actual
// output token count from the provider.
func (s *Conversation) AppendWithOuput(message Message, input, output uint) {
	// Estimate tokens for the last message (the user message just appended
	// by WithSession) so it reflects only its content cost.
	if n := len(*s); n > 0 && (*s)[n-1].Tokens == 0 {
		(*s)[n-1].Tokens = (*s)[n-1].EstimateTokens()
	}

	// Filter out empty content blocks — some providers (notably Gemini)
	// produce empty text parts during streaming which are rejected when
	// sent back as context.
	filtered := message.Content[:0]
	for _, block := range message.Content {
		if block.Text != nil && *block.Text == "" {
			continue
		}
		filtered = append(filtered, block)
	}
	message.Content = filtered

	// Set the output tokens on the response message
	message.Tokens = output

	// Append the message
	*s = append(*s, &message)
}

// Return the total number of tokens in the conversation
func (s Conversation) Tokens() uint {
	total := uint(0)
	for _, msg := range s {
		total += msg.Tokens
	}
	return total
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - SESSION

// Append adds a message to the session and updates the modified timestamp.
func (s *Session) Append(message Message) {
	s.Messages.Append(message)
	s.ModifiedAt = types.Ptr(time.Now())
}

// Tokens returns the total token count across all messages.
func (s *Session) Tokens() uint {
	return s.Messages.Tokens() + s.Overhead
}

// Conversation returns a pointer to the underlying message slice,
// compatible with generator.WithSession.
func (s *Session) Conversation() *Conversation {
	return &s.Messages
}

// Validate returns an error if the session is missing required fields.
func (s *Session) Validate() error {
	return nil
}

// Generator returns the session generator settings.
func (s SessionMeta) Generator() GeneratorMeta {
	return s.GeneratorMeta
}

// Generator returns the session generator settings.
func (s Session) Generator() GeneratorMeta {
	return s.GeneratorMeta
}

////////////////////////////////////////////////////////////////////////////////
// QUERY

func (req SessionListRequest) Query() url.Values {
	values := url.Values{}
	if req.Offset > 0 {
		values.Set("offset", strconv.FormatUint(req.Offset, 10))
	}
	if req.Limit != nil {
		values.Set("limit", strconv.FormatUint(types.Value(req.Limit), 10))
	}
	if req.Parent != nil && *req.Parent != uuid.Nil {
		values.Set("parent", req.Parent.String())
	}
	if req.User != nil && *req.User != uuid.Nil {
		values.Set("user", req.User.String())
	}
	if req.Title != nil && strings.TrimSpace(*req.Title) != "" {
		values.Set("title", strings.TrimSpace(*req.Title))
	}
	for _, tag := range normalizeSessionTags(req.Tags) {
		values.Add("tag", tag)
	}
	return values
}

////////////////////////////////////////////////////////////////////////////////
// SELECTORS

func (s SessionIDSelector) Select(bind *pg.Bind, op pg.Op) (string, error) {
	id := uuid.UUID(s)
	if id == uuid.Nil {
		return "", ErrBadParameter.With("session id is required")
	}
	bind.Set("id", id)

	switch op {
	case pg.Get:
		return bind.Query("session.select"), nil
	case pg.Update:
		return bind.Query("session.update"), nil
	case pg.Delete:
		return bind.Query("session.delete"), nil
	default:
		return "", ErrInternalServerError.Withf("unsupported SessionIDSelector operation %q", op)
	}
}

func (req SessionListRequest) Select(bind *pg.Bind, op pg.Op) (string, error) {
	bind.Del("where")

	if req.Parent != nil {
		if *req.Parent == uuid.Nil {
			return "", ErrBadParameter.With("parent session id cannot be nil")
		}
		bind.Append("where", `session.parent = `+bind.Set("parent", *req.Parent))
	}
	if req.User != nil {
		if *req.User == uuid.Nil {
			return "", ErrBadParameter.With("user id cannot be nil")
		}
		bind.Append("where", `session."user" = `+bind.Set("user", *req.User))
	}
	if req.Title != nil && strings.TrimSpace(*req.Title) != "" {
		bind.Append("where", `session.title ILIKE `+bind.Set("title", "%"+strings.TrimSpace(*req.Title)+"%"))
	}
	if tags := normalizeSessionTags(req.Tags); len(tags) > 0 {
		bind.Append("where", `COALESCE(session.tags, '{}'::text[]) @> `+bind.Set("tags", tags))
	}

	where := bind.Join("where", " AND ")
	if where == "" {
		bind.Set("where", "")
	} else {
		bind.Set("where", "WHERE "+where)
	}
	bind.Set("orderby", `ORDER BY COALESCE(session.modified_at, session.created_at) DESC, session.id ASC`)
	req.OffsetLimit.Bind(bind, SessionListMax)

	switch op {
	case pg.List:
		return bind.Query("session.list"), nil
	default:
		return "", ErrNotImplemented.Withf("unsupported SessionListRequest operation %q", op)
	}
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - READER

// Expected column order: id, parent, user, title, overhead, meta, tags, created_at, modified_at.
func (s *Session) Scan(row pg.Row) error {
	var parent *uuid.UUID
	var meta url.Values
	if err := row.Scan(
		&s.ID,
		&parent,
		&s.User,
		&s.Title,
		&s.Overhead,
		&meta,
		&s.Tags,
		&s.CreatedAt,
		&s.ModifiedAt,
	); err != nil {
		return err
	}
	if parent != nil {
		s.Parent = *parent
	} else {
		s.Parent = uuid.Nil
	}
	s.GeneratorMeta = GeneratorMetaFromValues(meta)
	if s.Tags == nil {
		s.Tags = []string{}
	}
	return nil
}

func (list *SessionList) Scan(row pg.Row) error {
	var session Session
	if err := session.Scan(row); err != nil {
		return err
	}
	list.Body = append(list.Body, &session)
	return nil
}

func (list *SessionList) ScanCount(row pg.Row) error {
	if err := row.Scan(&list.Count); err != nil {
		return err
	}
	return nil
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - WRITER

func (s SessionInsert) Insert(bind *pg.Bind) (string, error) {
	if s.User == uuid.Nil {
		return "", ErrBadParameter.With("session user is required")
	}
	if s.Parent == uuid.Nil {
		bind.Set("parent", nil)
	} else {
		bind.Set("parent", s.Parent)
	}
	bind.Set("user", s.User)

	if title := strings.TrimSpace(s.Title); title != "" {
		bind.Set("title", title)
	} else {
		bind.Set("title", nil)
	}

	meta := cloneSessionValues(s.GeneratorMeta.Values())
	if meta == nil {
		meta = make(url.Values)
	}
	bind.Set("meta", meta)
	bind.Set("tags", normalizeSessionTags(s.Tags))

	return bind.Query("session.insert"), nil
}

func (s SessionInsert) Update(_ *pg.Bind) error {
	return fmt.Errorf("SessionInsert: update: not supported")
}

func (s SessionMeta) Insert(_ *pg.Bind) (string, error) {
	return "", fmt.Errorf("SessionMeta: insert: not supported")
}

func (s SessionMeta) Update(bind *pg.Bind) error {
	bind.Del("patch")

	if title := strings.TrimSpace(s.Title); title != "" {
		bind.Append("patch", `title = `+bind.Set("title", title))
	}
	if meta := cloneSessionValues(s.GeneratorMeta.Values()); meta != nil {
		bind.Append("patch", `meta = `+bind.Set("meta", meta))
	}
	if s.Tags != nil {
		bind.Append("patch", `tags = `+bind.Set("tags", normalizeSessionTags(s.Tags)))
	}

	patch := bind.Join("patch", ", ")
	if patch == "" {
		return ErrBadParameter.With("no fields to update")
	}
	bind.Set("patch", patch)
	return nil
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func cloneSessionValues(values url.Values) url.Values {
	if len(values) == 0 {
		return nil
	}
	clone := make(url.Values, len(values))
	for key, vals := range values {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			continue
		}
		for _, value := range vals {
			clone[trimmedKey] = append(clone[trimmedKey], strings.TrimSpace(value))
		}
	}
	if len(clone) == 0 {
		return nil
	}
	return clone
}

func normalizeSessionTags(tags []string) []string {
	if len(tags) == 0 {
		return []string{}
	}
	result := make([]string, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if _, exists := seen[tag]; exists {
			continue
		}
		seen[tag] = struct{}{}
		result = append(result, tag)
	}
	return result
}
