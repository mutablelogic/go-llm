package ollama

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	// Packages
	client "github.com/mutablelogic/go-client"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
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
		return schema.ErrBadParameter.With("model does not belong to this client")
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
		return schema.ErrBadParameter.With("model does not belong to this client")
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
		return schema.ErrBadParameter.With("model does not belong to this client")
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
	result := schema.Model{
		Name:            m.Name,
		Description:     m.Model,
		Created:         m.ModifiedAt,
		OwnedBy:         c.Name(),
		InputTokenLimit: contextLengthFromModel(m),
		Cap:             modelCapabilities(m.Capabilities),
	}

	// Marshal Details to JSON then unmarshal into a map so we use the
	// canonical JSON encoding rather than the String() representation.
	meta := make(map[string]any)
	if data, err := json.Marshal(m.Details); err == nil {
		_ = json.Unmarshal(data, &meta)
	}
	for k, v := range m.Info {
		meta[k] = v
	}
	meta = compactMetaMap(meta)
	if len(meta) > 0 {
		result.Meta = meta
	}

	return result
}

func contextLengthFromModel(m model) *uint {
	if limit := findPositiveUint(m.Info, m.Details.Family+".context_length"); limit != nil {
		return limit
	}
	for key, value := range m.Info {
		if key == "context_length" || strings.HasSuffix(key, ".context_length") {
			if limit := positiveUint(value); limit != nil {
				return limit
			}
		}
	}
	return nil
}

func findPositiveUint(values map[string]any, key string) *uint {
	if key == "" {
		return nil
	}
	value, exists := values[key]
	if !exists {
		return nil
	}
	return positiveUint(value)
}

func positiveUint(value any) *uint {
	switch value := value.(type) {
	case uint:
		if value == 0 {
			return nil
		}
		result := value
		return &result
	case uint32:
		if value == 0 {
			return nil
		}
		result := uint(value)
		return &result
	case uint64:
		if value == 0 {
			return nil
		}
		result := uint(value)
		return &result
	case int:
		if value <= 0 {
			return nil
		}
		result := uint(value)
		return &result
	case int32:
		if value <= 0 {
			return nil
		}
		result := uint(value)
		return &result
	case int64:
		if value <= 0 {
			return nil
		}
		result := uint(value)
		return &result
	case float32:
		if value <= 0 {
			return nil
		}
		result := uint(value)
		return &result
	case float64:
		if value <= 0 {
			return nil
		}
		result := uint(value)
		return &result
	default:
		return nil
	}
}

func modelCapabilities(values []string) schema.ModelCap {
	var cap schema.ModelCap

	for _, value := range values {
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "completion":
			cap |= schema.ModelCapCompletion
		case "embedding", "embeddings":
			cap |= schema.ModelCapEmbeddings
		case "vision":
			cap |= schema.ModelCapVision
		case "tools", "tool":
			cap |= schema.ModelCapTools
		case "thinking":
			cap |= schema.ModelCapThinking
		case "transcription":
			cap |= schema.ModelCapTranscription
		case "translation":
			cap |= schema.ModelCapTranslation
		}
	}

	return cap
}

func compactMetaMap(values map[string]any) map[string]any {
	if len(values) == 0 {
		return nil
	}
	result := make(map[string]any, len(values))
	for key, value := range values {
		if compacted, ok := compactMetaValue(value); ok {
			result[key] = compacted
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func compactMetaValue(value any) (any, bool) {
	switch value := value.(type) {
	case nil:
		return nil, false
	case string:
		if strings.TrimSpace(value) == "" {
			return nil, false
		}
		return value, true
	case []any:
		result := make([]any, 0, len(value))
		for _, item := range value {
			if compacted, ok := compactMetaValue(item); ok {
				result = append(result, compacted)
			}
		}
		if len(result) == 0 {
			return nil, false
		}
		return result, true
	case map[string]any:
		result := compactMetaMap(value)
		if len(result) == 0 {
			return nil, false
		}
		return result, true
	default:
		return value, true
	}
}
