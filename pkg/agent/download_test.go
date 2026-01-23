package agent

import (
	"context"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	"github.com/stretchr/testify/assert"
)

///////////////////////////////////////////////////////////////////////////////
// MOCK CLIENT

// mockClient is a basic client that doesn't support downloading
type mockClient struct {
	name string
}

func (m *mockClient) Name() string {
	return m.name
}

func (m *mockClient) ListModels(ctx context.Context) ([]schema.Model, error) {
	return nil, nil
}

func (m *mockClient) GetModel(ctx context.Context, name string) (*schema.Model, error) {
	return nil, llm.ErrNotFound
}

// mockDownloader is a client that supports downloading
type mockDownloader struct {
	mockClient
	downloadCalled bool
	deleteCalled   bool
	downloadPath   string
	deleteModel    schema.Model
}

func (m *mockDownloader) DownloadModel(ctx context.Context, path string, opts ...opt.Opt) (*schema.Model, error) {
	m.downloadCalled = true
	m.downloadPath = path
	return &schema.Model{
		Name:    path,
		OwnedBy: m.name,
	}, nil
}

func (m *mockDownloader) DeleteModel(ctx context.Context, model schema.Model) error {
	m.deleteCalled = true
	m.deleteModel = model
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// TESTS

func TestDownloadModel_InvalidPath(t *testing.T) {
	// Create agent
	agent, err := NewAgent()
	assert.NoError(t, err)

	// Try invalid path formats
	testCases := []string{
		"invalid",           // no colon
		"provider:",         // empty model
		":model",            // empty provider
		"",                  // empty string
		"provider:  ",       // whitespace model
		"  :model",          // whitespace provider
	}

	for _, path := range testCases {
		_, err = agent.DownloadModel(context.Background(), path)
		assert.Error(t, err, "path: %q", path)
		assert.ErrorIs(t, err, llm.ErrBadParameter, "path: %q", path)
	}
}

func TestDownloadModel_NotFound(t *testing.T) {
	// Create agent with no clients
	agent, err := NewAgent()
	assert.NoError(t, err)

	// Try to download from non-existent provider
	_, err = agent.DownloadModel(context.Background(), "nonexistent:model-name")
	assert.Error(t, err)
	assert.ErrorIs(t, err, llm.ErrNotFound)
}

func TestDownloadModel_NotImplemented(t *testing.T) {
	// Create agent with client that doesn't support downloading
	mockClient := &mockClient{name: "test"}
	agent, err := NewAgent(WithClient(mockClient))
	assert.NoError(t, err)

	// Try to download from provider that doesn't support it
	_, err = agent.DownloadModel(context.Background(), "test:model-name")
	assert.Error(t, err)
	assert.ErrorIs(t, err, llm.ErrNotImplemented)
}

func TestDownloadModel_Success(t *testing.T) {
	// Create agent with downloader client
	mockDownloader := &mockDownloader{
		mockClient: mockClient{name: "ollama"},
	}
	agent, err := NewAgent(WithClient(mockDownloader))
	assert.NoError(t, err)

	// Download model
	model, err := agent.DownloadModel(context.Background(), "ollama:llama2")
	assert.NoError(t, err)
	assert.NotNil(t, model)
	assert.Equal(t, "llama2", model.Name)
	assert.Equal(t, "ollama", model.OwnedBy)
	assert.True(t, mockDownloader.downloadCalled)
	assert.Equal(t, "llama2", mockDownloader.downloadPath)
}

func TestDownloadModel_WithVersion(t *testing.T) {
	// Create agent with downloader client
	mockDownloader := &mockDownloader{
		mockClient: mockClient{name: "ollama"},
	}
	agent, err := NewAgent(WithClient(mockDownloader))
	assert.NoError(t, err)

	// Download model with version tag
	model, err := agent.DownloadModel(context.Background(), "ollama:llama2:7b")
	assert.NoError(t, err)
	assert.NotNil(t, model)
	assert.True(t, mockDownloader.downloadCalled)
	assert.Equal(t, "llama2:7b", mockDownloader.downloadPath)
}

func TestDeleteModel_NotFound(t *testing.T) {
	// Create agent with no clients
	agent, err := NewAgent()
	assert.NoError(t, err)

	// Try to delete model with no owner
	model := schema.Model{Name: "test-model"}
	err = agent.DeleteModel(context.Background(), model)
	assert.Error(t, err)
	assert.ErrorIs(t, err, llm.ErrNotFound)
}

func TestDeleteModel_NotImplemented(t *testing.T) {
	// Create agent with client that doesn't support downloading
	mockClient := &mockClient{name: "test"}
	agent, err := NewAgent(WithClient(mockClient))
	assert.NoError(t, err)

	// Try to delete model from provider that doesn't support it
	model := schema.Model{Name: "test-model", OwnedBy: "test"}
	err = agent.DeleteModel(context.Background(), model)
	assert.Error(t, err)
	assert.ErrorIs(t, err, llm.ErrNotImplemented)
}

func TestDeleteModel_Success(t *testing.T) {
	// Create agent with downloader client
	mockDownloader := &mockDownloader{
		mockClient: mockClient{name: "ollama"},
	}
	agent, err := NewAgent(WithClient(mockDownloader))
	assert.NoError(t, err)

	// Delete model
	model := schema.Model{Name: "llama2", OwnedBy: "ollama"}
	err = agent.DeleteModel(context.Background(), model)
	assert.NoError(t, err)
	assert.True(t, mockDownloader.deleteCalled)
	assert.Equal(t, "llama2", mockDownloader.deleteModel.Name)
}
