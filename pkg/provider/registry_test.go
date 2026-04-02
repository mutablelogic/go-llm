package provider

import (
	"context"
	"testing"

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

func (c *registryModelClient) GetModel(context.Context, string, ...opt.Opt) (*schema.Model, error) {
	return nil, nil
}

func TestRegistryGetModelsInvalidPattern(t *testing.T) {
	assert := assert.New(t)

	r := New()
	r.providers["ollama"] = provider{client: &registryModelClient{name: "ollama"}}

	_, err := r.GetModels(context.Background(), "ollama", []string{"["}, nil)
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

	models, err := r.GetModels(context.Background(), "ollama", []string{"^llama", "^mistral"}, []string{"vision$"})
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

func TestRegistryGetModelsPreservesOwnedBy(t *testing.T) {
	assert := assert.New(t)

	r := New()
	r.providers["proxy"] = provider{client: &registryModelClient{
		name: "proxy",
		models: []schema.Model{{Name: "claude-sonnet", OwnedBy: "anthropic"}},
	}}

	models, err := r.GetModels(context.Background(), "proxy", nil, nil)
	if !assert.NoError(err) {
		return
	}

	if assert.Len(models, 1) {
		assert.Equal("anthropic", models[0].OwnedBy)
	}
}