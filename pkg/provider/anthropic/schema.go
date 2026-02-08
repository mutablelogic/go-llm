package anthropic

import (
	"encoding/json"
	"time"

	// Packages
	"github.com/google/jsonschema-go/jsonschema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES - Anthropic REST API wire format
//
// Reference: https://docs.anthropic.com/en/api/messages
//            https://docs.anthropic.com/en/api/models
//            https://docs.anthropic.com/en/api/streaming

///////////////////////////////////////////////////////////////////////////////
// MESSAGES — REQUEST

// messagesRequest is the request body for POST /v1/messages.
type messagesRequest struct {
	MaxTokens     int                `json:"max_tokens"`
	Messages      []anthropicMessage `json:"messages"`
	Metadata      *messagesMetadata  `json:"metadata,omitempty"`
	Model         string             `json:"model"`
	OutputConfig  string             `json:"output_config,omitempty"`
	OutputFormat  *outputFormat      `json:"output_format,omitempty"`
	ServiceTier   string             `json:"service_tier,omitempty"`
	StopSequences []string           `json:"stop_sequences,omitempty"`
	Stream        bool               `json:"stream,omitempty"`
	System        any                `json:"system,omitempty"`
	Temperature   *float64           `json:"temperature,omitempty"`
	Thinking      *thinkingConfig    `json:"thinking,omitempty"`
	ToolChoice    *toolChoice        `json:"tool_choice,omitempty"`
	Tools         []json.RawMessage  `json:"tools,omitempty"`
	TopK          *uint              `json:"top_k,omitempty"`
	TopP          *float64           `json:"top_p,omitempty"`
}

// messagesMetadata carries request-level metadata.
type messagesMetadata struct {
	UserId string `json:"user_id,omitempty"`
}

// thinkingConfig controls extended thinking (chain-of-thought).
type thinkingConfig struct {
	Type         string `json:"type"`
	BudgetTokens uint   `json:"budget_tokens,omitempty"`
}

// toolChoice specifies which tool(s) the model may use.
type toolChoice struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}

// outputFormat constrains the response format (e.g. JSON schema).
type outputFormat struct {
	Type       string             `json:"type"`
	JSONSchema *jsonschema.Schema `json:"json_schema,omitempty"`
}

///////////////////////////////////////////////////////////////////////////////
// MESSAGES — RESPONSE

// messagesResponse is the response body from POST /v1/messages (non-streaming)
// and the payload of the message_start SSE event.
type messagesResponse struct {
	Id           string                  `json:"id"`
	Model        string                  `json:"model"`
	Type         string                  `json:"type"`
	Role         string                  `json:"role"`
	Content      []anthropicContentBlock `json:"content"`
	StopReason   string                  `json:"stop_reason"`
	StopSequence *string                 `json:"stop_sequence,omitempty"`
	Usage        messagesUsage           `json:"usage"`
}

// messagesUsage reports token counts for a messages request.
type messagesUsage struct {
	InputTokens  uint   `json:"input_tokens"`
	OutputTokens uint   `json:"output_tokens"`
	ServiceTier  string `json:"service_tier"`
}

///////////////////////////////////////////////////////////////////////////////
// CONTENT — MESSAGES & BLOCKS

// anthropicMessage represents a single turn in a conversation.
type anthropicMessage struct {
	Role    string                  `json:"role"`
	Content []anthropicContentBlock `json:"content"`
}

// anthropicContentBlock represents a content block in Anthropic's API format.
// Different block types use different subsets of fields.
type anthropicContentBlock struct {
	Type string `json:"type"`

	// text block
	Text string `json:"text,omitempty"`

	// image/document source
	Source *anthropicSource `json:"source,omitempty"`

	// tool_use block
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`

	// tool_result block
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   json.RawMessage `json:"content,omitempty"` // tool result payload
	IsError   bool            `json:"is_error,omitempty"`

	// thinking block
	Thinking  string `json:"thinking,omitempty"`
	Signature string `json:"signature,omitempty"`
}

// anthropicSource represents a media source (base64 or URL).
type anthropicSource struct {
	Type      string `json:"type"`                 // "base64" or "url"
	MediaType string `json:"media_type,omitempty"` // MIME type for base64
	Data      string `json:"data,omitempty"`       // base64-encoded data
	URL       string `json:"url,omitempty"`        // URL reference
}

///////////////////////////////////////////////////////////////////////////////
// SYSTEM PROMPT

// textBlockParam is used for system prompts with optional cache control.
type textBlockParam struct {
	Type         string                 `json:"type"`
	Text         string                 `json:"text"`
	CacheControl *cacheControlEphemeral `json:"cache_control,omitempty"`
}

// cacheControlEphemeral marks a block for prompt caching.
type cacheControlEphemeral struct {
	Type string `json:"type"`
}

///////////////////////////////////////////////////////////////////////////////
// STREAMING

// streamEvent is the envelope for all SSE events from the Anthropic streaming
// API. Different event types populate different subsets of fields.
type streamEvent struct {
	Type         string                 `json:"type"`
	Index        int                    `json:"index,omitempty"`
	Message      *messagesResponse      `json:"message,omitempty"`
	ContentBlock *anthropicContentBlock `json:"content_block,omitempty"`
	Delta        *streamDelta           `json:"delta,omitempty"`
	Usage        *messagesUsage         `json:"usage,omitempty"`
}

// streamDelta carries the incremental content within content_block_delta
// and message_delta events.
type streamDelta struct {
	Type         string `json:"type"`
	Text         string `json:"text,omitempty"`
	PartialJSON  string `json:"partial_json,omitempty"`
	Thinking     string `json:"thinking,omitempty"`
	Signature    string `json:"signature,omitempty"`
	StopReason   string `json:"stop_reason,omitempty"`
	StopSequence string `json:"stop_sequence,omitempty"`
}

///////////////////////////////////////////////////////////////////////////////
// MODELS — GET & LIST

// model represents the API response for GET /v1/models/{model_id}
// and each entry in the list response.
type model struct {
	Id          string    `json:"id"`
	DisplayName string    `json:"display_name"`
	Type        string    `json:"type"`
	CreatedAt   time.Time `json:"created_at"`
}

// listModelsResponse is the paginated response from GET /v1/models.
type listModelsResponse struct {
	Data    []model `json:"data"`
	HasMore bool    `json:"has_more"`
	FirstId string  `json:"first_id"`
	LastId  string  `json:"last_id"`
}

///////////////////////////////////////////////////////////////////////////////
// STOP REASON CONSTANTS

const (
	stopReasonEndTurn      = "end_turn"
	stopReasonMaxTokens    = "max_tokens"
	stopReasonStopSequence = "stop_sequence"
	stopReasonToolUse      = "tool_use"
	stopReasonPauseTurn    = "pause_turn"
	stopReasonRefusal      = "refusal"
)

///////////////////////////////////////////////////////////////////////////////
// DEFAULTS

const (
	defaultMaxTokens = 1024
)

///////////////////////////////////////////////////////////////////////////////
// STREAM EVENT TYPE CONSTANTS

const (
	eventMessageStart      = "message_start"
	eventContentBlockStart = "content_block_start"
	eventContentBlockDelta = "content_block_delta"
	eventContentBlockStop  = "content_block_stop"
	eventMessageDelta      = "message_delta"
	eventMessageStop       = "message_stop"
	eventPing              = "ping"
	eventError             = "error"
)

///////////////////////////////////////////////////////////////////////////////
// CONTENT BLOCK TYPE CONSTANTS

const (
	blockTypeText       = "text"
	blockTypeImage      = "image"
	blockTypeDocument   = "document"
	blockTypeToolUse    = "tool_use"
	blockTypeToolResult = "tool_result"
	blockTypeThinking   = "thinking"
)

///////////////////////////////////////////////////////////////////////////////
// DELTA TYPE CONSTANTS

const (
	deltaTypeText      = "text_delta"
	deltaTypeThinking  = "thinking_delta"
	deltaTypeSignature = "signature_delta"
	deltaTypeInputJSON = "input_json_delta"
)

///////////////////////////////////////////////////////////////////////////////
// SOURCE TYPE CONSTANTS

const (
	sourceTypeBase64 = "base64"
	sourceTypeURL    = "url"
)
