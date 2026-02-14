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
		WithClient(&mockClient{name: "p1", models: []schema.Model{
			{Name: "zulu", OwnedBy: "p1"},
			{Name: "alpha", OwnedBy: "p1"},
		}}),
		WithClient(&mockClient{name: "p2", models: []schema.Model{
			{Name: "bravo", OwnedBy: "p2"},
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
		WithClient(&mockClient{name: "p1", models: []schema.Model{{Name: "m1", OwnedBy: "p1"}}}),
		WithClient(&mockClient{name: "p2", models: []schema.Model{{Name: "m2", OwnedBy: "p2"}}}),
	)
	assert.NoError(err)

	resp, err := m.ListModels(context.TODO(), schema.ListModelsRequest{Provider: "p1"})
	assert.NoError(err)
	assert.Equal(uint(1), resp.Count)
	assert.Len(resp.Body, 1)
	assert.Equal("m1", resp.Body[0].Name)
}

// Test ListModels with no matching provider returns empty
func Test_model_003(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "p1", models: []schema.Model{{Name: "m1"}}}),
	)
	assert.NoError(err)

	resp, err := m.ListModels(context.TODO(), schema.ListModelsRequest{Provider: "nonexistent"})
	assert.NoError(err)
	assert.Equal(uint(0), resp.Count)
	assert.Empty(resp.Body)
}

// Test GetModel finds model across providers
func Test_model_004(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "p1", models: []schema.Model{{Name: "m1", OwnedBy: "p1"}}}),
		WithClient(&mockClient{name: "p2", models: []schema.Model{{Name: "m2", OwnedBy: "p2"}}}),
	)
	assert.NoError(err)

	model, err := m.GetModel(context.TODO(), schema.GetModelRequest{Name: "m2"})
	assert.NoError(err)
	assert.NotNil(model)
	assert.Equal("m2", model.Name)
	assert.Equal("p2", model.OwnedBy)
}

// Test GetModel returns not found for unknown model
func Test_model_005(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "p1", models: []schema.Model{{Name: "m1"}}}),
	)
	assert.NoError(err)

	_, err = m.GetModel(context.TODO(), schema.GetModelRequest{Name: "nonexistent"})
	assert.ErrorIs(err, llm.ErrNotFound)
}

// Test GetModel with provider filter
func Test_model_006(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "p1", models: []schema.Model{{Name: "shared", OwnedBy: "p1"}}}),
		WithClient(&mockClient{name: "p2", models: []schema.Model{{Name: "shared", OwnedBy: "p2"}}}),
	)
	assert.NoError(err)

	model, err := m.GetModel(context.TODO(), schema.GetModelRequest{Name: "shared", Provider: "p2"})
	assert.NoError(err)
	assert.Equal("p2", model.OwnedBy)
}

// Test GetModel with provider filter returns not found when provider doesn't have model
func Test_model_007(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "p1", models: []schema.Model{{Name: "m1", OwnedBy: "p1"}}}),
		WithClient(&mockClient{name: "p2", models: []schema.Model{{Name: "m2", OwnedBy: "p2"}}}),
	)
	assert.NoError(err)

	_, err = m.GetModel(context.TODO(), schema.GetModelRequest{Name: "m1", Provider: "p2"})
	assert.ErrorIs(err, llm.ErrNotFound)
}
