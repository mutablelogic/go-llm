package agent

import (
	"context"
	"os"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	"github.com/mutablelogic/go-llm/pkg/provider/anthropic"
	"github.com/mutablelogic/go-llm/pkg/provider/google"
	"github.com/mutablelogic/go-llm/pkg/provider/mistral"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	assert "github.com/stretchr/testify/assert"
)

///////////////////////////////////////////////////////////////////////////////
// HELPERS

// newManager creates a Manager with real clients based on environment variables.
// Skips the test if no API keys are set.
func newManager(t *testing.T) *Manager {
	t.Helper()

	var opts []Opt
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		client, err := anthropic.New(key)
		if err != nil {
			t.Fatal(err)
		}
		opts = append(opts, WithClient(client))
	}
	if key := os.Getenv("GEMINI_API_KEY"); key != "" {
		client, err := google.New(key)
		if err != nil {
			t.Fatal(err)
		}
		opts = append(opts, WithClient(client))
	}
	if key := os.Getenv("MISTRAL_API_KEY"); key != "" {
		client, err := mistral.New(key)
		if err != nil {
			t.Fatal(err)
		}
		opts = append(opts, WithClient(client))
	}
	if len(opts) == 0 {
		t.Skip("No API keys set (ANTHROPIC_API_KEY, GEMINI_API_KEY, MISTRAL_API_KEY)")
	}

	m, err := NewManager(opts...)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

///////////////////////////////////////////////////////////////////////////////
// MANAGER LISTMODELS TESTS

// Test ListModels returns models
func Test_manager_listmodels_001(t *testing.T) {
	assert := assert.New(t)
	m := newManager(t)

	resp, err := m.ListModels(context.Background(), schema.ListModelsRequest{})
	assert.NoError(err)
	assert.NotNil(resp)
	assert.Greater(resp.Count, uint(0))
	assert.NotEmpty(resp.Body)
	assert.NotEmpty(resp.Provider)
	t.Logf("Total models: %d, Providers: %v", resp.Count, resp.Provider)
}

// Test ListModels results are sorted by name
func Test_manager_listmodels_002(t *testing.T) {
	assert := assert.New(t)
	m := newManager(t)

	resp, err := m.ListModels(context.Background(), schema.ListModelsRequest{})
	assert.NoError(err)
	for i := 1; i < len(resp.Body); i++ {
		assert.LessOrEqual(resp.Body[i-1].Name, resp.Body[i].Name, "models should be sorted by name")
	}
}

// Test ListModels with limit
func Test_manager_listmodels_003(t *testing.T) {
	assert := assert.New(t)
	m := newManager(t)

	resp, err := m.ListModels(context.Background(), schema.ListModelsRequest{Limit: 3})
	assert.NoError(err)
	assert.LessOrEqual(len(resp.Body), 3)
	assert.Greater(resp.Count, uint(0))
	t.Logf("Returned %d of %d models", len(resp.Body), resp.Count)
}

// Test ListModels with offset and limit for pagination
func Test_manager_listmodels_004(t *testing.T) {
	assert := assert.New(t)
	m := newManager(t)

	// Get first page
	page1, err := m.ListModels(context.Background(), schema.ListModelsRequest{Limit: 2})
	assert.NoError(err)
	assert.LessOrEqual(len(page1.Body), 2)

	// Get second page
	page2, err := m.ListModels(context.Background(), schema.ListModelsRequest{Offset: 2, Limit: 2})
	assert.NoError(err)
	assert.LessOrEqual(len(page2.Body), 2)

	// Pages should not overlap (if enough models exist)
	if len(page1.Body) > 0 && len(page2.Body) > 0 {
		assert.NotEqual(page1.Body[0].Name, page2.Body[0].Name, "pages should not overlap")
	}
}

// Test ListModels filters by provider
func Test_manager_listmodels_005(t *testing.T) {
	assert := assert.New(t)
	m := newManager(t)

	// Get all to find a valid provider name
	all, err := m.ListModels(context.Background(), schema.ListModelsRequest{})
	assert.NoError(err)
	assert.NotEmpty(all.Provider)

	provider := all.Provider[0]
	filtered, err := m.ListModels(context.Background(), schema.ListModelsRequest{Provider: provider})
	assert.NoError(err)
	assert.Greater(filtered.Count, uint(0))
	assert.LessOrEqual(filtered.Count, all.Count)
	t.Logf("Provider %q: %d models", provider, filtered.Count)
}

// Test ListModels with nonexistent provider returns empty
func Test_manager_listmodels_006(t *testing.T) {
	assert := assert.New(t)
	m := newManager(t)

	resp, err := m.ListModels(context.Background(), schema.ListModelsRequest{Provider: "nonexistent-provider"})
	assert.NoError(err)
	assert.Equal(uint(0), resp.Count)
	assert.Empty(resp.Body)
}

// Test ListModels with offset beyond total returns empty body
func Test_manager_listmodels_007(t *testing.T) {
	assert := assert.New(t)
	m := newManager(t)

	resp, err := m.ListModels(context.Background(), schema.ListModelsRequest{Offset: 99999})
	assert.NoError(err)
	assert.Greater(resp.Count, uint(0))
	assert.Empty(resp.Body)
}

///////////////////////////////////////////////////////////////////////////////
// MANAGER GETMODEL TESTS

// Test getModel finds a model without specifying provider
func Test_manager_getmodel_001(t *testing.T) {
	assert := assert.New(t)
	m := newManager(t)

	// Get a model name to search for
	resp, err := m.ListModels(context.Background(), schema.ListModelsRequest{Limit: 1})
	assert.NoError(err)
	assert.NotEmpty(resp.Body)

	modelName := resp.Body[0].Name
	model, err := m.getModel(context.Background(), "", modelName)
	assert.NoError(err)
	assert.NotNil(model)
	assert.Equal(modelName, model.Name)
	t.Logf("Found model: %s (provider: %s)", model.Name, model.OwnedBy)
}

// Test getModel returns not found for nonexistent model
func Test_manager_getmodel_002(t *testing.T) {
	assert := assert.New(t)
	m := newManager(t)

	_, err := m.getModel(context.Background(), "", "nonexistent-model-xyz")
	assert.ErrorIs(err, llm.ErrNotFound)
}

// Test getModel with specific provider
func Test_manager_getmodel_003(t *testing.T) {
	assert := assert.New(t)
	m := newManager(t)

	// Get a model with its provider
	resp, err := m.ListModels(context.Background(), schema.ListModelsRequest{Limit: 1})
	assert.NoError(err)
	assert.NotEmpty(resp.Body)

	modelName := resp.Body[0].Name
	provider := resp.Body[0].OwnedBy
	model, err := m.getModel(context.Background(), provider, modelName)
	assert.NoError(err)
	assert.NotNil(model)
	assert.Equal(modelName, model.Name)
}

// Test getModel with unknown provider returns not found
func Test_manager_getmodel_004(t *testing.T) {
	assert := assert.New(t)
	m := newManager(t)

	_, err := m.getModel(context.Background(), "unknown-provider", "any-model")
	assert.ErrorIs(err, llm.ErrNotFound)
}

///////////////////////////////////////////////////////////////////////////////
// EMBEDDING INTEGRATION TESTS

// newGoogleManager creates a Manager with only the Google client.
func newGoogleManager(t *testing.T) *Manager {
	t.Helper()
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		t.Skip("GEMINI_API_KEY not set")
	}
	client, err := google.New(key)
	if err != nil {
		t.Fatal(err)
	}
	m, err := NewManager(WithClient(client))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// Test single embedding with Google
func Test_manager_embedding_001(t *testing.T) {
	assert := assert.New(t)
	m := newGoogleManager(t)

	resp, err := m.Embedding(context.Background(), &schema.EmbeddingRequest{
		Provider: "gemini",
		Model:    "gemini-embedding-001",
		Input:    []string{"Hello, world!"},
	})
	assert.NoError(err)
	if !assert.NotNil(resp) {
		return
	}
	assert.Equal("gemini", resp.Provider)
	assert.Equal("gemini-embedding-001", resp.Model)
	assert.Len(resp.Output, 1)
	assert.NotEmpty(resp.Output[0])
	assert.Greater(resp.OutputDimensionality, uint(0))
	t.Logf("Embedding dimensions: %d", resp.OutputDimensionality)
}

// Test batch embedding with Google
func Test_manager_embedding_002(t *testing.T) {
	assert := assert.New(t)
	m := newGoogleManager(t)

	inputs := []string{"The cat sat on the mat", "The dog chased the ball", "Machine learning is fascinating"}
	resp, err := m.Embedding(context.Background(), &schema.EmbeddingRequest{
		Provider: "gemini",
		Model:    "gemini-embedding-001",
		Input:    inputs,
	})
	assert.NoError(err)
	if !assert.NotNil(resp) {
		return
	}
	assert.Len(resp.Output, 3)
	for i, emb := range resp.Output {
		assert.NotEmpty(emb, "embedding %d should not be empty", i)
	}
	// All embeddings should be the same dimensionality
	assert.Equal(len(resp.Output[0]), len(resp.Output[1]))
	assert.Equal(len(resp.Output[1]), len(resp.Output[2]))
	t.Logf("Batch embedding: %d inputs, %d dimensions each", len(resp.Output), len(resp.Output[0]))
}

// Test embedding with custom task type
func Test_manager_embedding_003(t *testing.T) {
	assert := assert.New(t)
	m := newGoogleManager(t)

	resp, err := m.Embedding(context.Background(), &schema.EmbeddingRequest{
		Provider: "gemini",
		Model:    "gemini-embedding-001",
		Input:    []string{"What is the capital of France?"},
		TaskType: "RETRIEVAL_QUERY",
	})
	assert.NoError(err)
	if !assert.NotNil(resp) {
		return
	}
	assert.Equal("RETRIEVAL_QUERY", resp.TaskType)
	assert.NotEmpty(resp.Output[0])
}

// Test embedding with reduced output dimensionality
func Test_manager_embedding_004(t *testing.T) {
	assert := assert.New(t)
	m := newGoogleManager(t)

	resp, err := m.Embedding(context.Background(), &schema.EmbeddingRequest{
		Provider:             "gemini",
		Model:                "gemini-embedding-001",
		Input:                []string{"Dimensionality reduction test"},
		OutputDimensionality: 128,
	})
	assert.NoError(err)
	if !assert.NotNil(resp) {
		return
	}
	assert.Len(resp.Output, 1)
	assert.Equal(128, len(resp.Output[0]))
	t.Logf("Reduced dimensionality: %d", len(resp.Output[0]))
}

// Test embedding with RETRIEVAL_DOCUMENT task type and title
func Test_manager_embedding_005(t *testing.T) {
	assert := assert.New(t)
	m := newGoogleManager(t)

	resp, err := m.Embedding(context.Background(), &schema.EmbeddingRequest{
		Provider: "gemini",
		Model:    "gemini-embedding-001",
		Input:    []string{"Paris is the capital and largest city of France."},
		TaskType: "RETRIEVAL_DOCUMENT",
		Title:    "France Geography",
	})
	assert.NoError(err)
	if !assert.NotNil(resp) {
		return
	}
	assert.Equal("RETRIEVAL_DOCUMENT", resp.TaskType)
	assert.Equal("France Geography", resp.Title)
	assert.NotEmpty(resp.Output[0])
}
