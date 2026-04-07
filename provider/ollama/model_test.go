package ollama_test

import (
	"context"
	"errors"
	"net"
	"testing"

	// Packages
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	ollama "github.com/mutablelogic/go-llm/provider/ollama"
	assert "github.com/stretchr/testify/assert"
)

///////////////////////////////////////////////////////////////////////////////
// HELPERS

// requireClient creates a client using OLLAMA_URL or skips the test.
func requireClient(t *testing.T) *ollama.Client {
	t.Helper()
	if ollamaURL == "" {
		t.Skip("OLLAMA_URL not set, skipping")
	}
	c, err := ollama.New(ollamaURL)
	if err != nil {
		t.Fatalf("ollama.New: %v", err)
	}
	return c
}

// skipIfUnreachable skips the test if err looks like a network connectivity failure.
func skipIfUnreachable(t *testing.T, err error) {
	t.Helper()
	var netErr *net.OpError
	if errors.As(err, &netErr) {
		t.Skipf("server unreachable: %v", err)
	}
}

// firstModel returns the name of the first available model or skips.
func firstModel(t *testing.T, c *ollama.Client) string {
	t.Helper()
	models, err := c.ListModels(context.Background())
	if err != nil {
		skipIfUnreachable(t, err)
		t.Fatalf("ListModels: %v", err)
	}
	if len(models) == 0 {
		t.Skip("no models available, skipping")
	}
	return models[0].Name
}

///////////////////////////////////////////////////////////////////////////////
// ListModels

func Test_ListModels_ReturnsModels(t *testing.T) {
	c := requireClient(t)
	assert := assert.New(t)

	models, err := c.ListModels(context.Background())
	skipIfUnreachable(t, err)
	assert.NoError(err)
	assert.NotEmpty(models)
	for _, m := range models {
		assert.NotEmpty(m.Name)
		assert.Equal(c.Name(), m.OwnedBy)
	}
}

func Test_ListModels_Cached(t *testing.T) {
	c := requireClient(t)
	assert := assert.New(t)

	models1, err := c.ListModels(context.Background())
	skipIfUnreachable(t, err)
	assert.NoError(err)

	models2, err := c.ListModels(context.Background())
	assert.NoError(err)
	assert.Equal(len(models1), len(models2))
}

///////////////////////////////////////////////////////////////////////////////
// ListRunningModels

func Test_ListRunningModels(t *testing.T) {
	c := requireClient(t)
	assert := assert.New(t)

	// May be empty if no models are currently loaded
	models, err := c.ListRunningModels(context.Background())
	skipIfUnreachable(t, err)
	assert.NoError(err)
	for _, m := range models {
		assert.NotEmpty(m.Name)
		assert.Equal(c.Name(), m.OwnedBy)
	}
}

///////////////////////////////////////////////////////////////////////////////
// GetModel

func Test_GetModel_KnownModel(t *testing.T) {
	c := requireClient(t)
	assert := assert.New(t)

	name := firstModel(t, c)
	model, err := c.GetModel(context.Background(), name)
	assert.NoError(err)
	if assert.NotNil(model) {
		assert.Equal(name, model.Name)
		assert.Equal(c.Name(), model.OwnedBy)
	}
}

func Test_GetModel_Cached(t *testing.T) {
	c := requireClient(t)
	assert := assert.New(t)

	name := firstModel(t, c)

	m1, err := c.GetModel(context.Background(), name)
	assert.NoError(err)
	assert.NotNil(m1)

	m2, err := c.GetModel(context.Background(), name)
	assert.NoError(err)
	assert.NotNil(m2)
	assert.Equal(m1.Name, m2.Name)
}

func Test_GetModel_NotFound(t *testing.T) {
	c := requireClient(t)
	assert := assert.New(t)

	_, err := c.GetModel(context.Background(), "nonexistent-model-xyz:latest")
	assert.Error(err)
}

///////////////////////////////////////////////////////////////////////////////
// DeleteModel - ownership guard

func Test_DeleteModel_WrongOwner(t *testing.T) {
	c := requireClient(t)
	assert := assert.New(t)

	m, err := c.GetModel(context.Background(), firstModel(t, c))
	assert.NoError(err)
	if !assert.NotNil(m) {
		t.FailNow()
	}
	m.OwnedBy = "other-provider"
	err = c.DeleteModel(context.Background(), *m)
	assert.Error(err)
}

///////////////////////////////////////////////////////////////////////////////
// LoadModel / UnloadModel

func Test_LoadModel_WrongOwner(t *testing.T) {
	c := requireClient(t)
	assert := assert.New(t)

	m, err := c.GetModel(context.Background(), firstModel(t, c))
	assert.NoError(err)
	if !assert.NotNil(m) {
		t.FailNow()
	}
	m.OwnedBy = "other-provider"
	err = c.LoadModel(context.Background(), *m)
	assert.Error(err)
}

func Test_UnloadModel_WrongOwner(t *testing.T) {
	c := requireClient(t)
	assert := assert.New(t)

	m, err := c.GetModel(context.Background(), firstModel(t, c))
	assert.NoError(err)
	if !assert.NotNil(m) {
		t.FailNow()
	}
	m.OwnedBy = "other-provider"
	err = c.UnloadModel(context.Background(), *m)
	assert.Error(err)
}

func Test_LoadUnloadModel(t *testing.T) {
	c := requireClient(t)
	assert := assert.New(t)

	m, err := c.GetModel(context.Background(), firstModel(t, c))
	assert.NoError(err)
	if !assert.NotNil(m) {
		t.FailNow()
	}

	assert.NoError(c.LoadModel(context.Background(), *m))
	assert.NoError(c.UnloadModel(context.Background(), *m))
}

///////////////////////////////////////////////////////////////////////////////
// DownloadModel

func Test_DownloadModel_NoProgress(t *testing.T) {
	c := requireClient(t)
	assert := assert.New(t)

	name := firstModel(t, c)
	// Re-pulling an existing model is a fast no-op on the server side
	model, err := c.DownloadModel(context.Background(), name)
	assert.NoError(err)
	if assert.NotNil(model) {
		assert.Equal(name, model.Name)
		assert.Equal(c.Name(), model.OwnedBy)
	}
}

func Test_DownloadModel_WithProgress(t *testing.T) {
	c := requireClient(t)
	assert := assert.New(t)

	name := firstModel(t, c)
	var progressCalled bool
	model, err := c.DownloadModel(context.Background(), name,
		opt.WithProgress(func(status string, percent float64) {
			progressCalled = true
			assert.NotEmpty(status)
			assert.GreaterOrEqual(percent, float64(0))
			assert.LessOrEqual(percent, float64(100))
		}),
	)
	assert.NoError(err)
	assert.True(progressCalled, "progress callback should have been called")
	if assert.NotNil(model) {
		assert.Equal(name, model.Name)
	}
}
