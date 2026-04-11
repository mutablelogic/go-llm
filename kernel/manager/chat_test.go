package manager

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	// Packages
	uuid "github.com/google/uuid"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	memoryschema "github.com/mutablelogic/go-llm/memory/schema"
	llmtest "github.com/mutablelogic/go-llm/pkg/test"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
)

func TestConversationForSessionIntegration(t *testing.T) {
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
			Title:         types.Ptr("chat history"),
		},
	}, admin)
	if !assert.NoError(t, err) {
		return
	}

	entries := []schema.MessageInsert{
		{Session: session.ID, Message: schema.Message{Role: schema.RoleUser, Content: []schema.ContentBlock{{Text: types.Ptr("first user")}}, Tokens: 2}},
		{Session: session.ID, Message: schema.Message{Role: schema.RoleAssistant, Content: []schema.ContentBlock{{Text: types.Ptr("first reply")}}, Tokens: 3, Result: schema.ResultStop}},
		{Session: session.ID, Message: schema.Message{Role: schema.RoleUser, Content: []schema.ContentBlock{{Text: types.Ptr("follow up")}}, Tokens: 2}},
	}
	for _, entry := range entries {
		if err := m.PoolConn.Insert(ctx, nil, entry); !assert.NoError(t, err) {
			return
		}
	}

	conversation, err := m.conversationForSession(ctx, session.ID, admin)
	if !assert.NoError(t, err) {
		return
	}

	if assert.Len(t, conversation, 3) {
		assert.Equal(t, schema.RoleUser, conversation[0].Role)
		assert.Equal(t, "first user", conversation[0].Text())
		assert.Equal(t, schema.RoleAssistant, conversation[1].Role)
		assert.Equal(t, "first reply", conversation[1].Text())
		assert.Equal(t, schema.RoleUser, conversation[2].Role)
		assert.Equal(t, "follow up", conversation[2].Text())
	}
}

func TestConversationForSessionPaginatesIntegration(t *testing.T) {
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
			Title:         types.Ptr("chat history pages"),
		},
	}, admin)
	if !assert.NoError(t, err) {
		return
	}

	for i := uint64(0); i < schema.MessageListMax+5; i++ {
		entry := schema.MessageInsert{
			Session: session.ID,
			Message: schema.Message{
				Role:    schema.RoleUser,
				Content: []schema.ContentBlock{{Text: types.Ptr(fmt.Sprintf("message %03d", i))}},
				Tokens:  1,
			},
		}
		if err := m.PoolConn.Insert(ctx, nil, entry); !assert.NoError(t, err) {
			return
		}
	}

	conversation, err := m.conversationForSession(ctx, session.ID, admin)
	if !assert.NoError(t, err) {
		return
	}

	if assert.Len(t, conversation, int(schema.MessageListMax+5)) {
		assert.Equal(t, "message 000", conversation[0].Text())
		assert.Equal(t, fmt.Sprintf("message %03d", schema.MessageListMax+4), conversation[len(conversation)-1].Text())
	}
}

func TestConversationTurnOverhead(t *testing.T) {
	conversation := schema.Conversation{
		&schema.Message{Role: schema.RoleUser, Tokens: 5},
		&schema.Message{Role: schema.RoleAssistant, Tokens: 7},
		&schema.Message{Role: schema.RoleUser, Tokens: 4},
		&schema.Message{Role: schema.RoleAssistant, Tokens: 3},
	}
	reply := conversation[len(conversation)-1]
	usage := &schema.UsageMeta{InputTokens: 20, OutputTokens: 3}

	assert.Equal(t, uint(4), conversationTurnOverhead(conversation, reply, usage, ""))
}

func TestConversationTurnOverheadClampsAtZero(t *testing.T) {
	conversation := schema.Conversation{
		&schema.Message{Role: schema.RoleUser, Tokens: 5},
		&schema.Message{Role: schema.RoleAssistant, Tokens: 7},
	}
	reply := conversation[len(conversation)-1]
	usage := &schema.UsageMeta{InputTokens: 4}

	assert.Zero(t, conversationTurnOverhead(conversation, reply, usage, ""))
	assert.Zero(t, conversationTurnOverhead(conversation, reply, nil, ""))
}

func TestConversationTurnOverheadIncludesSystemPrompt(t *testing.T) {
	conversation := schema.Conversation{
		&schema.Message{Role: schema.RoleUser, Tokens: 5},
		&schema.Message{Role: schema.RoleAssistant, Tokens: 7},
		&schema.Message{Role: schema.RoleUser, Tokens: 4},
		&schema.Message{Role: schema.RoleAssistant, Tokens: 3},
	}
	reply := conversation[len(conversation)-1]
	systemPrompt := "You are a helpful assistant."
	expected := estimateSystemPromptTokens(systemPrompt)
	usage := &schema.UsageMeta{InputTokens: 16 + expected + 2, OutputTokens: 3}

	assert.Equal(t, uint(2), conversationTurnOverhead(conversation, reply, usage, systemPrompt))
}

func TestMergeSystemPrompt(t *testing.T) {
	assert.Nil(t, mergeSystemPrompt(nil, " "))
	assert.Equal(t, "child", types.Value(mergeSystemPrompt(nil, "child")))
	assert.Equal(t, "parent\n\nchild", types.Value(mergeSystemPrompt(types.Ptr("parent"), "child")))
}

func TestFirstTurnMemoryPromptUsesMemorySearchTool(t *testing.T) {
	sessionID := uuid.New()
	tool := &listToolsMockTool{
		name: memorySearchToolKey,
		run: func(ctx context.Context, input json.RawMessage) (any, error) {
			assert.Equal(t, sessionID.String(), toolkit.SessionFromContext(ctx).ID())
			assert.JSONEq(t, `{"q":"*"}`, string(input))
			return memoryschema.MemoryList{Body: []*memoryschema.Memory{
				{MemoryInsert: memoryschema.MemoryInsert{Key: "timezone"}},
				{MemoryInsert: memoryschema.MemoryInsert{Key: "date"}},
			}}, nil
		},
	}

	prompt, err := firstTurnMemoryPrompt(context.Background(), sessionID, nil, toolMap{memorySearchToolKey: tool})
	if !assert.NoError(t, err) {
		return
	}
	assert.Contains(t, prompt, "Current memory keys for this session: date, timezone.")
	assert.Contains(t, prompt, "The name key is not set yet.")
}

func TestFirstTurnMemoryPromptSkipsLaterTurns(t *testing.T) {
	tool := &listToolsMockTool{
		name: memorySearchToolKey,
		run: func(context.Context, json.RawMessage) (any, error) {
			t.Fatal("memory search should not run after the first turn")
			return nil, nil
		},
	}
	conversation := schema.Conversation{{Role: schema.RoleUser}}

	prompt, err := firstTurnMemoryPrompt(context.Background(), uuid.New(), conversation, toolMap{memorySearchToolKey: tool})
	if !assert.NoError(t, err) {
		return
	}
	assert.Empty(t, prompt)
}

func TestFormatMemorySystemPromptOmitsMissingNameReminderWhenPresent(t *testing.T) {
	prompt := formatMemorySystemPrompt([]string{"date", "name", "timezone"})

	assert.Contains(t, prompt, "Current memory keys for this session: date, name, timezone.")
	assert.NotContains(t, prompt, "The name key is not set yet.")
}

func TestMemoryKeysFromSearchResultDeduplicatesAndSorts(t *testing.T) {
	keys := memoryKeysFromSearchResult(map[string]any{
		"body": []map[string]any{{"key": "timezone"}, {"key": "date"}, {"key": "timezone"}, {"key": ""}},
	})

	assert.Equal(t, []string{"date", "timezone"}, keys)
}

func TestConversationLoopMaxIterationsUsesDefault(t *testing.T) {
	assert.Equal(t, uint(schema.DefaultMaxIterations), conversationLoopMaxIterations(0))
	assert.Equal(t, uint(3), conversationLoopMaxIterations(3))
}

func TestShouldEndConversationLoopOnNonToolResult(t *testing.T) {
	reply := &schema.Message{Role: schema.RoleAssistant, Result: schema.ResultStop}

	assert.True(t, shouldEndConversationLoop(reply, 0, 3))
	assert.Equal(t, schema.ResultStop, reply.Result)
}

func TestShouldEndConversationLoopContinuesOnToolCallBeforeMax(t *testing.T) {
	reply := &schema.Message{Role: schema.RoleAssistant, Result: schema.ResultToolCall}

	assert.False(t, shouldEndConversationLoop(reply, 0, 3))
	assert.Equal(t, schema.ResultToolCall, reply.Result)
}

func TestShouldEndConversationLoopMapsToolCallToMaxIterations(t *testing.T) {
	reply := &schema.Message{Role: schema.RoleAssistant, Result: schema.ResultToolCall}

	assert.True(t, shouldEndConversationLoop(reply, 1, 2))
	assert.Equal(t, schema.ResultMaxIterations, reply.Result)
}

func TestChatMessagesToPersistReturnsConversationTailOnSuccess(t *testing.T) {
	conversation := schema.Conversation{
		&schema.Message{Role: schema.RoleUser},
		&schema.Message{Role: schema.RoleAssistant},
		&schema.Message{Role: schema.RoleUser},
		&schema.Message{Role: schema.RoleAssistant},
	}

	got := chatMessagesToPersist(conversation, 2, true)
	if assert.Len(t, got, 2) {
		assert.Same(t, conversation[2], got[0])
		assert.Same(t, conversation[3], got[1])
	}
}

func TestChatMessagesToPersistDropsMessagesOnError(t *testing.T) {
	conversation := schema.Conversation{
		&schema.Message{Role: schema.RoleUser},
		&schema.Message{Role: schema.RoleAssistant},
	}

	assert.Nil(t, chatMessagesToPersist(conversation, 1, false))
	assert.Nil(t, chatMessagesToPersist(conversation, len(conversation), true))
}

func TestNextConversationIterationRunsTools(t *testing.T) {
	m := &Manager{}
	turn := &conversationTurn{
		Reply: &schema.Message{
			Role: schema.RoleAssistant,
			Content: []schema.ContentBlock{{
				ToolCall: &schema.ToolCall{
					ID:    "call_1",
					Name:  "builtin__echo",
					Input: json.RawMessage(`{"value":"hello"}`),
				},
			}},
			Result: schema.ResultToolCall,
		},
	}
	tools := toolMap{
		"builtin__echo": &listToolsMockTool{
			name:        "builtin__echo",
			description: "Echo input",
			run: func(_ context.Context, input json.RawMessage) (any, error) {
				return map[string]any{"echo": json.RawMessage(input)}, nil
			},
		},
	}
	var streamed []string

	message, ok, err := m.nextConversationIteration(context.Background(), uuid.New(), turn, tools, func(role, text string) {
		streamed = append(streamed, role+":"+text)
	})
	if !assert.NoError(t, err) {
		return
	}
	if assert.True(t, ok) && assert.NotNil(t, message) {
		assert.Equal(t, schema.RoleUser, message.Role)
		if assert.Len(t, message.Content, 1) {
			result := message.Content[0].ToolResult
			if assert.NotNil(t, result) {
				assert.Equal(t, "call_1", result.ID)
				assert.Equal(t, "builtin__echo", result.Name)
				assert.False(t, result.IsError)
				assert.JSONEq(t, `{"echo":{"value":"hello"}}`, string(result.Content))
			}
		}
	}
	assert.Equal(t, []string{"tool:builtin__echo: Echo input"}, streamed)
}

func TestNextConversationIterationReturnsToolErrorForMissingTool(t *testing.T) {
	m := &Manager{}
	turn := &conversationTurn{
		Reply: &schema.Message{
			Role: schema.RoleAssistant,
			Content: []schema.ContentBlock{{
				ToolCall: &schema.ToolCall{ID: "call_1", Name: "missing__tool"},
			}},
			Result: schema.ResultToolCall,
		},
	}

	message, ok, err := m.nextConversationIteration(context.Background(), uuid.New(), turn, nil, nil)
	if !assert.NoError(t, err) {
		return
	}
	if assert.True(t, ok) && assert.NotNil(t, message) && assert.Len(t, message.Content, 1) {
		result := message.Content[0].ToolResult
		if assert.NotNil(t, result) {
			assert.Equal(t, "call_1", result.ID)
			assert.Equal(t, "missing__tool", result.Name)
			assert.True(t, result.IsError)
			assert.JSONEq(t, `"not found: tool \"missing__tool\""`, string(result.Content))
		}
	}
}

func TestNextConversationIterationInjectsSession(t *testing.T) {
	m := &Manager{}
	sessionID := uuid.New()
	turn := &conversationTurn{
		Reply: &schema.Message{
			Role: schema.RoleAssistant,
			Content: []schema.ContentBlock{{
				ToolCall: &schema.ToolCall{
					ID:    "call_1",
					Name:  "memory__echo_session",
					Input: json.RawMessage(`{}`),
				},
			}},
			Result: schema.ResultToolCall,
		},
	}
	tools := toolMap{
		"memory__echo_session": &listToolsMockTool{
			name: "memory__echo_session",
			run: func(ctx context.Context, _ json.RawMessage) (any, error) {
				return map[string]any{"session": toolkit.SessionFromContext(ctx).ID()}, nil
			},
		},
	}

	message, ok, err := m.nextConversationIteration(context.Background(), sessionID, turn, tools, nil)
	if !assert.NoError(t, err) {
		return
	}
	if assert.True(t, ok) && assert.NotNil(t, message) && assert.Len(t, message.Content, 1) {
		result := message.Content[0].ToolResult
		if assert.NotNil(t, result) {
			assert.False(t, result.IsError)
			assert.JSONEq(t, fmt.Sprintf(`{"session":%q}`, sessionID.String()), string(result.Content))
		}
	}
}

func TestNextConversationIterationErrorsWithoutToolCalls(t *testing.T) {
	m := &Manager{}
	turn := &conversationTurn{
		Reply: &schema.Message{Role: schema.RoleAssistant, Result: schema.ResultToolCall},
	}

	message, ok, err := m.nextConversationIteration(context.Background(), uuid.New(), turn, nil, nil)
	assert.Error(t, err)
	assert.False(t, ok)
	assert.Nil(t, message)
}
