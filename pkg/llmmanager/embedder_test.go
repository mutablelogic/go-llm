package manager

import (
	"context"
	"testing"

	// Packages
	auth "github.com/djthorpe/go-auth/schema/auth"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	llmtest "github.com/mutablelogic/go-llm/pkg/test"
	assert "github.com/stretchr/testify/assert"
)

func TestEmbeddingRequiresInput(t *testing.T) {
	_, m := newIntegrationManager(t)

	_, err := m.Embedding(context.Background(), schema.EmbeddingRequest{Model: "ignored"}, nil)
	if assert.Error(t, err) {
		assert.ErrorIs(t, err, schema.ErrBadParameter)
	}
}

func TestEmbeddingRespectsProviderGroupsIntegration(t *testing.T) {
	conn, m := newIntegrationManager(t)
	conn.RequireProvider(t)
	ctx := llmtest.Context(t)
	provider := llmtest.CreateProvider(t, conn.ProviderInsert(), m.CreateProvider, m.SyncProviders)
	admin := llmtest.AdminUser(conn)
	modelName := integrationEmbeddingModelName(t, m, provider.Name, admin)

	t.Run("denies user without groups", func(t *testing.T) {
		assert := assert.New(t)
		_, err := m.Embedding(ctx, schema.EmbeddingRequest{
			Model: modelName,
			Input: []string{"hello world"},
		}, &auth.User{})
		if assert.Error(err) {
			assert.ErrorIs(err, schema.ErrNotFound)
		}
	})

	t.Run("allows matching group", func(t *testing.T) {
		assert := assert.New(t)
		resp, err := m.Embedding(ctx, schema.EmbeddingRequest{
			Model: modelName,
			Input: []string{"hello world"},
		}, admin)
		if llmtest.IsUnreachable(err) {
			t.Skipf("provider unreachable: %v", err)
		}
		if !assert.NoError(err) {
			return
		}
		assert.NotNil(resp)
		assert.Equal(provider.Name, resp.Provider)
		assert.Equal(modelName, resp.Model)
		assert.Equal(schema.EmbeddingTaskTypeDefault, resp.TaskType)
		assert.Len(resp.Output, 1)
		assert.NotEmpty(resp.Output[0])
	})
}

func integrationEmbeddingModelName(t *testing.T, m *Manager, provider string, user *auth.User) string {
	t.Helper()
	ctx := llmtest.Context(t)

	models, err := m.ListModels(ctx, schema.ModelListRequest{Provider: provider}, user)
	if llmtest.IsUnreachable(err) {
		t.Skipf("provider unreachable: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, model := range models.Body {
		if model.Cap&schema.ModelCapEmbeddings != 0 {
			return model.Name
		}
	}
	t.Skip("no embedding-capable model available, skipping")
	return ""
}
