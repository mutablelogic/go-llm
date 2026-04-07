package manager

import (
	"context"
	"strings"
	"testing"

	// Packages
	auth "github.com/djthorpe/go-auth/schema/auth"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
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
	return llmtest.ModelNameMatching(
		t,
		"",
		syncAndListModels(m, provider, user),
		func(model schema.Model) bool {
			return model.Cap&schema.ModelCapEmbeddings != 0
		},
		func(ctx context.Context, name string) error {
			if err := validateAccessibleModel(m, provider, user)(ctx, name); err != nil {
				return err
			}
			_, err := m.Embedding(ctx, schema.EmbeddingRequest{
				Provider: provider,
				Model:    name,
				Input:    []string{"hello world"},
			}, user)
			if err != nil && strings.Contains(err.Error(), "does not support embeddings") {
				return err
			}
			return err
		},
	)
}
