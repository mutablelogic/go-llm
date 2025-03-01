package ollama

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	impl "github.com/mutablelogic/go-llm/pkg/internal/impl"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// model is the implementation of the llm.Model interface
type model struct {
	*Client `json:"-"`
	ModelMeta
}

var _ llm.Model = (*model)(nil)

// ModelMeta is the metadata for an ollama model
type ModelMeta struct {
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

// PullStatus provides the status of a pull operation in a callback function
type PullStatus struct {
	Status         string `json:"status"`
	DigestName     string `json:"digest,omitempty"`
	TotalBytes     int64  `json:"total,omitempty"`
	CompletedBytes int64  `json:"completed,omitempty"`
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (m model) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.ModelMeta)
}

func (m model) String() string {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(data)
}

func (m PullStatus) String() string {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(data)
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - llm.Model implementation

func (m model) Name() string {
	return m.ModelMeta.Name
}

// Return model name
func (model) Aliases() []string {
	return nil
}

// Return model description
func (model model) Description() string {
	return strings.Join(model.ModelMeta.Details.Families, ", ")
}

// Agent interface
func (ollama *Client) Models(ctx context.Context) ([]llm.Model, error) {
	// We don't explicitly cache models
	return ollama.ListModels(ctx)
}

// Return the a model by name
func (ollama *Client) Model(ctx context.Context, name string) llm.Model {
	model, err := ollama.GetModel(ctx, name)
	if err != nil {
		panic(err)
	}

	// In the ollama version, we attempt to load the model into
	// memory here, so that we can use it immediately
	ollama.LoadModel(ctx, name)

	// Return the model
	return model
}

// Return a new empty session
func (model *model) Context(opts ...llm.Opt) llm.Context {
	return impl.NewSession(model, &messagefactory{}, opts...)
}

///////////////////////////////////////////////////////////////////////////////
// API CALLS

// List models
func (ollama *Client) ListModels(ctx context.Context) ([]llm.Model, error) {
	type respListModel struct {
		Models []ModelMeta `json:"models"`
	}

	// Send the request
	var response respListModel
	if err := ollama.DoWithContext(ctx, nil, &response, client.OptPath("tags")); err != nil {
		return nil, err
	}

	// Convert to llm.Model
	result := make([]llm.Model, 0, len(response.Models))
	for _, meta := range response.Models {
		result = append(result, &model{ollama, meta})
	}

	// Return models
	return result, nil
}

// List running models
func (ollama *Client) ListRunningModels(ctx context.Context) ([]llm.Model, error) {
	type respListModel struct {
		Models []ModelMeta `json:"models"`
	}

	// Send the request
	var response respListModel
	if err := ollama.DoWithContext(ctx, nil, &response, client.OptPath("ps")); err != nil {
		return nil, err
	}

	// Convert to llm.Model
	result := make([]llm.Model, 0, len(response.Models))
	for _, meta := range response.Models {
		result = append(result, &model{ollama, meta})
	}

	// Return models
	return result, nil
}

// Get model details
func (ollama *Client) GetModel(ctx context.Context, name string) (llm.Model, error) {
	type reqGetModel struct {
		Model string `json:"model"`
	}

	// Request
	req, err := client.NewJSONRequest(reqGetModel{
		Model: name,
	})
	if err != nil {
		return nil, err
	}

	// Response
	var response ModelMeta
	if err := ollama.DoWithContext(ctx, req, &response, client.OptPath("show")); err != nil {
		return nil, err
	} else {
		response.Name = name
	}

	// Return success
	return &model{ollama, response}, nil
}

// Copy a local model by name
func (ollama *Client) CopyModel(ctx context.Context, source, destination string) error {
	type reqCopyModel struct {
		Source      string `json:"source"`
		Destination string `json:"destination"`
	}

	// Request
	req, err := client.NewJSONRequest(reqCopyModel{
		Source:      source,
		Destination: destination,
	})
	if err != nil {
		return err
	}

	// Response
	return ollama.Do(req, nil, client.OptPath("copy"))
}

// Delete a local model by name
func (ollama *Client) DeleteModel(ctx context.Context, name string) error {
	type reqGetModel struct {
		Model string `json:"model"`
	}

	// Request
	req, err := client.NewJSONRequestEx(http.MethodDelete, reqGetModel{
		Model: name,
	}, client.ContentTypeAny)
	if err != nil {
		return err
	}

	// Response
	return ollama.Do(req, nil, client.OptPath("delete"))
}

// Pull a remote model locally
func (ollama *Client) PullModel(ctx context.Context, name string, opts ...llm.Opt) (llm.Model, error) {
	type reqPullModel struct {
		Model    string `json:"model"`
		Insecure bool   `json:"insecure,omitempty"`
		Stream   bool   `json:"stream"`
	}

	// Apply options
	opt, err := llm.ApplyOpts(opts...)
	if err != nil {
		return nil, err
	}

	// Request
	req, err := client.NewJSONRequest(reqPullModel{
		Model:    name,
		Stream:   optPullStatus(opt) != nil,
		Insecure: optInsecure(opt),
	})
	if err != nil {
		return nil, err
	}

	//  Response
	var response PullStatus
	if err := ollama.DoWithContext(ctx, req, &response, client.OptPath("pull"), client.OptNoTimeout(), client.OptJsonStreamCallback(func(v any) error {
		if v, ok := v.(*PullStatus); ok && v != nil {
			if fn := optPullStatus(opt); fn != nil {
				fn(v)
			}
		}
		return nil
	})); err != nil {
		return nil, err
	}

	// Return success
	return ollama.GetModel(ctx, name)
}

// Load a model into memory
func (ollama *Client) LoadModel(ctx context.Context, name string) (llm.Model, error) {
	type reqLoadModel struct {
		Model string `json:"model"`
	}

	// Request
	req, err := client.NewJSONRequest(reqLoadModel{
		Model: name,
	})
	if err != nil {
		return nil, err
	}

	// Response
	if err := ollama.DoWithContext(ctx, req, nil, client.OptPath("generate")); err != nil {
		return nil, err
	}

	// Return success
	return ollama.GetModel(ctx, name)
}

// Unload a model from memory
func (ollama *Client) UnloadModel(ctx context.Context, name string) error {
	type reqLoadModel struct {
		Model     string `json:"model"`
		KeepAlive uint   `json:"keepalive"`
	}

	// Request
	req, err := client.NewJSONRequest(reqLoadModel{
		Model:     name,
		KeepAlive: 0,
	})
	if err != nil {
		return err
	}

	// Response
	return ollama.DoWithContext(ctx, req, nil, client.OptPath("generate"))
}
