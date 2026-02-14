package schema

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	// Packages
	"github.com/mutablelogic/go-llm/pkg/opt"
	types "github.com/mutablelogic/go-server/pkg/types"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Message represents a message in a conversation with an LLM.
// It uses a universal content block representation that can be marshaled
// to any provider's format.
type Message struct {
	Role    string         `json:"role"`             // "user", "assistant", "system"
	Content []ContentBlock `json:"content"`          // Array of content blocks
	Tokens  uint           `json:"tokens,omitempty"` // Number of tokens
	Result  ResultType     `json:"result,omitempty"` // Result type
	Meta    map[string]any `json:"meta,omitzero"`    // Provider-specific metadata
}

// ContentBlock represents a single piece of content within a message.
// Exactly one of the fields should be non-nil/non-empty.
type ContentBlock struct {
	Text       *string     `json:"text,omitempty"`        // Text content
	Attachment *Attachment `json:"attachment,omitempty"`  // Image, document, audio, etc.
	ToolCall   *ToolCall   `json:"tool_call,omitempty"`   // Tool invocation (assistant → user)
	ToolResult *ToolResult `json:"tool_result,omitempty"` // Tool response (user → assistant)
}

// Attachment represents binary or URI-referenced media (images, documents, etc.)
type Attachment struct {
	Type string   `json:"type"`           // MIME type: "image/png", "application/pdf", etc.
	Data []byte   `json:"data,omitempty"` // Raw binary data
	URL  *url.URL `json:"url,omitempty"`  // URL reference (http, https, gs, file, etc.)
}

// ToolCall represents a tool invocation requested by the model
type ToolCall struct {
	ID    string          `json:"id,omitempty"`    // Provider-assigned call ID
	Name  string          `json:"name"`            // Tool function name
	Input json.RawMessage `json:"input,omitempty"` // JSON-encoded arguments
}

// ToolResult represents the result of running a tool
type ToolResult struct {
	ID      string          `json:"id,omitempty"`      // Matches the ToolCall ID
	Name    string          `json:"name,omitempty"`    // Tool function name
	Content json.RawMessage `json:"content,omitempty"` // JSON-encoded result
	IsError bool            `json:"is_error,omitempty"`
}

////////////////////////////////////////////////////////////////////////////////
// CONSTANTS

// Message role constants
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleSystem    = "system"
	RoleThinking  = "thinking"
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// Create a new message with the given role and text content
func NewMessage(role string, text string, opts ...opt.Opt) (*Message, error) {
	o, err := opt.Apply(opts...)
	if err != nil {
		return nil, err
	}

	// Create content blocks
	blocks := []ContentBlock{
		{Text: types.Ptr(text)},
	}
	if v := o.Get(opt.ContentBlockKey); v != nil {
		if attachments, ok := v.([]ContentBlock); !ok {
			return nil, fmt.Errorf("invalid attachments option")
		} else {
			blocks = append(blocks, attachments...)
		}
	}

	// Return the message
	return types.Ptr(Message{
		Role:    role,
		Content: blocks,
	}), nil
}

// NewToolResult creates a content block containing a successful tool result
func NewToolResult(id, name string, v any) ContentBlock {
	data, err := json.Marshal(v)
	if err != nil {
		return NewToolError(id, name, err)
	}
	return ContentBlock{
		ToolResult: &ToolResult{
			ID:      id,
			Name:    name,
			Content: json.RawMessage(data),
		},
	}
}

// NewToolError creates a content block containing a tool error result
func NewToolError(id, name string, err error) ContentBlock {
	return ContentBlock{
		ToolResult: &ToolResult{
			ID:      id,
			Name:    name,
			Content: json.RawMessage(fmt.Sprintf("%q", err.Error())),
			IsError: true,
		},
	}
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Text returns the concatenated text content from all text blocks in the message
func (m Message) Text() string {
	var result []string
	for _, block := range m.Content {
		if block.Text != nil {
			result = append(result, *block.Text)
		}
	}
	return strings.Join(result, "\n")
}

// ToolCalls returns all tool call blocks in the message
func (m Message) ToolCalls() []ToolCall {
	var result []ToolCall
	for _, block := range m.Content {
		if block.ToolCall != nil {
			result = append(result, *block.ToolCall)
		}
	}
	return result
}

////////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (m Message) String() string {
	return types.Stringify(m)
}

////////////////////////////////////////////////////////////////////////////////
// MESSAGE OPTIONS

// WithAttachmentURL creates an attachment from data read from the provided reader
// The MIME type is detected from the data. This is suitable for small attachments
// the caller is responsible for closing the reader after the data is read.
func WithAttachment(r io.Reader) opt.Opt {
	data, err := io.ReadAll(r)
	if err != nil {
		return opt.Error(err)
	}
	return opt.AddAny(opt.ContentBlockKey, ContentBlock{
		Attachment: types.Ptr(Attachment{
			Type: http.DetectContentType(data),
			Data: data,
		}),
	})
}

// WithAttachmentURL creates an attachment from a URL and explicit MIME type
func WithAttachmentURL(u string, mimetype string) opt.Opt {
	url, err := url.Parse(u)
	if err != nil {
		return opt.Error(fmt.Errorf("invalid URL: %w", err))
	}
	return opt.AddAny(opt.ContentBlockKey, ContentBlock{
		Attachment: types.Ptr(Attachment{
			Type: mimetype,
			URL:  url,
		}),
	})
}
