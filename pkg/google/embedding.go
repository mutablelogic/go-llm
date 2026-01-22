package google

import (
	"context"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// embedContentRequest is the request structure for embedContent API
type embedContentRequest struct {
	Model                string         `json:"model,omitempty"`
	Content              contentRequest `json:"content"`
	TaskType             string         `json:"taskType,omitempty"`
	Title                string         `json:"title,omitempty"`
	OutputDimensionality *uint          `json:"outputDimensionality,omitempty"`
}

// embedContentResponse is the response structure for embedContent API
type embedContentResponse struct {
	Embedding contentEmbedding `json:"embedding"`
}

// contentEmbedding represents the embedding vector
type contentEmbedding struct {
	Values []float64 `json:"values"`
}

// batchEmbedContentsRequest is the request structure for batchEmbedContents API
type batchEmbedContentsRequest struct {
	Requests []embedContentRequest `json:"requests"`
}

// batchEmbedContentsResponse is the response structure for batchEmbedContents API
type batchEmbedContentsResponse struct {
	Embeddings []contentEmbedding `json:"embeddings"`
}

///////////////////////////////////////////////////////////////////////////////
// INTERFACE CHECK

var _ llm.Embedder = (*Client)(nil)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Embedding generates an embedding vector for a single text using the specified model
func (c *Client) Embedding(ctx context.Context, model string, text string, opts ...opt.Opt) ([]float64, error) {
	// Build the request
	req, err := c.embedContentRequest("", text, opts...)
	if err != nil {
		return nil, err
	}

	// Create the HTTP request
	httpReq, err := client.NewJSONRequest(req)
	if err != nil {
		return nil, err
	}

	// Execute the request
	var response embedContentResponse
	if err := c.DoWithContext(ctx, httpReq, &response, client.OptPath("models", model+":embedContent")); err != nil {
		return nil, err
	}

	return response.Embedding.Values, nil
}

// BatchEmbedding generates embedding vectors for multiple texts using the specified model
func (c *Client) BatchEmbedding(ctx context.Context, model string, texts []string, opts ...opt.Opt) ([][]float64, error) {
	if len(texts) == 0 {
		return nil, llm.ErrBadParameter.With("at least one text is required")
	}

	// Build the batch request
	requests := make([]embedContentRequest, 0, len(texts))
	for _, text := range texts {
		req, err := c.embedContentRequest(model, text, opts...)
		if err != nil {
			return nil, err
		}
		requests = append(requests, *req)
	}

	batchReq := batchEmbedContentsRequest{
		Requests: requests,
	}

	// Create the HTTP request
	httpReq, err := client.NewJSONRequest(batchReq)
	if err != nil {
		return nil, err
	}

	// Execute the request
	var response batchEmbedContentsResponse
	if err := c.DoWithContext(ctx, httpReq, &response, client.OptPath("models", model+":batchEmbedContents")); err != nil {
		return nil, err
	}

	// Extract the embedding vectors
	result := make([][]float64, 0, len(response.Embeddings))
	for _, embedding := range response.Embeddings {
		result = append(result, embedding.Values)
	}

	return result, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// embedContentRequest builds an embedContentRequest from options
func (c *Client) embedContentRequest(model string, text string, opts ...opt.Opt) (*embedContentRequest, error) {
	// Apply options
	o, err := opt.Apply(opts...)
	if err != nil {
		return nil, err
	}

	req := &embedContentRequest{
		Content: contentRequest{
			Parts: []part{
				{Text: text},
			},
		},
	}

	// Set model if specified (required for batch requests)
	if model != "" {
		req.Model = "models/" + model
	}

	// Set task type if specified
	if taskType := o.GetString("task_type"); taskType != "" {
		req.TaskType = taskType
	}

	// Set title if specified (only for RETRIEVAL_DOCUMENT)
	if title := o.GetString("title"); title != "" {
		req.Title = title
	}

	// Set output dimensionality if specified
	if dim := o.GetUint("output_dimensionality"); dim > 0 {
		req.OutputDimensionality = &dim
	}

	return req, nil
}
