package schema

import (
	"bytes"
	"encoding/json"
	"maps"
	"net/url"
	"slices"
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

// SessionMeta is a session's settings: how its model is asked
type SessionMeta struct {
	GeneratorMeta
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
	Messages []providerschema.Message `json:"messages,omitzero" help:"Messages of the conversation, each with its tokens, as it's stored, which are listed on their own"`
	Usage    SessionUsage             `json:"usage,omitzero" help:"Tokens used by every request to the model, and of the conversation, as of the last"`
	Status   SessionStatus            `json:"status,omitempty" readonly:"" help:"Who's doing what in the session, which is typing, replying or idle, which the registry sets, and isn't stored" enum:"idle,typing,replying" example:"idle"`
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
	Role   string `json:"role,omitempty" help:"Returns only the messages of this role, such as user, assistant or tool" enum:"user,assistant,system,thinking,tool" example:"assistant" optional:""`
	Offset uint   `json:"offset,omitempty" help:"Messages to skip" example:"0" optional:""`
	Limit  *uint  `json:"limit,omitempty" help:"Most messages to return" example:"10" optional:""`
}

// MessageList is a list of a session's messages, oldest first, and how many
// there are, of the role
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
	return s.Model == nil && s.System == nil && s.Tools == nil && len(s.Opt) == 0 && len(bytes.TrimSpace(s.Schema)) == 0
}

// Merge returns the settings, changed by those of the update: its model, and
// its system prompt, when they're set, which empty clears; its tools, when they're set, which an
// empty list clears; its schema, when it's set, which null clears; and its
// options, each of which is set, or removed, when it's null. What the update
// doesn't set is kept.
func (s SessionMeta) Merge(update SessionMeta) SessionMeta {
	result := s
	if update.Model != nil {
		if model := strings.TrimSpace(*update.Model); model == "" {
			result.Model = nil
		} else {
			result.Model = types.Ptr(model)
		}
	}
	if update.System != nil {
		if system := strings.TrimSpace(*update.System); system == "" {
			result.System = nil
		} else {
			result.System = types.Ptr(*update.System)
		}
	}
	if update.Tools != nil {
		result.Tools = nil
		if len(update.Tools) > 0 {
			result.Tools = slices.Clone(update.Tools)
		}
	}
	if schema := bytes.TrimSpace(update.Schema); len(schema) > 0 {
		if string(schema) == "null" {
			result.Schema = nil
		} else {
			result.Schema = slices.Clone(update.Schema)
		}
	}
	if len(update.Opt) > 0 {
		opt := maps.Clone(s.Opt)
		if opt == nil {
			opt = make(map[string]json.RawMessage, len(update.Opt))
		}
		for key, value := range update.Opt {
			if value := bytes.TrimSpace(value); len(value) == 0 || string(value) == "null" {
				delete(opt, key)
			} else {
				opt[key] = slices.Clone(value)
			}
		}
		if len(opt) == 0 {
			opt = nil
		}
		result.Opt = opt
	}
	return result
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
	if r.Offset > 0 {
		values.Set("offset", strconv.FormatUint(uint64(r.Offset), 10))
	}
	if r.Limit != nil {
		values.Set("limit", strconv.FormatUint(uint64(*r.Limit), 10))
	}
	return values
}

///////////////////////////////////////////////////////////////////////////////
// TABLE

func (s Session) Header() []string {
	return []string{"ID", "Model", "Tools", "Context", "Modified"}
}

func (s Session) Width(col int) int {
	return 0
}

// Cell returns the session's identifier, model, tools, the tokens of its
// conversation, as of its last request, and when it was last modified
func (s Session) Cell(col int) tui.CellText {
	switch col {
	case 0:
		return s.ID.String()
	case 1:
		return types.Value(s.Model)
	case 2:
		return strings.Join(s.Tools, ", ")
	case 3:
		if tokens := s.Usage.Input + s.Usage.Output + s.Usage.Overhead; tokens > 0 {
			return strconv.FormatUint(uint64(tokens), 10)
		}
		return ""
	case 4:
		if s.Modified.IsZero() {
			return ""
		}
		return s.Modified.Local().Format(time.DateTime)
	default:
		return ""
	}
}
