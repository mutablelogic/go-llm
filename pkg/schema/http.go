package schema

import (
	"encoding/json"
	"io"
	"net/http"

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
	Model                string   `json:"model,omitempty" arg:"" help:"Model name"`
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

// CompletionRequest represents a request to generate content.
// Accepts JSON or multipart/form-data with file attachments.
type CompletionRequest struct {
	Text       string           `json:"text" help:"User input text"`
	Attachment gomultipart.File `json:"attachment,omitempty" help:"File attachment" optional:""`
}

// Attachments returns attachment content blocks from the request.
// If Attachment.Body is set (from multipart upload), it reads and
// auto-detects the MIME type.
func (r *CompletionRequest) Attachments() ([]ContentBlock, error) {
	if r.Attachment.Body == nil {
		return nil, nil
	}
	data, err := io.ReadAll(r.Attachment.Body)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}
	return []ContentBlock{
		{Attachment: types.Ptr(Attachment{
			Type: detectContentType(data),
			Data: data,
		})},
	}, nil
}

// CompletionResponse represents a response from a completion request.
type CompletionResponse struct {
	Role    string         `json:"role"`
	Content []ContentBlock `json:"content"`
	Result  ResultType     `json:"result,omitempty"`
}

// SessionMeta represents the metadata for a session.
type SessionMeta struct {
	Name         string `json:"name,omitempty" help:"Session name" optional:""`
	Provider     string `json:"provider,omitempty" help:"Provider name" optional:""`
	Model        string `json:"model" help:"Model name"`
	SystemPrompt string `json:"system_prompt,omitempty" help:"System prompt" optional:""`
}

// GetSessionRequest represents a request to get a session by ID
type GetSessionRequest struct {
	ID string `json:"id" help:"Session ID"`
}

// DeleteSessionRequest represents a request to delete a session by ID
type DeleteSessionRequest struct {
	ID string `json:"id" help:"Session ID"`
}

// ListSessionRequest represents a request to list sessions
type ListSessionRequest struct {
	Limit  *uint `json:"limit,omitempty" help:"Maximum number of sessions to return"`
	Offset uint  `json:"offset,omitempty" help:"Offset for pagination"`
}

// ListSessionResponse represents a response containing a list of sessions
type ListSessionResponse struct {
	Count  uint       `json:"count"`
	Offset uint       `json:"offset,omitzero"`
	Limit  *uint      `json:"limit,omitzero"`
	Body   []*Session `json:"body,omitzero"`
}

// ToolMeta represents a tool's metadata
type ToolMeta struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Schema      json.RawMessage `json:"schema,omitempty"`
}

// GetToolRequest represents a request to get a tool by name
type GetToolRequest struct {
	Name string `json:"name" help:"Tool name"`
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

func (r SessionMeta) String() string {
	return types.Stringify(r)
}

func (r GetSessionRequest) String() string {
	return types.Stringify(r)
}

func (r DeleteSessionRequest) String() string {
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

func (r GetToolRequest) String() string {
	return types.Stringify(r)
}

func (r ListToolRequest) String() string {
	return types.Stringify(r)
}

func (r ListToolResponse) String() string {
	return types.Stringify(r)
}

func (r CompletionRequest) String() string {
	return types.Stringify(r)
}

func (r CompletionResponse) String() string {
	return types.Stringify(r)
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func detectContentType(data []byte) string {
	return http.DetectContentType(data)
}
