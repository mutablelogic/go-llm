package schema

import (
	"encoding/json"
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
	ID   uuid.UUID `json:"id"`
	User uuid.UUID `json:"user" help:"User owning the session"`
	SessionInsert
	Input      uint       `json:"input,omitempty" help:"Cumulative input message tokens for the session"`
	Output     uint       `json:"output,omitempty" help:"Cumulative output message tokens for the session"`
	Overhead   uint       `json:"overhead,omitempty" help:"Cumulative non-message input token cost observed across the session"`
	CreatedAt  time.Time  `json:"created_at" help:"Creation timestamp"`
	ModifiedAt *time.Time `json:"modified_at,omitempty" help:"Last modification timestamp" optional:""`
}

// SessionMeta represents the metadata for a session.
type SessionMeta struct {
	GeneratorMeta
	Title *string  `json:"title,omitempty" help:"Session title" optional:""`
	Tags  []string `json:"tags,omitempty" help:"User-defined tags" optional:""`
}

type SessionInsert struct {
	Parent uuid.UUID `json:"parent,omitempty" help:"Parent session for threading" optional:""`
	SessionMeta
}

// SessionListRequest represents a request to list sessions.
type SessionListRequest struct {
	pg.OffsetLimit
	Parent *uuid.UUID `json:"parent,omitzero" help:"Filter by parent session ID" optional:""`
	User   *uuid.UUID `json:"user,omitzero" help:"Filter by user ID" optional:""`
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
// PUBLIC METHODS - CONVERSATION

func (c Conversation) Len() int {
	return len(c)
}

func (c Conversation) Last(n int) *Message {
	if n < 0 || n >= c.Len() {
		return nil
	}
	return c[c.Len()-n-1]
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - STRINGIFY

func (s Session) String() string {
	return types.Stringify(s)
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - CONVERSATION

// MarshalJSON omits User and Parent when they are the zero UUID.
func (s Session) MarshalJSON() ([]byte, error) {
	type alias Session
	type output struct {
		alias
		User   *uuid.UUID `json:"user,omitempty"`
		Parent *uuid.UUID `json:"parent,omitempty"`
	}
	o := output{alias: alias(s)}
	if s.User != uuid.Nil {
		o.User = &s.User
	}
	if s.Parent != uuid.Nil {
		o.Parent = &s.Parent
	}
	return json.Marshal(o)
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
	if session := uuid.UUID(s); session == uuid.Nil {
		return "", ErrBadParameter.With("session is required")
	} else {
		bind.Set("id", session)
	}

	// Set userwhere: if a non-nil user UUID is present, restrict to that user.
	if user, _ := bind.Get("user").(uuid.UUID); user != uuid.Nil {
		bind.Set("userwhere", `AND session."user" = @user`)
		bind.Set("user", user)
	} else {
		bind.Set("userwhere", "")
		bind.Del("user")
	}

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

// Expected column order: id, parent, user, title, input, output, overhead, meta, tags, created_at, modified_at.
func (s *Session) Scan(row pg.Row) error {
	var parent *uuid.UUID
	var user *uuid.UUID
	var meta url.Values
	if err := row.Scan(
		&s.ID,
		&parent,
		&user,
		&s.Title,
		&s.Input,
		&s.Output,
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
	if user != nil {
		s.User = *user
	} else {
		s.User = uuid.Nil
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
	if uid, ok := bind.Get("user").(uuid.UUID); ok && uid == uuid.Nil {
		bind.Set("user", (*uuid.UUID)(nil))
	}
	if s.Parent == uuid.Nil {
		bind.Set("parent", (*uuid.UUID)(nil))
	} else {
		bind.Set("parent", s.Parent)
	}

	var title *string
	if s.Title != nil {
		if t := strings.TrimSpace(*s.Title); t != "" {
			title = &t
		}
	}
	bind.Set("title", title)

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

	if s.Title != nil {
		t := strings.TrimSpace(*s.Title)
		if t == "" {
			// Explicit empty string clears the title.
			bind.Append("patch", `title = `+bind.Set("title", (*string)(nil)))
		} else {
			bind.Append("patch", `title = `+bind.Set("title", t))
		}
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
