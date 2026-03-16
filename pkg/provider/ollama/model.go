package ollama

import (
	"context"
	"encoding/json"
	"net/http"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// List all models in the Ollama registry
func (ollama *Client) ListModels(ctx context.Context, opts ...opt.Opt) ([]schema.Model, error) {
	return ollama.ModelCache.ListModels(ctx, opts, func(ctx context.Context, opts ...opt.Opt) ([]schema.Model, error) {
		var response listModelsResponse
		if err := ollama.DoWithContext(ctx, nil, &response, client.OptPath("tags")); err != nil {
			return nil, err
		}
		result := make([]schema.Model, len(response.Data))
		for i, m := range response.Data {
			result[i] = ollama.modelToSchema(m)
		}
		return result, nil
	})
}

// List running models
func (ollama *Client) ListRunningModels(ctx context.Context) ([]schema.Model, error) {
	var response listModelsResponse
	if err := ollama.DoWithContext(ctx, nil, &response, client.OptPath("ps")); err != nil {
		return nil, err
	}
	result := make([]schema.Model, len(response.Data))
	for i, m := range response.Data {
		result[i] = ollama.modelToSchema(m)
	}
	return result, nil
}

// GetModel returns the model with the given name
func (ollama *Client) GetModel(ctx context.Context, name string, opts ...opt.Opt) (*schema.Model, error) {
	return ollama.ModelCache.GetModel(ctx, name, func(ctx context.Context, name string) (*schema.Model, error) {
		var response model
		req, err := client.NewJSONRequest(map[string]string{"name": name})
		if err != nil {
			return nil, err
		}
		if err := ollama.DoWithContext(ctx, req, &response, client.OptPath("show")); err != nil {
			return nil, err
		}
		result := ollama.modelToSchema(response)
		// The show endpoint doesn't return the name, so set it from the request
		if result.Name == "" {
			result.Name = name
		}
		return &result, nil
	})
}

// Delete a model by name
func (ollama *Client) DeleteModel(ctx context.Context, model schema.Model) error {
	type reqGetModel struct {
		Model string `json:"model"`
	}

	// Check model
	if model.OwnedBy != ollama.Name() {
		return llm.ErrBadParameter.With("model does not belong to this client")
	}

	// Request
	req, err := client.NewJSONRequestEx(http.MethodDelete, reqGetModel{
		Model: model.Name,
	}, client.ContentTypeAny)
	if err != nil {
		return err
	}

	// Response
	if err := ollama.DoWithContext(ctx, req, nil, client.OptPath("delete")); err != nil {
		return err
	}

	// Invalidate the model cache
	ollama.ModelCache.Flush()
	return nil
}

// Load a model into memory
func (ollama *Client) LoadModel(ctx context.Context, model schema.Model) error {
	type reqGetModel struct {
		Model string `json:"model"`
	}

	// Check model
	if model.OwnedBy != ollama.Name() {
		return llm.ErrBadParameter.With("model does not belong to this client")
	}

	// Request
	req, err := client.NewJSONRequestEx(http.MethodPost, reqGetModel{
		Model: model.Name,
	}, client.ContentTypeAny)
	if err != nil {
		return err
	}

	// Response
	return ollama.DoWithContext(ctx, req, nil, client.OptPath("generate"))
}

// Unload a model from memory
func (ollama *Client) UnloadModel(ctx context.Context, model schema.Model) error {
	type reqLoadModel struct {
		Model     string `json:"model"`
		KeepAlive uint   `json:"keep_alive"`
	}

	// Check model
	if model.OwnedBy != ollama.Name() {
		return llm.ErrBadParameter.With("model does not belong to this client")
	}

	// Request
	req, err := client.NewJSONRequest(reqLoadModel{
		Model:     model.Name,
		KeepAlive: 0,
	})
	if err != nil {
		return err
	}

	// Response
	return ollama.DoWithContext(ctx, req, nil, client.OptPath("generate"))
}

// Download (pull) a model by name
func (ollama *Client) DownloadModel(ctx context.Context, name string, opts ...opt.Opt) (*schema.Model, error) {
	type reqPullModel struct {
		Model    string `json:"model"`
		Insecure bool   `json:"insecure,omitempty"`
		Stream   bool   `json:"stream"`
	}

	// Apply options to get progress callback
	options, err := opt.Apply(opts...)
	if err != nil {
		return nil, err
	}
	progressFn := options.GetProgress()

	// Enable streaming if progress callback is provided
	stream := progressFn != nil

	// Request
	req, err := client.NewJSONRequest(reqPullModel{
		Model:  name,
		Stream: stream,
	})
	if err != nil {
		return nil, err
	}

	// Response with optional streaming callback
	var response PullStatus
	clientOpts := []client.RequestOpt{client.OptPath("pull"), client.OptNoTimeout()}

	// Add streaming callback if progress function is provided
	if progressFn != nil {
		clientOpts = append(clientOpts, client.OptJsonStreamCallback(func(v any) error {
			if status, ok := v.(*PullStatus); ok && status != nil {
				// Calculate progress percentage
				var percent float64
				if status.TotalBytes > 0 {
					percent = float64(status.CompletedBytes) / float64(status.TotalBytes) * 100.0
				}
				// Call progress callback
				progressFn(status.Status, percent)
			}
			return nil
		}))
	}

	if err := ollama.DoWithContext(ctx, req, &response, clientOpts...); err != nil {
		return nil, err
	}

	// Invalidate the model cache so the newly downloaded model is fetched fresh
	ollama.ModelCache.Flush()

	// Return the downloaded model
	return ollama.GetModel(ctx, name)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// modelToSchema converts an API model response to schema.Model
func (c *Client) modelToSchema(m model) schema.Model {
	meta := make(map[string]any)
	if err := json.Unmarshal([]byte(m.Details.String()), &meta); err != nil {
		return schema.Model{}
	}
	for k, v := range m.Info {
		meta[k] = v
	}
	return schema.Model{
		Name:        m.Name,
		Description: m.Model,
		Created:     m.ModifiedAt,
		OwnedBy:     c.Name(),
		Meta:        meta,
	}
}
