package google_test

import (
	"context"
	"os"
	"strings"
	"testing"

	// Packages
	google "github.com/mutablelogic/go-llm/pkg/google"
)

func Test_embedding_001(t *testing.T) {
	client := getClient(t)
	ctx := context.Background()

	// Test single embedding
	vector, err := client.Embedding(ctx, "text-embedding-004", "Hello, world!")
	if err != nil {
		if strings.Contains(err.Error(), "429") {
			t.Skip("Rate limited")
		}
		t.Fatal(err)
	}

	// Check that we got a non-empty vector
	if len(vector) == 0 {
		t.Error("Expected non-empty embedding vector")
	}

	t.Logf("Got embedding vector with %d dimensions", len(vector))
}

func Test_embedding_002(t *testing.T) {
	client := getClient(t)
	ctx := context.Background()

	// Test batch embedding
	texts := []string{
		"Hello, world!",
		"How are you?",
		"The quick brown fox jumps over the lazy dog.",
	}

	vectors, err := client.BatchEmbedding(ctx, "text-embedding-004", texts)
	if err != nil {
		if strings.Contains(err.Error(), "429") {
			t.Skip("Rate limited")
		}
		t.Fatal(err)
	}

	// Check that we got the right number of vectors
	if len(vectors) != len(texts) {
		t.Errorf("Expected %d vectors, got %d", len(texts), len(vectors))
	}

	// Check that each vector is non-empty
	for i, vector := range vectors {
		if len(vector) == 0 {
			t.Errorf("Expected non-empty embedding vector for text %d", i)
		}
		t.Logf("Vector %d has %d dimensions", i, len(vector))
	}
}

func Test_embedding_003(t *testing.T) {
	client := getClient(t)
	ctx := context.Background()

	// Test embedding with task type
	vector, err := client.Embedding(ctx, "text-embedding-004", "What is the meaning of life?",
		google.WithTaskType(google.TaskTypeRetrievalQuery),
	)
	if err != nil {
		if strings.Contains(err.Error(), "429") {
			t.Skip("Rate limited")
		}
		t.Fatal(err)
	}

	if len(vector) == 0 {
		t.Error("Expected non-empty embedding vector")
	}

	t.Logf("Got embedding vector with %d dimensions (task type: RETRIEVAL_QUERY)", len(vector))
}

func Test_embedding_004(t *testing.T) {
	client := getClient(t)
	ctx := context.Background()

	// Test embedding with output dimensionality
	vector, err := client.Embedding(ctx, "text-embedding-004", "Hello, world!",
		google.WithOutputDimensionality(256),
	)
	if err != nil {
		if strings.Contains(err.Error(), "429") {
			t.Skip("Rate limited")
		}
		t.Fatal(err)
	}

	// Check that we got exactly 256 dimensions
	if len(vector) != 256 {
		t.Errorf("Expected 256 dimensions, got %d", len(vector))
	}

	t.Logf("Got embedding vector with %d dimensions (reduced from default)", len(vector))
}

func Test_embedding_005(t *testing.T) {
	client := getClient(t)
	ctx := context.Background()

	// Test embedding with semantic similarity task type
	vector, err := client.Embedding(ctx, "text-embedding-004", "This is a test sentence.",
		google.WithTaskType(google.TaskTypeSemantic),
	)
	if err != nil {
		if strings.Contains(err.Error(), "429") {
			t.Skip("Rate limited")
		}
		t.Fatal(err)
	}

	if len(vector) == 0 {
		t.Error("Expected non-empty embedding vector")
	}

	t.Logf("Got embedding vector with %d dimensions (task type: SEMANTIC_SIMILARITY)", len(vector))
}

func Test_embedding_006(t *testing.T) {
	client := getClient(t)
	ctx := context.Background()

	// Test batch embedding with title option (for document retrieval)
	texts := []string{
		"First sentence.",
		"Second sentence.",
	}

	vectors, err := client.BatchEmbedding(ctx, "text-embedding-004", texts,
		google.WithTaskType(google.TaskTypeRetrievalDocument),
	)
	if err != nil {
		if strings.Contains(err.Error(), "429") {
			t.Skip("Rate limited")
		}
		t.Fatal(err)
	}

	if len(vectors) != len(texts) {
		t.Errorf("Expected %d vectors, got %d", len(texts), len(vectors))
	}

	t.Logf("Got %d embedding vectors for document retrieval", len(vectors))
}

// getClient creates a new Google client for testing
func getClient(t *testing.T) *google.Client {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set")
	}

	client, err := google.New(apiKey)
	if err != nil {
		t.Fatal(err)
	}

	return client
}
