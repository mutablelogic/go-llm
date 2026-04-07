package manager

import (
	"context"
	"testing"

	// Packages
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	llmtest "github.com/mutablelogic/go-llm/pkg/test"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
)

func TestListMessagesIntegration(t *testing.T) {
	conn, m := newIntegrationManager(t)
	conn.RequireProvider(t)
	ctx := llmtest.Context(t)
	provider := llmtest.CreateProvider(t, conn.ProviderInsert(), m.CreateProvider, m.SyncProviders)
	admin := llmtest.AdminUser(conn)
	modelName := llmtest.ModelNameMatching(t, "", syncAndListModels(m, provider.Name, admin), func(model schema.Model) bool {
		return model.Cap&schema.ModelCapCompletion != 0
	}, validateAccessibleModel(m, provider.Name, admin))

	session, err := m.CreateSession(ctx, schema.SessionInsert{
		SessionMeta: schema.SessionMeta{
			GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr(modelName), Provider: types.Ptr(provider.Name)},
			Title:         types.Ptr("messages"),
		},
	}, admin)
	if !assert.NoError(t, err) {
		return
	}

	if err := m.PoolConn.Insert(ctx, nil, schema.MessageInsert{
		Session: session.ID,
		Message: schema.Message{Role: schema.RoleUser, Content: []schema.ContentBlock{{Text: types.Ptr("hello world")}}, Tokens: 2},
	}); !assert.NoError(t, err) {
		return
	}
	if err := m.PoolConn.Insert(ctx, nil, schema.MessageInsert{
		Session: session.ID,
		Message: schema.Message{Role: schema.RoleAssistant, Content: []schema.ContentBlock{{Text: types.Ptr("daily news summary")}}, Tokens: 4, Result: schema.ResultStop},
	}); !assert.NoError(t, err) {
		return
	}

	result, err := m.ListMessages(ctx, session.ID, schema.MessageListRequest{}, admin)
	if !assert.NoError(t, err) {
		return
	}

	if assert.Equal(t, uint(2), result.Count) && assert.Len(t, result.Body, 2) {
		assert.Equal(t, schema.RoleUser, result.Body[0].Role)
		assert.Equal(t, schema.RoleAssistant, result.Body[1].Role)
	}

	filtered, err := m.ListMessages(ctx, session.ID, schema.MessageListRequest{Role: schema.RoleAssistant, Text: "news"}, admin)
	if !assert.NoError(t, err) {
		return
	}

	if assert.Equal(t, uint(1), filtered.Count) && assert.Len(t, filtered.Body, 1) {
		assert.Equal(t, schema.RoleAssistant, filtered.Body[0].Role)
		assert.Equal(t, "daily news summary", filtered.Body[0].Text())
	}
}

func TestListMessagesRejectsInaccessibleSession(t *testing.T) {
	conn, m := newIntegrationManager(t)
	conn.RequireProvider(t)
	ctx := llmtest.Context(t)
	provider := llmtest.CreateProvider(t, conn.ProviderInsert(), m.CreateProvider, m.SyncProviders)
	owner := llmtest.AdminUser(conn)
	other := llmtest.User(conn)
	modelName := llmtest.ModelNameMatching(t, "", syncAndListModels(m, provider.Name, owner), func(model schema.Model) bool {
		return model.Cap&schema.ModelCapCompletion != 0
	}, validateAccessibleModel(m, provider.Name, owner))

	session, err := m.CreateSession(context.Background(), schema.SessionInsert{
		SessionMeta: schema.SessionMeta{
			GeneratorMeta: schema.GeneratorMeta{Model: types.Ptr(modelName), Provider: types.Ptr(provider.Name)},
		},
	}, owner)
	if !assert.NoError(t, err) {
		return
	}

	_, err = m.ListMessages(ctx, session.ID, schema.MessageListRequest{}, other)
	if assert.Error(t, err) {
		assert.ErrorIs(t, err, schema.ErrNotFound)
	}
}
