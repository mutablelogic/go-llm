package manager

import (
	"context"
	"testing"

	// Packages
	uuid "github.com/google/uuid"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	llmtest "github.com/mutablelogic/go-llm/pkg/test"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
)

func TestCreateSessionRequiresUser(t *testing.T) {
	_, m := newIntegrationManager(t)

	_, err := m.CreateSession(context.Background(), schema.SessionInsert{
		SessionMeta: schema.SessionMeta{
			GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("ignored")},
		},
	}, nil)
	if assert.Error(t, err) {
		assert.ErrorIs(t, err, schema.ErrNotFound)
	}
}

func TestCreateSessionIntegration(t *testing.T) {
	conn, m := newIntegrationManager(t)
	conn.RequireProvider(t)
	ctx := llmtest.Context(t)
	provider := llmtest.CreateProvider(t, conn.ProviderInsert(), m.CreateProvider, m.SyncProviders)
	admin := llmtest.AdminUser(conn)
	modelName := llmtest.ModelNameMatching(t, "", syncAndListModels(m, provider.Name, admin), func(model schema.Model) bool {
		return model.Cap&schema.ModelCapCompletion != 0
	}, validateAccessibleModel(m, provider.Name, admin))

	created, err := m.CreateSession(ctx, schema.SessionInsert{
		SessionMeta: schema.SessionMeta{
			GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr(modelName)},
			Title:         types.Ptr("test session"),
			Tags:          []string{"integration"},
		},
	}, admin)
	if !assert.NoError(t, err) {
		return
	}

	assert.NotEqual(t, uuid.Nil, created.ID)
	assert.Equal(t, admin.UUID(), created.User)
	assert.Equal(t, uuid.Nil, created.Parent)
	assert.Equal(t, provider.Name, types.Value(created.Provider))
	assert.Equal(t, modelName, types.Value(created.Model))
	assert.Equal(t, "test session", types.Value(created.Title))
	assert.Equal(t, []string{"integration"}, created.Tags)
	assert.Zero(t, created.Input)
	assert.Zero(t, created.Output)
	assert.False(t, created.CreatedAt.IsZero())
	assert.Nil(t, created.ModifiedAt)
}

func TestCreateSessionMergesParentGeneratorMeta(t *testing.T) {
	conn, m := newIntegrationManager(t)
	conn.RequireProvider(t)
	ctx := llmtest.Context(t)
	provider := llmtest.CreateProvider(t, conn.ProviderInsert(), m.CreateProvider, m.SyncProviders)
	admin := llmtest.AdminUser(conn)
	modelName := llmtest.ModelNameMatching(t, "", syncAndListModels(m, provider.Name, admin), func(model schema.Model) bool {
		return model.Cap&schema.ModelCapCompletion != 0
	}, validateAccessibleModel(m, provider.Name, admin))
	thinking := true

	parent, err := m.CreateSession(ctx, schema.SessionInsert{
		SessionMeta: schema.SessionMeta{
			GeneratorMeta: schema.GeneratorMeta{
				Model:          types.Ptr(modelName),
				Provider:       types.Ptr(provider.Name),
				SystemPrompt:   types.Ptr("parent prompt"),
				Thinking:       &thinking,
				ThinkingBudget: types.Ptr(uint(99)),
			},
			Title: types.Ptr("parent"),
		},
	}, admin)
	if !assert.NoError(t, err) {
		return
	}

	child, err := m.CreateSession(ctx, schema.SessionInsert{
		Parent: parent.ID,
		SessionMeta: schema.SessionMeta{
			GeneratorMeta: schema.GeneratorMeta{
				SystemPrompt: types.Ptr("child prompt"),
			},
			Title: types.Ptr("child"),
		},
	}, admin)
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, parent.ID, child.Parent)
	assert.Equal(t, admin.UUID(), child.User)
	assert.Equal(t, modelName, types.Value(child.Model))
	assert.Equal(t, provider.Name, types.Value(child.Provider))
	assert.Equal(t, "child prompt", types.Value(child.SystemPrompt))
	if assert.NotNil(t, child.Thinking) {
		assert.True(t, *child.Thinking)
	}
	assert.Equal(t, uint(99), types.Value(child.ThinkingBudget))
}

func TestCreateSessionRejectsParentFromAnotherUser(t *testing.T) {
	conn, m := newIntegrationManager(t)
	conn.RequireProvider(t)
	ctx := llmtest.Context(t)
	provider := llmtest.CreateProvider(t, conn.ProviderInsert(), m.CreateProvider, m.SyncProviders)
	owner := llmtest.AdminUser(conn)
	other := llmtest.User(conn)
	modelName := llmtest.ModelNameMatching(t, "", syncAndListModels(m, provider.Name, owner), func(model schema.Model) bool {
		return model.Cap&schema.ModelCapCompletion != 0
	}, validateAccessibleModel(m, provider.Name, owner))

	parent, err := m.CreateSession(ctx, schema.SessionInsert{
		SessionMeta: schema.SessionMeta{
			GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr(modelName), Provider: types.Ptr(provider.Name)},
			Title:         types.Ptr("parent"),
		},
	}, owner)
	if !assert.NoError(t, err) {
		return
	}

	_, err = m.CreateSession(ctx, schema.SessionInsert{
		Parent: parent.ID,
		SessionMeta: schema.SessionMeta{
			GeneratorMeta: schema.GeneratorMeta{SystemPrompt: types.Ptr("child")},
			Title:         types.Ptr("child"),
		},
	}, other)
	if assert.Error(t, err) {
		assert.ErrorIs(t, err, httpresponse.ErrForbidden)
	}
}

func TestCreateSessionRejectsMissingParent(t *testing.T) {
	conn, m := newIntegrationManager(t)
	owner := llmtest.AdminUser(conn)

	_, err := m.CreateSession(context.Background(), schema.SessionInsert{
		Parent: uuid.New(),
		SessionMeta: schema.SessionMeta{
			GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr("ignored")},
		},
	}, owner)
	if assert.Error(t, err) {
		assert.ErrorIs(t, err, schema.ErrNotFound)
	}
}
