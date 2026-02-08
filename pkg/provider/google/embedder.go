package google

import (
	"context"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// INTERFACE CHECK

var _ llm.Embedder = (*Client)(nil)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Embedding generates an embedding vector for a single text using the specified model
func (c *Client) Embedding(ctx context.Context, model schema.Model, text string, opts ...opt.Opt) ([]float64, error) {
	// Apply options
	o, err := opt.Apply(opts...)
	if err != nil {
		return nil, err
	}

	// Create request
	request := &geminiEmbedRequest{
		Content: geminiNewTextContent(text, "user"),
	}
	applyEmbedOpts(request, o)

	// Create payload
	payload, err := client.NewJSONRequest(request)
	if err != nil {
		return nil, err
	}

	// Execute the request
	var response geminiEmbedResponse
	if err := c.DoWithContext(ctx, payload, &response, client.OptPath("models", model.Name+":embedContent")); err != nil {
		return nil, err
	}
	if response.Embedding == nil {
		return nil, llm.ErrInternalServerError.With("empty embedding response")
	}
	return response.Embedding.Values, nil
}

// BatchEmbedding generates embedding vectors for multiple texts using the specified model
func (c *Client) BatchEmbedding(ctx context.Context, model schema.Model, texts []string, opts ...opt.Opt) ([][]float64, error) {
	if len(texts) == 0 {
		return nil, llm.ErrBadParameter.With("at least one text is required")
	}

	// Apply options
	o, err := opt.Apply(opts...)
	if err != nil {
		return nil, err
	}

	// Create batch request
	requests := make([]*geminiEmbedRequest, 0, len(texts))
	for _, text := range texts {
		req := &geminiEmbedRequest{
			Model:   "models/" + model.Name,
			Content: geminiNewTextContent(text, "user"),
		}
		applyEmbedOpts(req, o)
		requests = append(requests, req)
	}

	// Create payload
	payload, err := client.NewJSONRequest(&geminiBatchEmbedRequest{Requests: requests})
	if err != nil {
		return nil, err
	}

	// Execute the request
	var response geminiBatchEmbedResponse
	if err := c.DoWithContext(ctx, payload, &response, client.OptPath("models", model.Name+":batchEmbedContents")); err != nil {
		return nil, err
	}

	// Convert response
	result := make([][]float64, 0, len(response.Embeddings))
	for _, embedding := range response.Embeddings {
		result = append(result, embedding.Values)
	}
	return result, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// applyEmbedOpts sets optional fields on a geminiEmbedRequest from applied options
func applyEmbedOpts(req *geminiEmbedRequest, o opt.Options) {
	if v := o.GetString(opt.TaskTypeKey); v != "" {
		req.TaskType = v
	}
	if v := o.GetString(opt.TitleKey); v != "" {
		req.Title = v
	}
	if v := o.GetUint(opt.OutputDimensionalityKey); v > 0 {
		req.OutputDimensionality = int(v)
	}
}
