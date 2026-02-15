package agent

import (
	"context"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	assert "github.com/stretchr/testify/assert"
)

///////////////////////////////////////////////////////////////////////////////
// MOCK TYPES

// mockClient implements llm.Client only (no Generator, no Embedder)
type mockClient struct {
	name   string
	models []schema.Model
}

func (c *mockClient) Name() string { return c.name }
func (c *mockClient) ListModels(_ context.Context, _ ...opt.Opt) ([]schema.Model, error) {
	return c.models, nil
}
func (c *mockClient) GetModel(_ context.Context, name string, _ ...opt.Opt) (*schema.Model, error) {
	for _, m := range c.models {
		if m.Name == name {
			return &m, nil
		}
	}
	return nil, llm.ErrNotFound
}

///////////////////////////////////////////////////////////////////////////////
// MODEL TESTS

// Test ListModels aggregates from all providers and sorts
func Test_model_001(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{
			{Name: "zulu", OwnedBy: "provider-1"},
			{Name: "alpha", OwnedBy: "provider-1"},
		}}),
		WithClient(&mockClient{name: "provider-2", models: []schema.Model{
			{Name: "bravo", OwnedBy: "provider-2"},
		}}),
	)
	assert.NoError(err)

	resp, err := m.ListModels(context.TODO(), schema.ListModelsRequest{})
	assert.NoError(err)
	assert.Equal(uint(3), resp.Count)
	assert.Len(resp.Body, 3)
	// Should be sorted
	assert.Equal("alpha", resp.Body[0].Name)
	assert.Equal("bravo", resp.Body[1].Name)
	assert.Equal("zulu", resp.Body[2].Name)
}

// Test ListModels with provider filter
func Test_model_002(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}}),
		WithClient(&mockClient{name: "provider-2", models: []schema.Model{{Name: "model-2", OwnedBy: "provider-2"}}}),
	)
	assert.NoError(err)

	resp, err := m.ListModels(context.TODO(), schema.ListModelsRequest{Provider: "provider-1"})
	assert.NoError(err)
	assert.Equal(uint(1), resp.Count)
	assert.Len(resp.Body, 1)
	assert.Equal("model-1", resp.Body[0].Name)
}

// Test ListModels with no matching provider returns error
func Test_model_003(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1"}}}),
	)
	assert.NoError(err)

	_, err = m.ListModels(context.TODO(), schema.ListModelsRequest{Provider: "nonexistent"})
	assert.Error(err)
}

// Test GetModel finds model across providers
func Test_model_004(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}}),
		WithClient(&mockClient{name: "provider-2", models: []schema.Model{{Name: "model-2", OwnedBy: "provider-2"}}}),
	)
	assert.NoError(err)

	model, err := m.GetModel(context.TODO(), schema.GetModelRequest{Name: "model-2"})
	assert.NoError(err)
	assert.NotNil(model)
	assert.Equal("model-2", model.Name)
	assert.Equal("provider-2", model.OwnedBy)
}

// Test GetModel returns not found for unknown model
func Test_model_005(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1"}}}),
	)
	assert.NoError(err)

	_, err = m.GetModel(context.TODO(), schema.GetModelRequest{Name: "nonexistent"})
	assert.ErrorIs(err, llm.ErrNotFound)
}

// Test GetModel with provider filter
func Test_model_006(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "shared", OwnedBy: "provider-1"}}}),
		WithClient(&mockClient{name: "provider-2", models: []schema.Model{{Name: "shared", OwnedBy: "provider-2"}}}),
	)
	assert.NoError(err)

	model, err := m.GetModel(context.TODO(), schema.GetModelRequest{Name: "shared", Provider: "provider-2"})
	assert.NoError(err)
	assert.Equal("provider-2", model.OwnedBy)
}

// Test GetModel with provider filter returns not found when provider doesn't have model
func Test_model_007(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}}),
		WithClient(&mockClient{name: "provider-2", models: []schema.Model{{Name: "model-2", OwnedBy: "provider-2"}}}),
	)
	assert.NoError(err)

	_, err = m.GetModel(context.TODO(), schema.GetModelRequest{Name: "model-1", Provider: "provider-2"})
	assert.ErrorIs(err, llm.ErrNotFound)
}
