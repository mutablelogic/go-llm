package provider

import (
	"context"
	"testing"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	assert "github.com/stretchr/testify/assert"
)

type registryModelClient struct {
	name   string
	models []schema.Model
	err    error
	getErr error
}

func registryProvider(name string, include, exclude []string) *schema.Provider {
	return &schema.Provider{
		Name: name,
		ProviderMeta: schema.ProviderMeta{
			Include: include,
			Exclude: exclude,
		},
	}
}

var _ llm.Client = (*registryModelClient)(nil)

func (c *registryModelClient) Name() string {
	return c.name
}

func (c *registryModelClient) Ping(context.Context) error {
	return nil
}

func (c *registryModelClient) ListModels(context.Context, ...opt.Opt) ([]schema.Model, error) {
	if c.err != nil {
		return nil, c.err
	}
	return append([]schema.Model(nil), c.models...), nil
}

func (c *registryModelClient) GetModel(_ context.Context, name string, _ ...opt.Opt) (*schema.Model, error) {
	if c.getErr != nil {
		return nil, c.getErr
	}
	if c.err != nil {
		return nil, c.err
	}
	for _, model := range c.models {
		if model.Name == name {
			copy := model
			return &copy, nil
		}
	}
	return nil, schema.ErrNotFound.Withf("model %q not found", name)
}

func TestRegistryGetModelsInvalidPattern(t *testing.T) {
	assert := assert.New(t)

	r := New()
	r.providers["ollama"] = provider{client: &registryModelClient{name: "ollama"}}

	_, err := r.GetModels(context.Background(), registryProvider("ollama", []string{"["}, nil))
	if assert.Error(err) {
		assert.ErrorIs(err, schema.ErrBadParameter)
	}
}

func TestRegistryGetModelsIncludeExclude(t *testing.T) {
	assert := assert.New(t)

	r := New()
	r.providers["ollama"] = provider{client: &registryModelClient{
		name: "ollama",
		models: []schema.Model{
			{Name: "llama3.2"},
			{Name: "mistral-small"},
			{Name: "llama3.2-vision"},
			{Name: "gemma3"},
		},
	}}

	models, err := r.GetModels(context.Background(), registryProvider("ollama", []string{"^llama", "^mistral"}, []string{"vision$"}))
	if !assert.NoError(err) {
		return
	}

	if assert.Len(models, 2) {
		assert.Equal("llama3.2", models[0].Name)
		assert.Equal("ollama", models[0].OwnedBy)
		assert.Equal("mistral-small", models[1].Name)
		assert.Equal("ollama", models[1].OwnedBy)
	}
	assert.Len(r.regexpCache, 3)
}

func TestRegistryGetModelsOverwritesOwnedBy(t *testing.T) {
	assert := assert.New(t)

	r := New()
	r.providers["proxy"] = provider{client: &registryModelClient{
		name:   "proxy",
		models: []schema.Model{{Name: "claude-sonnet", OwnedBy: "anthropic"}},
	}}

	models, err := r.GetModels(context.Background(), registryProvider("proxy", nil, nil))
	if !assert.NoError(err) {
		return
	}

	if assert.Len(models, 1) {
		assert.Equal("proxy", models[0].OwnedBy)
	}
}

func TestRegistryGetModelsWrapsProviderError(t *testing.T) {
	assert := assert.New(t)

	r := New()
	r.providers["mistral-primary"] = provider{client: &registryModelClient{
		name: "mistral",
		err:  schema.ErrBadParameter.With("backend boom"),
	}}

	_, err := r.GetModels(context.Background(), &schema.Provider{
		Name:     "mistral-primary",
		Provider: schema.Mistral,
	})
	if assert.Error(err) {
		assert.ErrorIs(err, schema.ErrBadParameter)
		assert.Contains(err.Error(), "mistral-primary")
		assert.Contains(err.Error(), schema.Mistral)
		assert.Contains(err.Error(), "list models")
	}
}

func TestRegistryGetModelIncludeExclude(t *testing.T) {
	assert := assert.New(t)

	r := New()
	r.providers["ollama"] = provider{client: &registryModelClient{
		name: "ollama",
		models: []schema.Model{
			{Name: "llama3.2"},
			{Name: "llama3.2-vision"},
		},
	}}

	model, err := r.GetModel(context.Background(), registryProvider("ollama", []string{"^llama"}, []string{"vision$"}), "llama3.2")
	if !assert.NoError(err) {
		return
	}
	assert.Equal("llama3.2", model.Name)
	assert.Equal("ollama", model.OwnedBy)

	_, err = r.GetModel(context.Background(), registryProvider("ollama", []string{"^llama"}, []string{"vision$"}), "llama3.2-vision")
	if assert.Error(err) {
		assert.ErrorIs(err, schema.ErrNotFound)
	}
}

func TestRegistryGetModelOverwritesOwnedBy(t *testing.T) {
	assert := assert.New(t)

	r := New()
	r.providers["proxy"] = provider{client: &registryModelClient{
		name:   "proxy",
		models: []schema.Model{{Name: "claude-sonnet", OwnedBy: "anthropic"}},
	}}

	model, err := r.GetModel(context.Background(), registryProvider("proxy", nil, nil), "claude-sonnet")
	if !assert.NoError(err) {
		return
	}
	assert.Equal("proxy", model.OwnedBy)
}

func TestRegistryGetModelWrapsProviderError(t *testing.T) {
	assert := assert.New(t)

	r := New()
	r.providers["google-prod"] = provider{client: &registryModelClient{
		name: "google",
		err:  schema.ErrBadParameter.With("unexpected model name format"),
	}}

	_, err := r.GetModel(context.Background(), &schema.Provider{
		Name:     "google-prod",
		Provider: schema.Gemini,
	}, "x/flux2-klein:latest")
	if assert.Error(err) {
		assert.ErrorIs(err, schema.ErrBadParameter)
		assert.Contains(err.Error(), "google-prod")
		assert.Contains(err.Error(), schema.Gemini)
		assert.Contains(err.Error(), `get model "x/flux2-klein:latest"`)
	}
}

func TestRegistryGetModelFallsBackToListedModel(t *testing.T) {
	assert := assert.New(t)

	r := New()
	r.providers["ollama"] = provider{client: &registryModelClient{
		name: "ollama",
		models: []schema.Model{
			{Name: "lfm2:latest"},
		},
		getErr: schema.ErrNotFound.With("show not supported"),
	}}

	model, err := r.GetModel(context.Background(), &schema.Provider{Name: "ollama", Provider: schema.Ollama}, "lfm2:latest")
	if !assert.NoError(err) {
		return
	}
	assert.Equal("lfm2:latest", model.Name)
	assert.Equal("ollama", model.OwnedBy)
}

func TestRegistrySetSkipsUnchangedModifiedAtValue(t *testing.T) {
	assert := assert.New(t)

	r := New()
	modifiedAt := time.Unix(1710000000, 0).UTC()
	first := &schema.Provider{
		Name:       "eliza",
		Provider:   schema.Eliza,
		ModifiedAt: &modifiedAt,
	}

	updated, deleted, err := r.Set(first, schema.ProviderCredentials{})
	if !assert.NoError(err) {
		return
	}
	assert.True(updated)
	assert.False(deleted)

	// Simulate a fresh DB scan returning the same timestamp value with a new pointer.
	modifiedAtCopy := modifiedAt
	second := &schema.Provider{
		Name:       "eliza",
		Provider:   schema.Eliza,
		ModifiedAt: &modifiedAtCopy,
	}

	updated, deleted, err = r.Set(second, schema.ProviderCredentials{})
	if !assert.NoError(err) {
		return
	}
	assert.False(updated)
	assert.False(deleted)
	assert.NotNil(r.Get("eliza"))
}

func TestRegistrySetUpdatesWhenModifiedAtChanges(t *testing.T) {
	assert := assert.New(t)

	r := New()
	firstTime := time.Unix(1710000000, 0).UTC()
	secondTime := firstTime.Add(time.Minute)

	updated, deleted, err := r.Set(&schema.Provider{
		Name:       "eliza",
		Provider:   schema.Eliza,
		ModifiedAt: &firstTime,
	}, schema.ProviderCredentials{})
	if !assert.NoError(err) {
		return
	}
	assert.True(updated)
	assert.False(deleted)

	updated, deleted, err = r.Set(&schema.Provider{
		Name:       "eliza",
		Provider:   schema.Eliza,
		ModifiedAt: &secondTime,
	}, schema.ProviderCredentials{})
	if !assert.NoError(err) {
		return
	}
	assert.True(updated)
	assert.False(deleted)
}
