package schema

import (
	"net/url"
	"strconv"
	"strings"
	"time"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	tui "github.com/mutablelogic/go-server/pkg/tui"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// SessionMeta is a session's settings: its title, for people, and how its
// model is asked
type SessionMeta struct {
	// Title is what the session is about, for people, such as an event's
	// title, which its session takes
	Title *string `json:"title,omitempty" help:"Title of the session, for people, such as an event's, which its session takes" example:"Morning briefing" optional:""`

	GeneratorMeta

	// Trace is whether the session's clients show its model's thinking,
	// the tools it calls, and their results, in the conversation, as
	// they're streamed. They're always stored, either way.
	Trace *bool `json:"trace,omitempty" name:"trace" help:"Whether clients show the model's thinking, the tools it calls, and their results, in the conversation, which are always stored" negatable:"" optional:""`

	// Event is whether the session is an event's, such as a heartbeat's,
	// which the registry runs it in, so it isn't chatted in. It's set by the
	// registry, rather than by its API, which ignores it.
	Event bool `json:"event,omitempty" readonly:"" help:"Whether the session is an event's, such as a heartbeat's, which runs in it, so it isn't chatted in, which only the registry sets"`
}

// Session is a conversation with a model, as it's stored: its identifier,
// and serial, which its store sets, and changes each time it's stored, so a
// change is made to the session as it was last seen, its settings, its
// messages, each with its tokens, and its usage. The registry returns a
// session without its messages, which are listed on their own.
type Session struct {
	ID       uuid.UUID `json:"id" readonly:"" help:"Identifier of the session, which its store sets" example:"3b241101-e2bb-4255-8caf-4136c566a962"`
	Serial   uint64    `json:"serial,omitempty" readonly:"" help:"Serial of the session, which its store changes each time it's stored" example:"3"`
	Created  time.Time `json:"created,omitzero" readonly:"" help:"When the session was created"`
	Modified time.Time `json:"modified,omitzero" readonly:"" help:"When the session was last stored"`
	SessionMeta
	Messages  []providerschema.Message `json:"messages,omitzero" help:"Messages of the conversation, each with its tokens, as it's stored, which are listed on their own"`
	Forgotten uint64                   `json:"forgotten,omitempty" readonly:"" help:"How many times the session's messages were forgotten, which the registry counts, so a reply to a conversation which was forgotten isn't stored" example:"1"`
	Usage     SessionUsage             `json:"usage,omitzero" help:"Tokens used by every request to the model, and of the conversation, as of the last"`
	Status    SessionStatus            `json:"status,omitempty" readonly:"" help:"Who's doing what in the session, which is typing, replying or idle, which the registry sets, and isn't stored" enum:"idle,typing,replying" example:"idle"`
}

// SessionUsage is the tokens used by requests to a model, and the tokens of
// a session's conversation, of the messages sent to the model, of its
// replies, and of what's sent with them, such as the system prompt and the
// tools, which together are about the tokens of the next request
type SessionUsage struct {
	llm.Usage
	Input    uint `json:"input,omitempty" help:"Tokens of the conversation's messages sent to the model, such as from the user, and the results of tools" example:"1200"`
	Output   uint `json:"output,omitempty" help:"Tokens of the conversation's replies from the model" example:"300"`
	Overhead uint `json:"overhead,omitempty" help:"Tokens of what's sent with the messages, such as the system prompt and the tools" example:"150"`
}

// SessionListRequest is a request to list sessions, from the offset, and at
// most the limit of them, or all of them, when there's no limit
type SessionListRequest struct {
	Offset uint  `json:"offset,omitempty" help:"Sessions to skip" example:"0" optional:""`
	Limit  *uint `json:"limit,omitempty" help:"Most sessions to return" example:"10" optional:""`
}

// SessionList is a list of sessions, without their messages, and how many
// sessions there are
type SessionList struct {
	SessionListRequest
	Count uint      `json:"count" help:"How many sessions there are" example:"42"`
	Body  []Session `json:"body,omitempty" help:"The sessions, without their messages"`
}

// MessageListRequest is a request to list a session's messages, oldest
// first, of a role, or every role, from the offset, and at most the limit of
// them, or all of them, when there's no limit
type MessageListRequest struct {
	Role    string `json:"role,omitempty" help:"Returns only the messages of this role, such as user, assistant or tool" enum:"user,assistant,system,thinking,tool,context" example:"assistant" optional:""`
	Context bool   `json:"context,omitempty" help:"Returns only the messages of the conversation's context, which a model is sent: its last marker, with the summary of what came before it, and the messages after it" optional:""`
	Offset  int    `json:"offset,omitempty" help:"Messages to skip, or, when it's negative, how many from the end to start at, such as -50 for the last 50, which the list's offset is, from the start" example:"0" optional:""`
	Limit   *uint  `json:"limit,omitempty" help:"Most messages to return" example:"10" optional:""`
}

// MessageList is a list of a session's messages, oldest first, and how many
// there are, of the role, or of its context, from the offset, which is from
// the start, when the request's is from the end
type MessageList struct {
	MessageListRequest
	Count uint                     `json:"count" help:"How many messages there are, of the role" example:"42"`
	Body  []providerschema.Message `json:"body,omitempty" help:"The messages, each with its tokens"`
}

// ChatRequest is the messages to add to a session, which its model replies to
type ChatRequest struct {
	Messages []providerschema.MessageMeta `json:"messages" help:"Messages to add to the session, such as a question, which the session's model replies to"`
}

// ChatResponse is what was added to a session, which is stored, with its
// identifier, and its serial, as it was stored
type ChatResponse struct {
	ID     uuid.UUID `json:"id" help:"Identifier of the session" example:"3b241101-e2bb-4255-8caf-4136c566a962"`
	Serial uint64    `json:"serial" help:"Serial of the session, as it was stored" example:"4"`
	SessionResponse
}

// SessionResponse is what's added to a session's conversation: the
// messages, the last of which is the model's reply, each with its tokens,
// and the usage of every request to the model, with the tokens of the
// conversation, with the messages. With a schema, the model's reply, when
// its result is stop, is JSON which conforms to it.
type SessionResponse struct {
	Messages []providerschema.Message `json:"messages" help:"Messages to add to the conversation: those sent, and those added, the last of which is the model's reply, each with its tokens"`
	Usage    SessionUsage             `json:"usage" help:"Tokens used, by every request to the model, and of the conversation, with the messages"`
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (s SessionMeta) String() string {
	return types.Stringify(s)
}

func (s Session) String() string {
	return types.Stringify(s)
}

func (s SessionUsage) String() string {
	return types.Stringify(s)
}

func (s SessionListRequest) String() string {
	return types.Stringify(s)
}

func (s SessionList) String() string {
	return types.Stringify(s)
}

func (r MessageListRequest) String() string {
	return types.Stringify(r)
}

func (r MessageList) String() string {
	return types.Stringify(r)
}

func (r ChatRequest) String() string {
	return types.Stringify(r)
}

func (r ChatResponse) String() string {
	return types.Stringify(r)
}

func (s SessionResponse) String() string {
	return types.Stringify(s)
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// IsZero returns true when the settings set nothing, so as an update, they
// change nothing
func (s SessionMeta) IsZero() bool {
	return s.Title == nil && s.Trace == nil && s.GeneratorMeta.IsZero()
}

// Merge returns the settings, changed by those of the update: its title,
// when it's set, which empty clears, whether it's traced, when that's set,
// and the rest, as GeneratorMeta.Merge does. Whether it's an event's isn't
// changed, as the registry sets it.
func (s SessionMeta) Merge(update SessionMeta) SessionMeta {
	title, trace := s.Title, s.Trace
	if update.Title != nil {
		title = trimmed(update.Title)
	}
	if update.Trace != nil {
		trace = types.Ptr(*update.Trace)
	}
	return SessionMeta{Title: title, GeneratorMeta: s.GeneratorMeta.Merge(update.GeneratorMeta), Trace: trace, Event: s.Event}
}

// Traced returns whether the session's clients show its model's thinking,
// the tools it calls, and their results, which they don't, unless it's set
func (s SessionMeta) Traced() bool {
	return types.Value(s.Trace)
}

// trimmed returns the text, without the space around it, or nil, when it's
// nil, or empty
func trimmed(s *string) *string {
	if s == nil {
		return nil
	} else if t := strings.TrimSpace(*s); t != "" {
		return &t
	}
	return nil
}

// Query returns the request as the parameters of a URL's query
func (r SessionListRequest) Query() url.Values {
	values := url.Values{}
	if r.Offset > 0 {
		values.Set("offset", strconv.FormatUint(uint64(r.Offset), 10))
	}
	if r.Limit != nil {
		values.Set("limit", strconv.FormatUint(uint64(*r.Limit), 10))
	}
	return values
}

// Query returns the request as the parameters of a URL's query
func (r MessageListRequest) Query() url.Values {
	values := url.Values{}
	if r.Role != "" {
		values.Set("role", r.Role)
	}
	if r.Context {
		values.Set("context", "true")
	}
	if r.Offset != 0 {
		values.Set("offset", strconv.Itoa(r.Offset))
	}
	if r.Limit != nil {
		values.Set("limit", strconv.FormatUint(uint64(*r.Limit), 10))
	}
	return values
}

///////////////////////////////////////////////////////////////////////////////
// TABLE

func (s Session) Header() []string {
	return []string{"ID", "Title", "Model", "Tools", "Context", "Updated"}
}

func (s Session) Width(col int) int {
	return 0
}

// Cell returns the session's identifier, title, model, tools, the tokens of its
// conversation, as of its last request, and when it was last updated, which
// is when it was created, or modified, whichever is later
func (s Session) Cell(col int) tui.CellText {
	switch col {
	case 0:
		return s.ID.String()
	case 1:
		return types.Value(s.Title)
	case 2:
		return types.Value(s.Model)
	case 3:
		return strings.Join(s.Tools, ", ")
	case 4:
		if tokens := s.Usage.Input + s.Usage.Output + s.Usage.Overhead; tokens > 0 {
			return strconv.FormatUint(uint64(tokens), 10)
		}
		return ""
	case 5:
		return updated(s.Created, s.Modified)
	default:
		return ""
	}
}
