package ollama

import (
	"context"
	"net/http"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// model represents the API response for a model from Ollama
type model struct {
	Name       string       `json:"name"`
	Model      string       `json:"model,omitempty"`
	ModifiedAt time.Time    `json:"modified_at"`
	Size       int64        `json:"size,omitempty"`
	Digest     string       `json:"digest,omitempty"`
	Details    ModelDetails `json:"details"`
	File       string       `json:"modelfile,omitempty"`
	Parameters string       `json:"parameters,omitempty"`
	Template   string       `json:"template,omitempty"`
	Info       ModelInfo    `json:"model_info,omitempty"`
}

// ModelDetails are the details of the model
type ModelDetails struct {
	ParentModel       string   `json:"parent_model,omitempty"`
	Format            string   `json:"format"`
	Family            string   `json:"family"`
	Families          []string `json:"families"`
	ParameterSize     string   `json:"parameter_size"`
	QuantizationLevel string   `json:"quantization_level"`
}

// ModelInfo provides additional model parameters
type ModelInfo map[string]any

// listModelsResponse represents the API response for listing models
type listModelsResponse struct {
	Data []model `json:"models"`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// List all models in the Ollama registry
func (ollama *Client) ListModels(ctx context.Context) ([]schema.Model, error) {
	// Send the request
	var response listModelsResponse
	if err := ollama.DoWithContext(ctx, nil, &response, client.OptPath("tags")); err != nil {
		return nil, err
	}

	result := make([]schema.Model, len(response.Data))
	for i, m := range response.Data {
		result[i] = m.toSchema()
	}

	// Return models
	return result, nil
}

// List running models
func (ollama *Client) ListRunningModels(ctx context.Context) ([]schema.Model, error) {
	// Send the request
	var response listModelsResponse
	if err := ollama.DoWithContext(ctx, nil, &response, client.OptPath("ps")); err != nil {
		return nil, err
	}

	result := make([]schema.Model, len(response.Data))
	for i, m := range response.Data {
		result[i] = m.toSchema()
	}

	// Return models
	return result, nil
}

// GetModel returns the model with the given name
func (ollama *Client) GetModel(ctx context.Context, name string) (*schema.Model, error) {
	var response model
	req, err := client.NewJSONRequest(map[string]string{"name": name})
	if err != nil {
		return nil, err
	}
	if err := ollama.DoWithContext(ctx, req, &response, client.OptPath("show")); err != nil {
		return nil, err
	}

	result := response.toSchema()
	// The show endpoint doesn't return the name, so set it from the request
	if result.Name == "" {
		result.Name = name
	}
	return &result, nil
}

// Delete a model by name
func (ollama *Client) DeleteModel(ctx context.Context, model schema.Model) error {
	type reqGetModel struct {
		Model string `json:"model"`
	}

	// Check model
	if model.Name != ollama.Name() {
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
	return ollama.DoWithContext(ctx, req, nil, client.OptPath("delete"))
}

// Load a model into memory
func (ollama *Client) LoadModel(ctx context.Context, model schema.Model) error {
	type reqGetModel struct {
		Model string `json:"model"`
	}

	// Check model
	if model.Name != ollama.Name() {
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
	return ollama.DoWithContext(ctx, req, nil, client.OptPath("generate"))
}

// Unload a model from memory
func (ollama *Client) UnloadModel(ctx context.Context, model schema.Model) error {
	type reqLoadModel struct {
		Model     string `json:"model"`
		KeepAlive uint   `json:"keepalive"`
	}

	// Check model
	if model.Name != ollama.Name() {
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
func (ollama *Client) DownloadModel(ctx context.Context, path string) (*schema.Model, error) {
	return nil, llm.ErrNotImplemented.With("TODO Ollama does not support downloading models via API YET")
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// toSchema converts an API model response to schema.Model
func (m model) toSchema() schema.Model {
	return schema.Model{
		Name:        m.Name,
		Description: m.Model,
		Created:     m.ModifiedAt,
		OwnedBy:     defaultName,
	}
}
