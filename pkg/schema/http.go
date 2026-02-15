package schema

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	// Packages
	gomultipart "github.com/mutablelogic/go-client/pkg/multipart"
	types "github.com/mutablelogic/go-server/pkg/types"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	EmbeddingTaskTypeDefault = "DEFAULT"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// ListModelsRequest represents a request to list models
type ListModelsRequest struct {
	Provider string `json:"provider,omitempty" help:"Filter by provider name" optional:""`
	Limit    *uint  `json:"limit,omitempty" help:"Maximum number of models to return"`
	Offset   uint   `json:"offset,omitempty" help:"Offset for pagination"`
}

// ListModelsResponse represents a response containing a list of models and providers
type ListModelsResponse struct {
	Count    uint     `json:"count"`
	Offset   uint     `json:"offset,omitzero"`
	Limit    *uint    `json:"limit,omitzero"`
	Provider []string `json:"provider,omitempty"`
	Body     []Model  `json:"body,omitzero"`
}

// GetModelRequest represents a request to get a model
type GetModelRequest struct {
	Provider string `json:"provider,omitempty" help:"Filter by provider name" optional:""`
	Name     string `json:"name,omitempty" help:"Model name"`
}

// EmbeddingRequest represents a request to embed text
type EmbeddingRequest struct {
	Provider             string   `json:"provider,omitempty" help:"Provider name" optional:""`
	Model                string   `json:"model,omitempty" help:"Model name" optional:""`
	Input                []string `json:"input,omitempty" arg:"" help:"Text inputs to embed"`
	TaskType             string   `json:"task_type,omitempty" help:"Embedding task type (Google-specific)" enum:"DEFAULT,RETRIEVAL_QUERY,RETRIEVAL_DOCUMENT,SEMANTIC_SIMILARITY,CLASSIFICATION,CLUSTERING,QUESTION_ANSWERING,FACT_VERIFICATION,CODE_RETRIEVAL_QUERY," default:"DEFAULT"`
	Title                string   `json:"title,omitempty" help:"Document title, used with RETRIEVAL_DOCUMENT task type (Google-specific)"`
	OutputDimensionality uint     `json:"output_dimensionality,omitempty" help:"Truncate embedding to this many dimensions (Google-specific)"`
}

// EmbeddingResponse represents a response from an embedding request
type EmbeddingResponse struct {
	EmbeddingRequest
	Output [][]float64 `json:"output,omitempty"`
}

// CompletionResponse represents a response from a completion request.
type CompletionResponse struct {
	Role    string         `json:"role"`
	Content []ContentBlock `json:"content"`
	Result  ResultType     `json:"result"`
}

// StreamDelta represents a single streamed text chunk in an SSE stream.
type StreamDelta struct {
	Role string `json:"role"`
	Text string `json:"text"`
}

// StreamError represents an error event in an SSE stream.
type StreamError struct {
	Error string `json:"error"`
}

// GeneratorMeta represents the metadata needed to invoke a generator model.
type GeneratorMeta struct {
	Provider       string          `json:"provider,omitempty" help:"Provider name" optional:""`
	Model          string          `json:"model,omitempty" help:"Model name" optional:""`
	SystemPrompt   string          `json:"system_prompt,omitempty" help:"System prompt" optional:""`
	Format         json.RawMessage `json:"format,omitempty" help:"JSON schema for structured output" optional:""`
	Thinking       *bool           `json:"thinking,omitempty" help:"Enable thinking/reasoning" optional:""`
	ThinkingBudget uint            `json:"thinking_budget,omitempty" help:"Thinking token budget (required for Anthropic, optional for Google)" optional:""`
}

// SessionMeta represents the metadata for a session.
type SessionMeta struct {
	GeneratorMeta
	Name   string            `json:"name,omitempty" help:"Session name" optional:""`
	Labels map[string]string `json:"labels,omitempty" help:"User-defined labels for UI storage" optional:""`
}

// AskRequest represents a stateless request to generate content.
type AskRequest struct {
	GeneratorMeta
	Text        string       `json:"text" arg:"" help:"User input text"`
	Attachments []Attachment `json:"attachments,omitempty" help:"File attachments" optional:""`
}

// MultipartAskRequest is the HTTP-layer request type supporting both JSON
// (with base64 attachments) and multipart/form-data file uploads.
type MultipartAskRequest struct {
	AskRequest
	File gomultipart.File `json:"file,omitempty" help:"File attachment (multipart upload)" optional:""`
}

// AskResponse represents the response from an ask request.
type AskResponse struct {
	CompletionResponse
	Usage *Usage `json:"usage,omitempty"`
}

// ChatRequest represents a stateful chat request within a session.
type ChatRequest struct {
	Session       string       `json:"session" help:"Session ID"`
	Text          string       `json:"text" arg:"" help:"User input text"`
	Attachments   []Attachment `json:"attachments,omitempty" help:"File attachments" optional:""`
	Tools         []string     `json:"tools,omitempty" help:"Tool names to include (empty means all)" optional:""`
	MaxIterations uint         `json:"max_iterations,omitempty" help:"Maximum tool-calling iterations (0 uses default)" optional:""`
}

// MultipartChatRequest is the HTTP-layer request type supporting both JSON
// (with base64 attachments) and multipart/form-data file uploads for chat.
type MultipartChatRequest struct {
	ChatRequest
	File gomultipart.File `json:"file,omitempty" help:"File attachment (multipart upload)" optional:""`
}

// ChatResponse represents the response from a chat request.
type ChatResponse struct {
	CompletionResponse
	Session string `json:"session"`
	Usage   *Usage `json:"usage,omitempty"`
}

// ListSessionRequest represents a request to list sessions
type ListSessionRequest struct {
	Limit  *uint    `json:"limit,omitempty" help:"Maximum number of sessions to return"`
	Offset uint     `json:"offset,omitempty" help:"Offset for pagination"`
	Label  []string `json:"label,omitempty" help:"Filter by labels (key:value)"`
}

// ListSessionResponse represents a response containing a list of sessions
type ListSessionResponse struct {
	Count  uint       `json:"count"`
	Offset uint       `json:"offset,omitzero"`
	Limit  *uint      `json:"limit,omitzero"`
	Body   []*Session `json:"body,omitzero"`
}

// ListToolRequest represents a request to list tools
type ListToolRequest struct {
	Limit  *uint `json:"limit,omitempty" help:"Maximum number of tools to return"`
	Offset uint  `json:"offset,omitempty" help:"Offset for pagination"`
}

// ListToolResponse represents a response containing a list of tools
type ListToolResponse struct {
	Count  uint       `json:"count"`
	Offset uint       `json:"offset,omitzero"`
	Limit  *uint      `json:"limit,omitzero"`
	Body   []ToolMeta `json:"body,omitzero"`
}

// ToolMeta represents a tool's metadata
type ToolMeta struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Schema      json.RawMessage `json:"schema,omitempty"`
}

////////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (r ListModelsRequest) String() string {
	return types.Stringify(r)
}

func (r ListModelsResponse) String() string {
	return types.Stringify(r)
}

func (r GetModelRequest) String() string {
	return types.Stringify(r)
}

func (r EmbeddingRequest) String() string {
	return types.Stringify(r)
}

func (r EmbeddingResponse) String() string {
	return types.Stringify(r)
}

func (r GeneratorMeta) String() string {
	return types.Stringify(r)
}

func (r SessionMeta) String() string {
	return types.Stringify(r)
}

func (r ListSessionRequest) String() string {
	return types.Stringify(r)
}

func (r ListSessionResponse) String() string {
	return types.Stringify(r)
}

func (r ToolMeta) String() string {
	return types.Stringify(r)
}

func (r ListToolRequest) String() string {
	return types.Stringify(r)
}

func (r ListToolResponse) String() string {
	return types.Stringify(r)
}

func (r CompletionResponse) String() string {
	return types.Stringify(r)
}

func (r AskRequest) String() string {
	return types.Stringify(r)
}

func (r AskResponse) String() string {
	return types.Stringify(r)
}

func (r ChatRequest) String() string {
	return types.Stringify(r)
}

func (r ChatResponse) String() string {
	return types.Stringify(r)
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// FileAttachment reads the multipart file (if present) and returns it
// as an Attachment with auto-detected MIME type. Returns nil if no file
// was uploaded.
func (r *MultipartAskRequest) FileAttachment() (*Attachment, error) {
	return fileAttachment(r.File)
}

// FileAttachment reads the multipart file (if present) and returns it
// as an Attachment with auto-detected MIME type. Returns nil if no file
// was uploaded.
func (r *MultipartChatRequest) FileAttachment() (*Attachment, error) {
	return fileAttachment(r.File)
}

func fileAttachment(f gomultipart.File) (*Attachment, error) {
	if f.Body == nil {
		return nil, nil
	}
	data, err := io.ReadAll(f.Body)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}
	a := &Attachment{
		Type: http.DetectContentType(data),
		Data: data,
	}
	if f.Path != "" {
		a.URL = types.Ptr(url.URL{Scheme: "file", Path: f.Path})
	}
	return a, nil
}
