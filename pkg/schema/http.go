package schema

import (
	// Packages
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
	Limit    uint   `json:"limit,omitempty" help:"Maximum number of models to return"`
	Offset   uint   `json:"offset,omitempty" help:"Offset for pagination"`
}

// ListModelsResponse represents a response containing a list of models and providers
type ListModelsResponse struct {
	Count    uint     `json:"count"`
	Offset   uint     `json:"offset"`
	Limit    uint     `json:"limit"`
	Provider []string `json:"provider,omitempty"`
	Body     []Model  `json:"body"`
}

// GetModelRequest represents a request to get a model
type GetModelRequest struct {
	Provider string `json:"provider,omitempty" help:"Filter by provider name" optional:""`
	Name     string `json:"name,omitempty" help:"Model name"`
}

// EmbeddingRequest represents a request to embed text
type EmbeddingRequest struct {
	Provider             string   `json:"provider,omitempty" help:"Provider name" optional:""`
	Model                string   `json:"model,omitempty" help:"Model name"`
	Input                []string `json:"input,omitempty" help:"Text inputs to embed"`
	TaskType             string   `json:"task_type,omitempty" help:"Embedding task type (Google-specific)" enum:"DEFAULT,RETRIEVAL_QUERY,RETRIEVAL_DOCUMENT,SEMANTIC_SIMILARITY,CLASSIFICATION,CLUSTERING,QUESTION_ANSWERING,FACT_VERIFICATION,CODE_RETRIEVAL_QUERY," default:"DEFAULT"`
	Title                string   `json:"title,omitempty" help:"Document title, used with RETRIEVAL_DOCUMENT task type (Google-specific)"`
	OutputDimensionality uint     `json:"output_dimensionality,omitempty" help:"Truncate embedding to this many dimensions (Google-specific)"`
}

// EmbeddingResponse represents a response from an embedding request
type EmbeddingResponse struct {
	EmbeddingRequest
	Output [][]float64 `json:"output,omitempty"`
}

// CreateSessionRequest represents a request to create a new session
type CreateSessionRequest struct {
	Name     string `json:"name,omitempty" help:"Session name" optional:""`
	Provider string `json:"provider,omitempty" help:"Provider name" optional:""`
	Model    string `json:"model" help:"Model name"`
}

// GetSessionRequest represents a request to get a session by ID
type GetSessionRequest struct {
	ID string `json:"id" help:"Session ID"`
}

// DeleteSessionRequest represents a request to delete a session by ID
type DeleteSessionRequest struct {
	ID string `json:"id" help:"Session ID"`
}

// ListSessionsRequest represents a request to list sessions
type ListSessionsRequest struct {
	Limit  uint `json:"limit,omitempty" help:"Maximum number of sessions to return"`
	Offset uint `json:"offset,omitempty" help:"Offset for pagination"`
}

// ListSessionsResponse represents a response containing a list of sessions
type ListSessionsResponse struct {
	Count  uint       `json:"count"`
	Offset uint       `json:"offset"`
	Limit  uint       `json:"limit"`
	Body   []*Session `json:"body"`
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

func (r CreateSessionRequest) String() string {
	return types.Stringify(r)
}

func (r GetSessionRequest) String() string {
	return types.Stringify(r)
}

func (r DeleteSessionRequest) String() string {
	return types.Stringify(r)
}

func (r ListSessionsRequest) String() string {
	return types.Stringify(r)
}

func (r ListSessionsResponse) String() string {
	return types.Stringify(r)
}
