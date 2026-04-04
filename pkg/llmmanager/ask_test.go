package manager

import (
	"context"
	"testing"

	// Packages
	auth "github.com/djthorpe/go-auth/schema/auth"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	ollama "github.com/mutablelogic/go-llm/pkg/provider/ollama"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	assert "github.com/stretchr/testify/assert"
)

func TestOllamaWithThinking(t *testing.T) {
	t.Run("chat enables boolean thinking", func(t *testing.T) {
		o, err := opt.Apply(ollama.WithThinking("chat"))
		if !assert.NoError(t, err) {
			return
		}
		assert.True(t, o.GetBool(opt.ThinkingKey))
		assert.Equal(t, "true", o.GetString(opt.ThinkingKey))
	})

	t.Run("chat budget maps to medium", func(t *testing.T) {
		o, err := opt.Apply(ollama.WithThinking("chat", 34))
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, "medium", o.GetString(opt.ThinkingKey))
	})

	t.Run("ask rejects thinking", func(t *testing.T) {
		_, err := opt.Apply(ollama.WithThinking("ask"))
		if assert.Error(t, err) {
			assert.ErrorIs(t, err, schema.ErrBadParameter)
			assert.Contains(t, err.Error(), "chat context")
		}
	})

	t.Run("generate rejects thinking", func(t *testing.T) {
		_, err := opt.Apply(ollama.WithThinking("generate", 34))
		if assert.Error(t, err) {
			assert.ErrorIs(t, err, schema.ErrBadParameter)
			assert.Contains(t, err.Error(), "chat context")
		}
	})
}

func TestGeneratorFromMetaSupportsOllamaSystemPromptIntegration(t *testing.T) {
	conn, m := newIntegrationManager(t)
	conn.RequireProvider(t)
	provider := createIntegrationProvider(t, m, conn.ProviderInsert())
	admin := integrationAdminUser(conn)
	modelName := integrationModelName(t, m, provider.Name, admin, conn.Config.Model)

	_, _, opts, err := m.generatorFromMeta(context.Background(), schema.GeneratorMeta{
		Provider:     provider.Name,
		Model:        modelName,
		SystemPrompt: "be concise",
	}, admin, generationContextAsk)
	if isIntegrationUnreachable(err) {
		t.Skipf("provider unreachable: %v", err)
	}
	if !assert.NoError(t, err) {
		return
	}

	applied, err := opt.Apply(opts...)
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, "be concise", applied.GetString(opt.SystemPromptKey))
}

func TestGeneratorFromMetaSupportsOllamaJSONOutputIntegration(t *testing.T) {
	conn, m := newIntegrationManager(t)
	conn.RequireProvider(t)
	provider := createIntegrationProvider(t, m, conn.ProviderInsert())
	admin := integrationAdminUser(conn)
	modelName := integrationModelName(t, m, provider.Name, admin, conn.Config.Model)
	rawSchema := schema.JSONSchema(`{"type":"object","properties":{"answer":{"type":"string"}}}`)

	_, _, opts, err := m.generatorFromMeta(context.Background(), schema.GeneratorMeta{
		Provider: provider.Name,
		Model:    modelName,
		Format:   rawSchema,
	}, admin, generationContextAsk)
	if isIntegrationUnreachable(err) {
		t.Skipf("provider unreachable: %v", err)
	}
	if !assert.NoError(t, err) {
		return
	}

	applied, err := opt.Apply(opts...)
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, string(rawSchema), applied.GetString(opt.JSONSchemaKey))
}

func TestGeneratorFromMetaRejectsElizaThinkingBudgetIntegration(t *testing.T) {
	conn, m := newIntegrationManager(t)
	insert := conn.ProviderInsert()
	insert.Name = "restricted-eliza"
	insert.Provider = schema.Eliza
	insert.URL = nil
	insert.APIKey = ""
	provider := createIntegrationProvider(t, m, insert)
	admin := integrationAdminUser(conn)
	modelName := integrationModelName(t, m, provider.Name, admin, "")

	_, _, opts, err := m.generatorFromMeta(context.Background(), schema.GeneratorMeta{
		Provider:       provider.Name,
		Model:          modelName,
		ThinkingBudget: 2048,
	}, admin, generationContextAsk)
	if !assert.NoError(t, err) {
		return
	}

	_, err = opt.Apply(opts...)
	if assert.Error(t, err) {
		assert.ErrorIs(t, err, schema.ErrBadParameter)
	}
}

func TestAskRespectsProviderGroupsIntegration(t *testing.T) {
	conn, m := newIntegrationManager(t)
	conn.RequireProvider(t)
	ctx := context.Background()
	provider := createIntegrationProvider(t, m, conn.ProviderInsert())
	admin := integrationAdminUser(conn)
	modelName := integrationModelName(t, m, provider.Name, admin, conn.Config.Model)

	t.Run("denies user without groups", func(t *testing.T) {
		assert := assert.New(t)
		_, err := m.Ask(ctx, schema.AskRequest{
			AskRequestCore: schema.AskRequestCore{
				GeneratorMeta: schema.GeneratorMeta{Model: modelName},
				Text:          "Say hello in exactly three words.",
			},
		}, &auth.User{}, nil)
		if assert.Error(err) {
			assert.ErrorIs(err, schema.ErrNotFound)
		}
	})

	t.Run("allows matching group", func(t *testing.T) {
		assert := assert.New(t)
		resp, err := m.Ask(ctx, schema.AskRequest{
			AskRequestCore: schema.AskRequestCore{
				GeneratorMeta: schema.GeneratorMeta{Model: modelName},
				Text:          "Say hello in exactly three words.",
			},
		}, admin, nil)
		if isIntegrationUnreachable(err) {
			t.Skipf("provider unreachable: %v", err)
		}
		if !assert.NoError(err) {
			return
		}
		assert.NotNil(resp)
		assert.Equal(schema.RoleAssistant, resp.Role)
		assert.NotEmpty(resp.Content)
	})
}
