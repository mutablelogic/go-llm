package google

import (
	"context"
	"os"
	"testing"

	// Packages
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	assert "github.com/stretchr/testify/assert"
)

///////////////////////////////////////////////////////////////////////////////
// UNIT TESTS

func Test_applyEmbedOpts_001(t *testing.T) {
	// Test with no options leaves request unchanged
	assert := assert.New(t)
	o, err := opt.Apply()
	assert.NoError(err)

	req := &geminiEmbedRequest{}
	applyEmbedOpts(req, o)
	assert.Empty(req.TaskType)
	assert.Empty(req.Title)
	assert.Zero(req.OutputDimensionality)
}

func Test_applyEmbedOpts_002(t *testing.T) {
	// Test with task type
	assert := assert.New(t)
	o, err := opt.Apply(WithTaskType("RETRIEVAL_QUERY"))
	assert.NoError(err)

	req := &geminiEmbedRequest{}
	applyEmbedOpts(req, o)
	assert.Equal("RETRIEVAL_QUERY", req.TaskType)
}

func Test_applyEmbedOpts_003(t *testing.T) {
	// Test with output dimensionality
	assert := assert.New(t)
	o, err := opt.Apply(WithOutputDimensionality(256))
	assert.NoError(err)

	req := &geminiEmbedRequest{}
	applyEmbedOpts(req, o)
	assert.Equal(256, req.OutputDimensionality)
}

func Test_applyEmbedOpts_004(t *testing.T) {
	// Test with title
	assert := assert.New(t)
	o, err := opt.Apply(WithTitle("My Document"))
	assert.NoError(err)

	req := &geminiEmbedRequest{}
	applyEmbedOpts(req, o)
	assert.Equal("My Document", req.Title)
}

func Test_applyEmbedOpts_005(t *testing.T) {
	// Test with all options combined
	assert := assert.New(t)
	o, err := opt.Apply(
		WithTaskType("RETRIEVAL_DOCUMENT"),
		WithTitle("My Document"),
		WithOutputDimensionality(512),
	)
	assert.NoError(err)

	req := &geminiEmbedRequest{}
	applyEmbedOpts(req, o)
	assert.Equal("RETRIEVAL_DOCUMENT", req.TaskType)
	assert.Equal("My Document", req.Title)
	assert.Equal(512, req.OutputDimensionality)
}

///////////////////////////////////////////////////////////////////////////////
// INTEGRATION TESTS

func Test_embedding_001(t *testing.T) {
	// Test single embedding
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("GOOGLE_API_KEY")
	}
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set, skipping")
	}
	assert := assert.New(t)
	client, err := New(apiKey)
	assert.NoError(err)

	model := schema.Model{Name: "gemini-embedding-001"}
	vector, err := client.Embedding(context.TODO(), model, "Hello, world!")
	assert.NoError(err)
	assert.NotEmpty(vector)
	t.Logf("Got embedding vector with %d dimensions", len(vector))
}

func Test_embedding_002(t *testing.T) {
	// Test batch embedding
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("GOOGLE_API_KEY")
	}
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set, skipping")
	}
	assert := assert.New(t)
	client, err := New(apiKey)
	assert.NoError(err)

	model := schema.Model{Name: "gemini-embedding-001"}
	texts := []string{
		"Hello, world!",
		"How are you?",
		"The quick brown fox jumps over the lazy dog.",
	}
	vectors, err := client.BatchEmbedding(context.TODO(), model, texts)
	assert.NoError(err)
	assert.Len(vectors, len(texts))

	for i, v := range vectors {
		assert.NotEmpty(v)
		t.Logf("Vector %d has %d dimensions", i, len(v))
	}
}

func Test_embedding_003(t *testing.T) {
	// Test embedding with output dimensionality
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("GOOGLE_API_KEY")
	}
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set, skipping")
	}
	assert := assert.New(t)
	client, err := New(apiKey)
	assert.NoError(err)

	model := schema.Model{Name: "gemini-embedding-001"}
	vector, err := client.Embedding(context.TODO(), model, "Hello, world!",
		WithOutputDimensionality(256),
	)
	assert.NoError(err)
	assert.Len(vector, 256)
	t.Logf("Got embedding vector with %d dimensions", len(vector))
}

func Test_embedding_004(t *testing.T) {
	// Test batch embedding with empty input returns error
	assert := assert.New(t)
	client, err := New("test-key")
	assert.NoError(err)

	model := schema.Model{Name: "gemini-embedding-001"}
	_, err = client.BatchEmbedding(context.TODO(), model, []string{})
	assert.Error(err)
}

func Test_embedding_005(t *testing.T) {
	// Test embedding with task type
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("GOOGLE_API_KEY")
	}
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set, skipping")
	}
	assert := assert.New(t)
	client, err := New(apiKey)
	assert.NoError(err)

	model := schema.Model{Name: "gemini-embedding-001"}
	vector, err := client.Embedding(context.TODO(), model, "What is the meaning of life?",
		WithTaskType("RETRIEVAL_QUERY"),
	)
	assert.NoError(err)
	assert.NotEmpty(vector)
	t.Logf("Got embedding vector with %d dimensions (task type: RETRIEVAL_QUERY)", len(vector))
}
