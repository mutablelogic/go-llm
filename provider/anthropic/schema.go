package anthropic

import (
	"encoding/json"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES - MODELS

// apiModel is a model, as GET /v1/models returns it: its identifier, its
// name for people, when it was created, its context window, the most tokens
// it replies with, and what it can do
type apiModel struct {
	ID             string          `json:"id"`
	DisplayName    string          `json:"display_name,omitempty"`
	CreatedAt      string          `json:"created_at,omitempty"`
	MaxInputTokens uint            `json:"max_input_tokens,omitempty"`
	MaxTokens      uint            `json:"max_tokens,omitempty"`
	Capabilities   json.RawMessage `json:"capabilities,omitempty"`
}

// modelList is a page of models, and whether there are more after the
// last of them
type modelList struct {
	Data    []apiModel `json:"data"`
	HasMore bool       `json:"has_more"`
	LastID  string     `json:"last_id,omitempty"`
}

// supported is a capability of a model, which it has, or hasn't
type supported struct {
	Supported bool `json:"supported"`
}

// modelCapabilities are those of a model's capabilities which the client
// uses: whether it understands images, and PDFs, replies with structured
// output, thinks, and how, and has effort
type modelCapabilities struct {
	ImageInput        supported `json:"image_input"`
	PDFInput          supported `json:"pdf_input"`
	StructuredOutputs supported `json:"structured_outputs"`
	Thinking          struct {
		supported
		Types struct {
			Adaptive supported `json:"adaptive"`
			Enabled  supported `json:"enabled"`
		} `json:"types"`
	} `json:"thinking"`
	Effort supported `json:"effort"`
}

///////////////////////////////////////////////////////////////////////////////
// TYPES - MESSAGES

// messagesRequest is a request to POST /v1/messages, or, without its
// options, to POST /v1/messages/count_tokens
type messagesRequest struct {
	Model         string          `json:"model"`
	MaxTokens     uint            `json:"max_tokens,omitempty"`
	System        string          `json:"system,omitempty"`
	Messages      []message       `json:"messages"`
	Tools         []tool          `json:"tools,omitempty"`
	ToolChoice    *toolChoice     `json:"tool_choice,omitempty"`
	Thinking      *thinking       `json:"thinking,omitempty"`
	OutputConfig  *outputConfig   `json:"output_config,omitempty"`
	StopSequences []string        `json:"stop_sequences,omitempty"`
	Metadata      *metadata       `json:"metadata,omitempty"`
	ServiceTier   string          `json:"service_tier,omitempty"`
	CacheControl  *cacheControl   `json:"cache_control,omitempty"`
	Stream        bool            `json:"stream,omitempty"`
	beta          map[string]bool // The betas the request needs, which are sent as a header
}

// message is a message of a conversation, from the user, which has the
// results of tools, or the assistant
type message struct {
	Role    string         `json:"role"`
	Content []contentBlock `json:"content"`
}

// contentBlock is a part of a message: its text, its thinking, with its
// signature, an image or a document, a tool the model called, or a tool's
// result
type contentBlock struct {
	Type string `json:"type"`

	// Text
	Text string `json:"text,omitempty"`

	// Thinking
	Thinking  *string `json:"thinking,omitempty"`
	Signature string  `json:"signature,omitempty"`

	// Image, or document
	Source *source `json:"source,omitempty"`

	// Tool use
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`

	// Tool result
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
	IsError   bool   `json:"is_error,omitempty"`
}

// source is the data of an image, or a document, as base64, or as text
type source struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

// tool is a tool which the model can call, with the JSON schema of its input
type tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// toolChoice is whether the model calls the tools
type toolChoice struct {
	Type string `json:"type"`
}

// thinking is whether the model thinks, and how: adaptively, as the model
// decides, with what's shown of it, or within a budget of tokens, for older
// models, and whether thinking which was sent back, which is bound to the
// conversation it was in, is dropped, rather than fails the request, when
// the conversation was changed
type thinking struct {
	Type         string        `json:"type"`
	Display      string        `json:"display,omitempty"`
	BudgetTokens uint          `json:"budget_tokens,omitempty"`
	BlockBinding *blockBinding `json:"block_binding,omitempty"`
}

// blockBinding is what's done with thinking which was sent back, when the
// conversation before it changed
type blockBinding struct {
	PrefixMismatchBehavior string `json:"prefix_mismatch_behavior"`
}

// outputConfig is how much effort the model makes, and the format of its
// reply
type outputConfig struct {
	Effort string        `json:"effort,omitempty"`
	Format *outputFormat `json:"format,omitempty"`
}

// outputFormat is the JSON schema of the reply
type outputFormat struct {
	Type   string          `json:"type"`
	Schema json.RawMessage `json:"schema"`
}

// metadata identifies the end user
type metadata struct {
	UserID string `json:"user_id,omitempty"`
}

// cacheControl caches the request, up to its last block which can be
// cached, so the next request with the same start reads it from the cache
type cacheControl struct {
	Type string `json:"type"`
}

// messagesResponse is the reply from POST /v1/messages: its content, why
// the model stopped, and the tokens used
type messagesResponse struct {
	ID         string         `json:"id"`
	Model      string         `json:"model"`
	Role       string         `json:"role"`
	Content    []contentBlock `json:"content"`
	StopReason string         `json:"stop_reason,omitempty"`
	Usage      usage          `json:"usage"`
}

// usage is the tokens of a request, and its reply: the input tokens, which
// don't include those read from, or written to, the cache, and the output
// tokens
type usage struct {
	InputTokens              uint `json:"input_tokens"`
	OutputTokens             uint `json:"output_tokens"`
	CacheReadInputTokens     uint `json:"cache_read_input_tokens,omitempty"`
	CacheCreationInputTokens uint `json:"cache_creation_input_tokens,omitempty"`
}

// countResponse is the reply from POST /v1/messages/count_tokens
type countResponse struct {
	InputTokens uint `json:"input_tokens"`
}

///////////////////////////////////////////////////////////////////////////////
// TYPES - STREAMING

// streamEvent is an event of a streamed reply: the message as it starts, a
// content block as it starts, a piece of it, and as it stops, why the model
// stopped, with the tokens, and an error
type streamEvent struct {
	Type         string            `json:"type"`
	Message      *messagesResponse `json:"message,omitempty"`
	Index        int               `json:"index"`
	ContentBlock *contentBlock     `json:"content_block,omitempty"`
	Delta        *streamDelta      `json:"delta,omitempty"`
	Usage        *usage            `json:"usage,omitempty"`
	Error        *apiError         `json:"error,omitempty"`
}

// streamDelta is a piece of a content block, or of the message, as it ends
type streamDelta struct {
	Type        string `json:"type,omitempty"`
	Text        string `json:"text,omitempty"`
	Thinking    string `json:"thinking,omitempty"`
	Signature   string `json:"signature,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`
	StopReason  string `json:"stop_reason,omitempty"`
}

// apiError is an error in a streamed reply
type apiError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}
