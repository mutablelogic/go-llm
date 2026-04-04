package schema

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	// Packages
	"github.com/mutablelogic/go-pg"
	types "github.com/mutablelogic/go-server/pkg/types"
)

////////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	EmbeddingTaskTypeDefault = "DEFAULT"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// ModelListRequest represents a request to list models
type ModelListRequest struct {
	pg.OffsetLimit
	Provider string `json:"provider,omitempty" help:"Filter by provider name" optional:""`
}

// ModelList represents a response containing a list of models and providers
type ModelList struct {
	ModelListRequest
	Provider []string `json:"provider,omitempty"`
	Count    uint     `json:"count"`
	Body     []Model  `json:"body,omitzero"`
}

// ModelNameSelector selects a model by name for path-based GET operations.
type ModelNameSelector struct {
	Name string `json:"name" help:"Model name"`
}

// ModelProviderSelector selects a model by provider and name for path-based GET operations.
type ModelProviderSelector struct {
	Provider string `json:"provider" help:"Provider name"`
	Name     string `json:"name" help:"Model name"`
}

// GetModelRequest represents a request to get a model
type GetModelRequest struct {
	Provider string `json:"provider,omitempty" help:"Filter by provider name" optional:""`
	Name     string `json:"name,omitempty" help:"Model name"`
}

// DownloadModelRequest represents a request to download a model
type DownloadModelRequest struct {
	Provider string `json:"provider,omitempty" help:"Provider name" optional:""`
	Name     string `json:"name" help:"Model name to download"`
}

// DeleteModelRequest represents a request to delete a model
type DeleteModelRequest struct {
	Provider string `json:"provider,omitempty" help:"Provider name" optional:""`
	Name     string `json:"name" help:"Model name to delete"`
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
	Role    string         `json:"role" help:"Role of the generated response, typically assistant" example:"assistant"`
	Content []ContentBlock `json:"content" help:"Structured response content blocks returned by the model" example:"[{\"text\":\"Unit tests catch regressions early and make refactoring safer.\"}]"`
	Result  ResultType     `json:"result" help:"Completion result status" example:"\"stop\""`
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
	Provider       string     `json:"provider,omitempty" yaml:"provider" help:"Provider name" optional:"" example:"ollama"`
	Model          string     `json:"model,omitempty" yaml:"model" help:"Model name" optional:"" example:"llama3.2"`
	SystemPrompt   string     `json:"system_prompt,omitempty" yaml:"system_prompt" help:"System prompt" optional:"" example:"Be concise and answer in one sentence."`
	Format         JSONSchema `json:"format,omitempty" yaml:"output" help:"JSON schema for structured output" optional:"" example:"{\"type\":\"object\",\"properties\":{\"summary\":{\"type\":\"string\"}}}"`
	Thinking       *bool      `json:"thinking,omitempty" yaml:"thinking" help:"Enable thinking/reasoning" optional:"" example:"true"`
	ThinkingBudget uint       `json:"thinking_budget,omitempty" yaml:"thinking_budget" help:"Thinking token budget (required for Anthropic, optional for Google)" optional:"" example:"2048"`
}

// SessionMeta represents the metadata for a session.
type SessionMeta struct {
	GeneratorMeta
	Name   string            `json:"name,omitempty" help:"Session name" optional:""`
	Labels map[string]string `json:"labels,omitempty" help:"User-defined labels for UI storage" optional:""`
}

// AskRequestCore contains the core fields of an ask request without attachments.
type AskRequestCore struct {
	GeneratorMeta
	Text string `json:"text" arg:"" help:"User input text" example:"Summarize the benefits of unit testing in one sentence."`
}

// AskRequest represents a stateless request to generate content.
type AskRequest struct {
	AskRequestCore
	Attachments []Attachment `json:"attachments,omitempty" help:"File attachments" optional:"" example:"[{\"type\":\"image/png\",\"url\":\"https://example.com/image.png\"}]"`
}

// MultipartAskRequest is the HTTP-layer request type supporting both JSON
// (with base64 attachments) and multipart/form-data file uploads.
type MultipartAskRequest struct {
	AskRequest
	File types.File `json:"file,omitempty" help:"File attachment (multipart upload)" optional:""`
}

// AskResponse represents the response from an ask request.
type AskResponse struct {
	CompletionResponse
	Usage *Usage `json:"usage,omitempty" help:"Token usage information for the request, when available" example:"{\"input_tokens\":18,\"output_tokens\":12}"`
}

// ChatRequestCore contains the core fields of a chat request without attachments.
type ChatRequestCore struct {
	Session       string   `json:"session" help:"Session ID"`
	Text          string   `json:"text" arg:"" help:"User input text"`
	Tools         []string `json:"tools,omitzero" help:"Tool names to include (nil means all, empty means none)" optional:""`
	MaxIterations uint     `json:"max_iterations,omitempty" help:"Maximum tool-calling iterations (0 uses default)" optional:""`
	SystemPrompt  string   `json:"system_prompt,omitempty" help:"Per-request system prompt appended to the session prompt" optional:""`
}

// ChatRequest represents a stateful chat request within a session.
type ChatRequest struct {
	ChatRequestCore
	Attachments []Attachment `json:"attachments,omitempty" help:"File attachments" optional:""`
}

// MultipartChatRequest is the HTTP-layer request type supporting both JSON
// (with base64 attachments) and multipart/form-data file uploads for chat.
type MultipartChatRequest struct {
	ChatRequest
	File types.File `json:"file,omitempty" help:"File attachment (multipart upload)" optional:""`
}

// ChatResponse represents the response from a chat request.
type ChatResponse struct {
	CompletionResponse
	Session string `json:"session"`
	Usage   *Usage `json:"usage,omitempty"`
}

// CreateAgentSessionRequest represents the body of a request to create a
// session from an agent definition. The agent is identified by path/query
// parameters (agent ID or name, optional version) — not included here.
// The agent's template is executed with Input, and a new session is created
// with the merged GeneratorMeta and agent labels. If Parent is set, the parent
// session's GeneratorMeta is used as defaults (agent fields take precedence).
// The caller can then use the returned text and tools with the Chat endpoint.
type CreateAgentSessionRequest struct {
	Parent string          `json:"parent,omitempty" help:"Parent session ID for traceability" optional:""`
	Input  json.RawMessage `json:"input,omitempty" help:"Input data for the agent template" optional:""`
}

// CreateAgentSessionResponse is the result of creating a session from an agent.
// It contains the session ID plus the prepared text and tools, which can be
// passed directly to a ChatRequest.
type CreateAgentSessionResponse struct {
	Session string   `json:"session"`         // Created session ID
	Text    string   `json:"text"`            // Rendered template text (first user message)
	Tools   []string `json:"tools,omitempty"` // Tool names the agent is allowed to use
}

// ListAgentRequest represents a request to list agents
type ListAgentRequest struct {
	Name    string `json:"name,omitempty" help:"Filter by agent name" optional:""`
	Version *uint  `json:"version,omitempty" help:"Filter by version number (requires name)" optional:""`
	Limit   *uint  `json:"limit,omitempty" help:"Maximum number of agents to return" default:"100"`
	Offset  uint   `json:"offset,omitempty" help:"Offset for pagination" default:"0"`
}

// ListAgentResponse represents a response containing a list of agents
type ListAgentResponse struct {
	Count  uint     `json:"count"`
	Offset uint     `json:"offset,omitzero"`
	Limit  *uint    `json:"limit,omitzero"`
	Body   []*Agent `json:"body,omitzero"`
}

// ListSessionRequest represents a request to list sessions
type ListSessionRequest struct {
	Limit  *uint    `json:"limit,omitempty" help:"Maximum number of sessions to return" default:"100"`
	Offset uint     `json:"offset,omitempty" help:"Offset for pagination" default:"0"`
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
	Limit  *uint `json:"limit,omitempty" help:"Maximum number of tools to return" default:"100"`
	Offset uint  `json:"offset,omitempty" help:"Offset for pagination" default:"0"`
}

// ListToolResponse represents a response containing a list of tools
type ListToolResponse struct {
	Count  uint       `json:"count"`
	Offset uint       `json:"offset,omitzero"`
	Limit  *uint      `json:"limit,omitzero"`
	Body   []ToolMeta `json:"body,omitzero"`
}

// ListConnectorsRequest represents a request to list registered MCP connectors.
type ListConnectorsRequest struct {
	Namespace string `json:"namespace,omitempty" help:"Filter by namespace" optional:""`
	Enabled   *bool  `json:"enabled,omitempty" help:"Filter by enabled state" optional:""`
	Limit     *uint  `json:"limit,omitempty" help:"Maximum number of connectors to return" default:"100"`
	Offset    uint   `json:"offset,omitempty" help:"Offset for pagination" default:"0"`
}

// ListConnectorsResponse represents a response containing a list of MCP connectors.
type ListConnectorsResponse struct {
	Count  uint         `json:"count"`
	Offset uint         `json:"offset,omitzero"`
	Limit  *uint        `json:"limit,omitzero"`
	Body   []*Connector `json:"body,omitzero"`
}

// ToolMeta represents a tool's metadata
type ToolMeta struct {
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	Input       JSONSchema `json:"input,omitempty"`
}

// CallToolRequest represents a request to call a tool directly
type CallToolRequest struct {
	Input json.RawMessage `json:"input,omitempty"`
}

// CallToolResponse represents the result of calling a tool
type CallToolResponse struct {
	Tool   string          `json:"tool"`
	Result json.RawMessage `json:"result"`
}

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewToolMeta creates a ToolMeta with the given name, description and optional
// input schema. The schema value (if non-nil) is marshalled to JSON.
func NewToolMeta(name, description string, inputSchema any) (ToolMeta, error) {
	meta := ToolMeta{
		Name:        name,
		Description: description,
	}
	if inputSchema != nil {
		data, err := json.Marshal(inputSchema)
		if err != nil {
			return meta, fmt.Errorf("tool %q schema: %w", name, err)
		}
		if string(data) != "null" {
			meta.Input = NewJSONSchema(data)
		}
	}
	return meta, nil
}

////////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (r ModelListRequest) String() string {
	return types.Stringify(r)
}

func (r ModelListRequest) Query() url.Values {
	values := url.Values{}
	if r.Offset > 0 {
		values.Set("offset", fmt.Sprintf("%d", r.Offset))
	}
	if r.Limit != nil {
		values.Set("limit", fmt.Sprintf("%d", types.Value(r.Limit)))
	}
	if r.Provider != "" {
		values.Set("provider", r.Provider)
	}
	return values
}

func (r ModelList) String() string {
	return types.Stringify(r)
}

func (r GetModelRequest) String() string {
	return types.Stringify(r)
}

func (r DownloadModelRequest) String() string {
	return types.Stringify(r)
}

func (r DeleteModelRequest) String() string {
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

func (r ListAgentRequest) String() string {
	return types.Stringify(r)
}

func (r ListAgentResponse) String() string {
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

func (r ListConnectorsRequest) String() string {
	return types.Stringify(r)
}

func (r ListConnectorsResponse) String() string {
	return types.Stringify(r)
}

func (r CallToolRequest) String() string {
	return types.Stringify(r)
}

func (r CallToolResponse) String() string {
	return types.Stringify(r)
}

func (r CompletionResponse) String() string {
	return types.Stringify(r)
}

func (r AskRequestCore) String() string {
	return types.Stringify(r)
}

func (r AskRequest) String() string {
	return types.Stringify(r)
}

func (r AskResponse) String() string {
	return types.Stringify(r)
}

func (r ChatRequestCore) String() string {
	return types.Stringify(r)
}

func (r ChatRequest) String() string {
	return types.Stringify(r)
}

func (r ChatResponse) String() string {
	return types.Stringify(r)
}

func (r CreateAgentSessionRequest) String() string {
	return types.Stringify(r)
}

func (r CreateAgentSessionResponse) String() string {
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

func fileAttachment(f types.File) (*Attachment, error) {
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
		ContentType: http.DetectContentType(data),
		Data:        data,
	}
	if f.Path != "" {
		a.URL = types.Ptr(url.URL{Scheme: "file", Path: f.Path})
	}
	return a, nil
}
