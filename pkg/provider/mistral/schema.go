package mistral

import (
	"encoding/json"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES - Mistral REST API wire format
//
// Reference: https://docs.mistral.ai/api/#tag/chat/operation/chat_completion_v1_chat_completions_post
//            https://docs.mistral.ai/api/#tag/models

///////////////////////////////////////////////////////////////////////////////
// CHAT COMPLETIONS — REQUEST

// chatCompletionRequest is the request body for POST /v1/chat/completions.
type chatCompletionRequest struct {
	Model            string           `json:"model"`
	Messages         []mistralMessage `json:"messages"`
	Temperature      *float64         `json:"temperature,omitempty"`
	TopP             *float64         `json:"top_p,omitempty"`
	MaxTokens        *int             `json:"max_tokens,omitempty"`
	Stream           bool             `json:"stream,omitempty"`
	Stop             []string         `json:"stop,omitempty"`
	RandomSeed       *uint            `json:"random_seed,omitempty"`
	Tools            []toolDefinition `json:"tools,omitempty"`
	ToolChoice       any              `json:"tool_choice,omitempty"`
	ResponseFormat   *responseFormat  `json:"response_format,omitempty"`
	PresencePenalty  *float64         `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float64         `json:"frequency_penalty,omitempty"`
	NumChoices       *int             `json:"n,omitempty"`
	SafePrompt       bool             `json:"safe_prompt,omitempty"`
}

///////////////////////////////////////////////////////////////////////////////
// CHAT COMPLETIONS — RESPONSE

// chatCompletionResponse is the response body from POST /v1/chat/completions
// (non-streaming).
type chatCompletionResponse struct {
	Id      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []chatChoice `json:"choices"`
	Usage   chatUsage    `json:"usage"`
}

// chatChoice is one element of the choices array.
type chatChoice struct {
	Index        int            `json:"index"`
	Message      mistralMessage `json:"message"`
	FinishReason string         `json:"finish_reason"`
}

// chatUsage reports token counts for a chat completion request.
type chatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

///////////////////////////////////////////////////////////////////////////////
// MESSAGES

// mistralMessage represents a single turn in a conversation.
// For user/system roles the Content field is typically a plain string;
// for assistant roles it may include ToolCalls. Tool-result messages
// carry the ToolCallID to correlate with the original call.
type mistralMessage struct {
	Role       string            `json:"role"`
	Content    any               `json:"content,omitempty"`      // string or []contentPart; nil omitted for tool-call-only assistant messages
	ToolCalls  []mistralToolCall `json:"tool_calls,omitempty"`   // assistant only
	ToolCallID string            `json:"tool_call_id,omitempty"` // tool role only
}

// contentPart represents one element in a multi-part content array
// (used for vision / multi-modal input).
type contentPart struct {
	Type     string    `json:"type"`                // "text" or "image_url"
	Text     string    `json:"text,omitempty"`      // for type "text"
	ImageURL *imageURL `json:"image_url,omitempty"` // for type "image_url"
}

// imageURL carries the URL (or data-URI) for an image content part.
type imageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"` // "auto", "low", "high"
}

///////////////////////////////////////////////////////////////////////////////
// TOOL CALLS

// mistralToolCall represents a tool invocation in an assistant message.
type mistralToolCall struct {
	Id       string          `json:"id"`
	Type     string          `json:"type"` // always "function"
	Function mistralFunction `json:"function"`
}

// mistralFunction carries the function name and JSON-encoded arguments
// within a tool call.
type mistralFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

///////////////////////////////////////////////////////////////////////////////
// TOOL DEFINITIONS

// toolDefinition describes a tool the model may call.
type toolDefinition struct {
	Type     string          `json:"type"` // always "function"
	Function toolFunctionDef `json:"function"`
}

// toolFunctionDef describes the function signature for a tool definition.
type toolFunctionDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"` // JSON Schema
}

///////////////////////////////////////////////////////////////////////////////
// RESPONSE FORMAT

// responseFormat constrains the model output format.
type responseFormat struct {
	Type       string          `json:"type"`                  // "text", "json_object", "json_schema"
	JSONSchema json.RawMessage `json:"json_schema,omitempty"` // for type "json_schema"
}

///////////////////////////////////////////////////////////////////////////////
// STREAMING

// chatCompletionChunk is a single SSE event for a streaming chat completion.
// The stream is terminated by a `data: [DONE]` sentinel.
type chatCompletionChunk struct {
	Id      string        `json:"id"`
	Object  string        `json:"object"`
	Created int64         `json:"created"`
	Model   string        `json:"model"`
	Choices []chunkChoice `json:"choices"`
	Usage   *chatUsage    `json:"usage,omitempty"` // present in the final chunk
}

// chunkChoice carries the incremental delta for one choice.
type chunkChoice struct {
	Index        int        `json:"index"`
	Delta        chunkDelta `json:"delta"`
	FinishReason string     `json:"finish_reason,omitempty"`
}

// chunkDelta is the incremental content within a streaming chunk.
type chunkDelta struct {
	Role      string            `json:"role,omitempty"`
	Content   string            `json:"content,omitempty"`
	ToolCalls []mistralToolCall `json:"tool_calls,omitempty"`
}

///////////////////////////////////////////////////////////////////////////////
// FINISH REASON CONSTANTS

const (
	finishReasonStop        = "stop"
	finishReasonToolCalls   = "tool_calls"
	finishReasonLength      = "length"
	finishReasonModelLength = "model_length"
	finishReasonError       = "error"
)

///////////////////////////////////////////////////////////////////////////////
// ROLE CONSTANTS

const (
	roleSystem    = "system"
	roleUser      = "user"
	roleAssistant = "assistant"
	roleTool      = "tool"
)

///////////////////////////////////////////////////////////////////////////////
// TOOL CHOICE CONSTANTS

const (
	toolChoiceAuto     = "auto"
	toolChoiceNone     = "none"
	toolChoiceAny      = "any"
	toolChoiceRequired = "required"
)

///////////////////////////////////////////////////////////////////////////////
// RESPONSE FORMAT CONSTANTS

const (
	responseFormatText       = "text"
	responseFormatJSONObject = "json_object"
	responseFormatJSONSchema = "json_schema"
)

///////////////////////////////////////////////////////////////////////////////
// DEFAULTS

const (
	defaultMaxTokens = 1024
)

///////////////////////////////////////////////////////////////////////////////
// EMBEDDINGS — REQUEST

// embeddingsRequest is the request body for POST /v1/embeddings.
type embeddingsRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

///////////////////////////////////////////////////////////////////////////////
// EMBEDDINGS — RESPONSE

// embeddingsResponse is the response body from POST /v1/embeddings.
type embeddingsResponse struct {
	Id    string           `json:"id"`
	Model string           `json:"model"`
	Data  []embeddingEntry `json:"data"`
	Usage embeddingsUsage  `json:"usage"`
}

// embeddingEntry is one element of the data array.
type embeddingEntry struct {
	Index     int       `json:"index"`
	Embedding []float64 `json:"embedding"`
}

// embeddingsUsage reports token counts for an embeddings request.
type embeddingsUsage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}
