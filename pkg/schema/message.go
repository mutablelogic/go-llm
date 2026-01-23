package schema

import (
	"encoding/json"
	"strings"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Message represents a message in a conversation with an LLM.
// It uses a universal content block representation that can be marshaled
// to any provider's format.
type Message struct {
	Role    string         `json:"role"`    // "user", "assistant", "system", "tool", etc.
	Content []ContentBlock `json:"content"` // Array of content blocks
	Tokens  uint           `json:"-"`       // Number of tokens (not serialized)
}

// ContentBlock represents a single piece of content within a message.
// It's a superset of all content types across all providers.
type ContentBlock struct {
	Type string `json:"type"` // "text", "image", "tool_use", "tool_result", "thinking", "document", etc.

	// Text content
	Text *string `json:"text,omitempty"`

	// Image content
	ImageSource *ImageSource `json:"image_source,omitempty"`

	// Document content (Anthropic)
	DocumentSource  *DocumentSource  `json:"document_source,omitempty"`
	DocumentTitle   *string          `json:"document_title,omitempty"`
	DocumentContext *string          `json:"document_context,omitempty"`
	Citations       *CitationOptions `json:"citations,omitempty"`

	// Tool Use (assistant → user)
	ToolUseID *string         `json:"tool_use_id,omitempty"`
	ToolName  *string         `json:"tool_name,omitempty"`
	ToolInput json.RawMessage `json:"tool_input,omitempty"`

	// Tool Result (user → assistant)
	ToolResultID      *string         `json:"tool_result_id,omitempty"`
	ToolResultContent json.RawMessage `json:"tool_result_content,omitempty"`
	IsError           *bool           `json:"is_error,omitempty"`

	// Thinking/Reasoning
	Thinking          *string `json:"thinking,omitempty"`
	ThinkingSignature *string `json:"thinking_signature,omitempty"` // Anthropic extended thinking
	ReasoningContent  *string `json:"reasoning_content,omitempty"`  // DeepSeek

	// Redacted Thinking (Anthropic)
	RedactedThinkingData *string `json:"redacted_thinking_data,omitempty"`

	// Cache Control (Anthropic)
	CacheControl *CacheControl `json:"cache_control,omitempty"`

	// Server-side tool use metrics (Anthropic)
	CacheCreation            *CacheMetrics `json:"cache_creation,omitempty"`
	CacheReadInputTokens     *uint         `json:"cache_read_input_tokens,omitempty"`
	InputTokens              *uint         `json:"input_tokens,omitempty"`
	CacheCreationInputTokens *uint         `json:"cache_creation_input_tokens,omitempty"`

	// Function Response (Gemini)
	FunctionResponse json.RawMessage `json:"function_response,omitempty"`
}

// ImageSource represents an image in various formats
type ImageSource struct {
	Type        string  `json:"type"`                   // "base64", "url", "file"
	MediaType   string  `json:"media_type,omitempty"`   // "image/jpeg", "image/png", etc. (only for base64)
	Data        *string `json:"data,omitempty"`         // base64 encoded data
	URL         *string `json:"url,omitempty"`          // image URL
	FileID      *string `json:"file_id,omitempty"`      // file reference (Anthropic)
	FileURI     *string `json:"file_uri,omitempty"`     // file URI (Gemini)
	DisplayName *string `json:"display_name,omitempty"` // display name (Gemini)
}

// MarshalJSON customizes the JSON marshaling to omit media_type for url/file types
func (is ImageSource) MarshalJSON() ([]byte, error) {
	type Alias ImageSource
	if is.Type == "url" || is.Type == "file" {
		// For url and file types, omit media_type
		return json.Marshal(&struct {
			Type        string  `json:"type"`
			URL         *string `json:"url,omitempty"`
			FileID      *string `json:"file_id,omitempty"`
			FileURI     *string `json:"file_uri,omitempty"`
			DisplayName *string `json:"display_name,omitempty"`
		}{
			Type:        is.Type,
			URL:         is.URL,
			FileID:      is.FileID,
			FileURI:     is.FileURI,
			DisplayName: is.DisplayName,
		})
	}
	// For base64, include all fields
	return json.Marshal((Alias)(is))
}

// DocumentSource represents a document in various formats
type DocumentSource struct {
	Type      string         `json:"type"`                 // "base64", "url", "text", "content"
	MediaType string         `json:"media_type,omitempty"` // "application/pdf", "text/plain", etc.
	Data      *string        `json:"data,omitempty"`       // base64 encoded data
	URL       *string        `json:"url,omitempty"`        // document URL
	Text      *string        `json:"text,omitempty"`       // plain text content
	Content   []ContentBlock `json:"content,omitempty"`    // content blocks (for type="content")
}

// MarshalJSON customizes the JSON marshaling to omit media_type for url/content types
func (ds DocumentSource) MarshalJSON() ([]byte, error) {
	type Alias DocumentSource
	if ds.Type == "url" || ds.Type == "content" {
		// For url and content types, omit media_type
		return json.Marshal(&struct {
			Type    string         `json:"type"`
			URL     *string        `json:"url,omitempty"`
			Content []ContentBlock `json:"content,omitempty"`
		}{
			Type:    ds.Type,
			URL:     ds.URL,
			Content: ds.Content,
		})
	}
	// For base64 and text, include all fields
	return json.Marshal((Alias)(ds))
}

// CitationOptions represents citation configuration for documents (Anthropic)
type CitationOptions struct {
	Enabled bool `json:"enabled"`
}

// CacheMetrics represents cache creation metrics (Anthropic server-side tools)
type CacheMetrics struct {
	InputTokens *uint `json:"input_tokens,omitempty"`
}

// CacheControl represents prompt caching configuration (Anthropic)
type CacheControl struct {
	Type string `json:"type"`          // "ephemeral"
	TTL  string `json:"ttl,omitempty"` // time-to-live (e.g., "5m", "1h")
}

// ContentBlock Types
const (
	ContentTypeText        = "text"
	ContentTypeImage       = "image"
	ContentTypeDocument    = "document"
	ContentTypeToolUse     = "tool_use"
	ContentTypeToolResult  = "tool_result"
	ContentTypeThinking    = "thinking"
	ContentTypeRedacted    = "redacted_thinking"
	ContentTypeFunctionRes = "function_response"
)

////////////////////////////////////////////////////////////////////////////////
// CONSTRUCTORS

// NewMessage creates a new message with a single text content block
func NewMessage(role, text string, opt ...Opt) (*Message, error) {
	self := Message{
		Role: role,
		Content: []ContentBlock{
			{
				Type: ContentTypeText,
				Text: &text,
			},
		},
	}
	for _, o := range opt {
		if err := o(&self); err != nil {
			return nil, err
		}
	}
	return &self, nil
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Text returns the concatenated text content from all text blocks in the message
func (m Message) Text() string {
	var result []string
	for _, block := range m.Content {
		if block.Type == ContentTypeText && block.Text != nil {
			result = append(result, *block.Text)
		}
	}
	return strings.Join(result, "\n")
}

////////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (m Message) String() string {
	return Stringify(m)
}
