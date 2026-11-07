package schema

import (
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	types "github.com/mutablelogic/go-server/pkg/types"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// MessageMeta is a message as it's sent to a model: its role, which is the
// user's when omitted, and its content. It uses a universal content block
// representation that can be marshaled to any provider's format.
type MessageMeta struct {
	Sender  string         `json:"role,omitempty" help:"Message role, which is user when omitted" enum:"user,assistant,system,thinking,tool" example:"user" optional:""`
	Content []ContentBlock `json:"content" help:"Structured content blocks that make up the message" example:"[{\"text\":\"Unit tests catch regressions early and make refactoring safer.\"}]"`
}

// Message is a message in a conversation with a model, as a provider
// replies with it, with the tokens it used, and why the model stopped
// generating it
type Message struct {
	MessageMeta
	Tokens uint64     `json:"tokens,omitempty" help:"Token count attributed to this message" example:"12"`
	Result ResultType `json:"result,omitempty" help:"Why the model stopped generating the message" enum:"stop,max_tokens,blocked,tool_call,error,other,max_iterations,max_context" example:"stop"`
}

// ContentBlock represents a single piece of content within a message.
// Exactly one of the fields should be non-nil/non-empty, apart from the
// signature, which is with the thinking it signs, or what a model made,
// such as an image, which Gemini signs.
type ContentBlock struct {
	Text       *string     `json:"text,omitempty" help:"Plain text content emitted by the model" example:"Unit tests catch regressions early and make refactoring safer."`
	Thinking   *string     `json:"thinking,omitempty" help:"Reasoning or thinking text emitted by the model" example:"I should keep this answer brief and concrete."`
	Signature  *string     `json:"signature,omitempty" help:"Signature of the thinking, which a provider, such as Anthropic, returns with it, so it's sent back as it was" example:"EqQBCgIYAhIM..."`
	Redacted   *string     `json:"redacted_thinking,omitempty" help:"Thinking which the provider, such as Anthropic, returned encrypted, which is sent back as it was" example:"EmwKAhgBEgy3va3pzix..."`
	Resource   *Resource   `json:"attachment,omitempty" help:"Attachment content such as an image, document, or audio asset" example:"{\"type\":\"image/png\",\"url\":\"https://example.com/image.png\"}"`
	ToolCall   *ToolCall   `json:"tool_call,omitempty" help:"Tool invocation requested by the model" example:"{\"id\":\"call_123\",\"name\":\"get_weather\",\"input\":{\"city\":\"London\"}}"`
	ToolResult *ToolResult `json:"tool_result,omitempty" help:"Tool execution result returned to the model" example:"{\"id\":\"call_123\",\"name\":\"get_weather\",\"content\":{\"temperature_c\":18},\"is_error\":false}"`
}

var _ llm.Message = (*MessageMeta)(nil)
var _ llm.Message = (*Message)(nil)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// MessageFrom returns the message, which is a schema message, or otherwise a
// new message with the message's role and text. Resources are left out of a
// new message, as they'd need to be read.
func MessageFrom(message llm.Message) *Message {
	switch m := message.(type) {
	case nil:
		return nil
	case *Message:
		return m
	case Message:
		return types.Ptr(m)
	}
	return types.Ptr(Message{MessageMeta: MessageMeta{
		Sender:  message.Role(),
		Content: []ContentBlock{{Text: types.Ptr(message.Text())}},
	}})
}

// Create a new message with the given role and text content
func NewMessage(role string, text string) (*Message, error) {
	return types.Ptr(Message{MessageMeta: MessageMeta{
		Sender:  role,
		Content: []ContentBlock{{Text: types.Ptr(text)}},
	}}), nil
}

////////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (m Message) String() string {
	return types.Stringify(m)
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Text returns the text content blocks of the message, joined with newlines
func (m MessageMeta) Text() string {
	var result []string
	for _, block := range m.Content {
		if block.Text != nil {
			result = append(result, *block.Text)
		}
	}
	return strings.Join(result, "\n")
}

// Thinking returns the thinking content blocks of the message, joined with
// newlines, which some providers need sent back with the message's tool calls
func (m MessageMeta) Thinking() string {
	var result []string
	for _, block := range m.Content {
		if block.Thinking != nil {
			result = append(result, *block.Thinking)
		}
	}
	return strings.Join(result, "\n")
}

// Resources returns the resource content blocks of the message
func (m MessageMeta) Resources() []llm.Resource {
	var result []llm.Resource
	for _, block := range m.Content {
		if block.Resource != nil {
			result = append(result, block.Resource)
		}
	}
	return result
}

// ToolCalls returns the tool call content blocks of the message
func (m MessageMeta) ToolCalls() []llm.ToolCall {
	var result []llm.ToolCall
	for _, block := range m.Content {
		if call := block.ToolCall; call != nil {
			result = append(result, llm.ToolCall{ID: call.ID, Name: call.Name, Input: call.Input})
		}
	}
	return result
}

// ToolResults returns the tool result content blocks of the message
func (m MessageMeta) ToolResults() []llm.ToolResult {
	var result []llm.ToolResult
	for _, block := range m.Content {
		if r := block.ToolResult; r != nil {
			result = append(result, llm.ToolResult{ID: r.ID, Name: r.Name, Content: r.Content, IsError: r.IsError})
		}
	}
	return result
}

// Role returns the role of the message, which is the user's when it's
// omitted
func (m MessageMeta) Role() string {
	if m.Sender == "" {
		return "user"
	}
	return m.Sender
}
