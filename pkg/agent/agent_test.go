package agent

import (
	"context"
	"encoding/json"
	"testing"

	// Packages
	jsonschema "github.com/google/jsonschema-go/jsonschema"
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	"github.com/mutablelogic/go-llm/pkg/tool"
	assert "github.com/stretchr/testify/assert"
)

///////////////////////////////////////////////////////////////////////////////
// MOCK TYPES

// mockClient implements llm.Client only (no Generator, no Embedder)
type mockClient struct {
	name   string
	models []schema.Model
}

func (c *mockClient) Name() string { return c.name }
func (c *mockClient) ListModels(_ context.Context, _ ...opt.Opt) ([]schema.Model, error) {
	return c.models, nil
}
func (c *mockClient) GetModel(_ context.Context, name string, _ ...opt.Opt) (*schema.Model, error) {
	for _, m := range c.models {
		if m.Name == name {
			return &m, nil
		}
	}
	return nil, llm.ErrNotFound
}

// mockGenerator extends mockClient with Generator and Embedder support
type mockGenerator struct {
	mockClient
	response    *schema.Message
	embedResult []float64
}

var _ llm.Generator = (*mockGenerator)(nil)
var _ llm.Embedder = (*mockGenerator)(nil)

func (g *mockGenerator) WithoutSession(_ context.Context, _ schema.Model, _ *schema.Message, _ ...opt.Opt) (*schema.Message, error) {
	return g.response, nil
}
func (g *mockGenerator) WithSession(_ context.Context, _ schema.Model, session *schema.Session, message *schema.Message, _ ...opt.Opt) (*schema.Message, error) {
	session.Append(*message)
	return g.response, nil
}
func (g *mockGenerator) Embedding(_ context.Context, _ schema.Model, _ string, _ ...opt.Opt) ([]float64, error) {
	return g.embedResult, nil
}
func (g *mockGenerator) BatchEmbedding(_ context.Context, _ schema.Model, texts []string, _ ...opt.Opt) ([][]float64, error) {
	result := make([][]float64, len(texts))
	for i := range texts {
		result[i] = g.embedResult
	}
	return result, nil
}

// mockToolCallGenerator returns a tool call on the first invocation,
// then a final text response on the second.
type mockToolCallGenerator struct {
	mockClient
	callCount int
	toolCalls []schema.ToolCall
	finalResp *schema.Message
}

var _ llm.Generator = (*mockToolCallGenerator)(nil)

func (g *mockToolCallGenerator) WithoutSession(_ context.Context, _ schema.Model, _ *schema.Message, _ ...opt.Opt) (*schema.Message, error) {
	return g.finalResp, nil
}
func (g *mockToolCallGenerator) WithSession(_ context.Context, _ schema.Model, session *schema.Session, message *schema.Message, _ ...opt.Opt) (*schema.Message, error) {
	g.callCount++
	session.Append(*message)

	if g.callCount == 1 {
		// First call: return tool call response
		blocks := make([]schema.ContentBlock, 0, len(g.toolCalls))
		for i := range g.toolCalls {
			blocks = append(blocks, schema.ContentBlock{ToolCall: &g.toolCalls[i]})
		}
		return &schema.Message{
			Role:    schema.RoleAssistant,
			Content: blocks,
			Result:  schema.ResultToolCall,
		}, nil
	}

	// Subsequent calls: return final response
	return g.finalResp, nil
}

// mockTool implements tool.Tool for testing
type mockTool struct {
	name        string
	description string
	runResult   any
	runErr      error
}

func (t *mockTool) Name() string        { return t.name }
func (t *mockTool) Description() string { return t.description }
func (t *mockTool) Schema() (*jsonschema.Schema, error) {
	return &jsonschema.Schema{Type: "object"}, nil
}
func (t *mockTool) Run(_ context.Context, _ json.RawMessage) (any, error) {
	return t.runResult, t.runErr
}

// mockInfiniteToolCallGenerator always returns tool calls, never resolving.
type mockInfiniteToolCallGenerator struct {
	mockClient
	callCount int
	toolCalls []schema.ToolCall
}

var _ llm.Generator = (*mockInfiniteToolCallGenerator)(nil)

func (g *mockInfiniteToolCallGenerator) WithoutSession(_ context.Context, _ schema.Model, _ *schema.Message, _ ...opt.Opt) (*schema.Message, error) {
	return nil, llm.ErrNotImplemented
}
func (g *mockInfiniteToolCallGenerator) WithSession(_ context.Context, _ schema.Model, session *schema.Session, message *schema.Message, _ ...opt.Opt) (*schema.Message, error) {
	g.callCount++
	session.Append(*message)
	blocks := make([]schema.ContentBlock, 0, len(g.toolCalls))
	for i := range g.toolCalls {
		blocks = append(blocks, schema.ContentBlock{ToolCall: &g.toolCalls[i]})
	}
	return &schema.Message{
		Role:    schema.RoleAssistant,
		Content: blocks,
		Result:  schema.ResultToolCall,
	}, nil
}

///////////////////////////////////////////////////////////////////////////////
// HELPERS

func newMockModel(name, provider string) schema.Model {
	return schema.Model{Name: name, OwnedBy: provider}
}

func textPtr(s string) *string { return &s }

///////////////////////////////////////////////////////////////////////////////
// AGENT LIFECYCLE TESTS

// Test NewAgent with no clients
func Test_agent_001(t *testing.T) {
	assert := assert.New(t)

	a, err := NewAgent()
	assert.NoError(err)
	assert.NotNil(a)
	assert.Equal("agent", a.Name())
	assert.Empty(a.Providers())
}

// Test NewAgent with one client
func Test_agent_002(t *testing.T) {
	assert := assert.New(t)

	c := &mockClient{name: "test-provider"}
	a, err := NewAgent(WithClient(c))
	assert.NoError(err)
	assert.NotNil(a)
	assert.Len(a.Providers(), 1)
	assert.Equal("test-provider", a.Providers()[0].Name())
}

// Test NewAgent with multiple clients
func Test_agent_003(t *testing.T) {
	assert := assert.New(t)

	c1 := &mockClient{name: "provider-a"}
	c2 := &mockClient{name: "provider-b"}
	a, err := NewAgent(WithClient(c1), WithClient(c2))
	assert.NoError(err)
	assert.Len(a.Providers(), 2)
}

// Test duplicate client name overwrites
func Test_agent_004(t *testing.T) {
	assert := assert.New(t)

	c1 := &mockClient{name: "same", models: []schema.Model{{Name: "m1"}}}
	c2 := &mockClient{name: "same", models: []schema.Model{{Name: "m2"}}}
	a, err := NewAgent(WithClient(c1), WithClient(c2))
	assert.NoError(err)
	assert.Len(a.Providers(), 1)
	// The second client should have overwritten the first
	models, err := a.ListModels(context.TODO())
	assert.NoError(err)
	assert.Len(models, 1)
	assert.Equal("m2", models[0].Name)
}

// Test clientForModel returns nil for unknown provider
func Test_agent_005(t *testing.T) {
	assert := assert.New(t)

	a, _ := NewAgent()
	impl := a.(*agent)
	assert.Nil(impl.clientForModel(schema.Model{Name: "m", OwnedBy: "unknown"}))
}

// Test clientForModel returns nil for empty OwnedBy
func Test_agent_006(t *testing.T) {
	assert := assert.New(t)

	c := &mockClient{name: "test"}
	a, _ := NewAgent(WithClient(c))
	impl := a.(*agent)
	assert.Nil(impl.clientForModel(schema.Model{Name: "m"}))
}

// Test clientForModel returns client for matching provider
func Test_agent_007(t *testing.T) {
	assert := assert.New(t)

	c := &mockClient{name: "test"}
	a, _ := NewAgent(WithClient(c))
	impl := a.(*agent)
	assert.Equal(c, impl.clientForModel(schema.Model{Name: "m", OwnedBy: "test"}))
}

///////////////////////////////////////////////////////////////////////////////
// MODEL TESTS

// Test ListModels aggregates from all providers and sorts
func Test_model_001(t *testing.T) {
	assert := assert.New(t)

	c1 := &mockClient{name: "p1", models: []schema.Model{
		{Name: "zulu", OwnedBy: "p1"},
		{Name: "alpha", OwnedBy: "p1"},
	}}
	c2 := &mockClient{name: "p2", models: []schema.Model{
		{Name: "bravo", OwnedBy: "p2"},
	}}
	a, _ := NewAgent(WithClient(c1), WithClient(c2))
	models, err := a.ListModels(context.TODO())
	assert.NoError(err)
	assert.Len(models, 3)
	// Should be sorted
	assert.Equal("alpha", models[0].Name)
	assert.Equal("bravo", models[1].Name)
	assert.Equal("zulu", models[2].Name)
}

// Test ListModels with WithProvider filters to one provider
func Test_model_002(t *testing.T) {
	assert := assert.New(t)

	c1 := &mockClient{name: "p1", models: []schema.Model{{Name: "m1", OwnedBy: "p1"}}}
	c2 := &mockClient{name: "p2", models: []schema.Model{{Name: "m2", OwnedBy: "p2"}}}
	a, _ := NewAgent(WithClient(c1), WithClient(c2))
	models, err := a.ListModels(context.TODO(), WithProvider("p1"))
	assert.NoError(err)
	assert.Len(models, 1)
	assert.Equal("m1", models[0].Name)
}

// Test ListModels with no matching provider returns empty
func Test_model_003(t *testing.T) {
	assert := assert.New(t)

	c := &mockClient{name: "p1", models: []schema.Model{{Name: "m1"}}}
	a, _ := NewAgent(WithClient(c))
	models, err := a.ListModels(context.TODO(), WithProvider("nonexistent"))
	assert.NoError(err)
	assert.Empty(models)
}

// Test GetModel finds model across providers
func Test_model_004(t *testing.T) {
	assert := assert.New(t)

	c1 := &mockClient{name: "p1", models: []schema.Model{{Name: "m1", OwnedBy: "p1"}}}
	c2 := &mockClient{name: "p2", models: []schema.Model{{Name: "m2", OwnedBy: "p2"}}}
	a, _ := NewAgent(WithClient(c1), WithClient(c2))
	model, err := a.GetModel(context.TODO(), "m2")
	assert.NoError(err)
	assert.NotNil(model)
	assert.Equal("m2", model.Name)
	assert.Equal("p2", model.OwnedBy)
}

// Test GetModel returns not found for unknown model
func Test_model_005(t *testing.T) {
	assert := assert.New(t)

	c := &mockClient{name: "p1", models: []schema.Model{{Name: "m1"}}}
	a, _ := NewAgent(WithClient(c))
	_, err := a.GetModel(context.TODO(), "nonexistent")
	assert.ErrorIs(err, llm.ErrNotFound)
}

// Test GetModel with WithProvider filters search
func Test_model_006(t *testing.T) {
	assert := assert.New(t)

	c1 := &mockClient{name: "p1", models: []schema.Model{{Name: "shared", OwnedBy: "p1"}}}
	c2 := &mockClient{name: "p2", models: []schema.Model{{Name: "shared", OwnedBy: "p2"}}}
	a, _ := NewAgent(WithClient(c1), WithClient(c2))
	model, err := a.GetModel(context.TODO(), "shared", WithProvider("p2"))
	assert.NoError(err)
	assert.Equal("p2", model.OwnedBy)
}

// Test matchProvider helper
func Test_model_007(t *testing.T) {
	assert := assert.New(t)

	// No provider filter — matches all
	o, _ := opt.Apply()
	assert.True(matchProvider(o, "anything"))

	// With provider filter — matches only that provider
	o2, _ := opt.Apply(WithProvider("gemini"))
	assert.True(matchProvider(o2, "gemini"))
	assert.False(matchProvider(o2, "anthropic"))
}

///////////////////////////////////////////////////////////////////////////////
// GENERATOR TESTS

// Test WithoutSession returns error for unknown model
func Test_generator_001(t *testing.T) {
	assert := assert.New(t)

	a, _ := NewAgent()
	msg, _ := schema.NewMessage(schema.RoleUser, "hello")
	_, err := a.WithoutSession(context.TODO(), schema.Model{Name: "m"}, msg)
	assert.Error(err)
}

// Test WithoutSession returns error for non-Generator client
func Test_generator_002(t *testing.T) {
	assert := assert.New(t)

	c := &mockClient{name: "basic", models: []schema.Model{{Name: "m", OwnedBy: "basic"}}}
	a, _ := NewAgent(WithClient(c))
	msg, _ := schema.NewMessage(schema.RoleUser, "hello")
	_, err := a.WithoutSession(context.TODO(), newMockModel("m", "basic"), msg)
	assert.Error(err)
}

// Test WithoutSession delegates to generator
func Test_generator_003(t *testing.T) {
	assert := assert.New(t)

	resp := &schema.Message{Role: schema.RoleAssistant, Content: []schema.ContentBlock{{Text: textPtr("hi")}}}
	g := &mockGenerator{
		mockClient: mockClient{name: "gen", models: []schema.Model{{Name: "m", OwnedBy: "gen"}}},
		response:   resp,
	}
	a, _ := NewAgent(WithClient(g))
	msg, _ := schema.NewMessage(schema.RoleUser, "hello")
	result, err := a.WithoutSession(context.TODO(), newMockModel("m", "gen"), msg)
	assert.NoError(err)
	assert.Equal("hi", *result.Content[0].Text)
}

// Test WithSession returns error for unknown model
func Test_generator_004(t *testing.T) {
	assert := assert.New(t)

	a, _ := NewAgent()
	session := make(schema.Session, 0)
	msg, _ := schema.NewMessage(schema.RoleUser, "hello")
	_, err := a.WithSession(context.TODO(), schema.Model{Name: "m"}, &session, msg)
	assert.Error(err)
}

// Test WithSession delegates to generator
func Test_generator_005(t *testing.T) {
	assert := assert.New(t)

	resp := &schema.Message{Role: schema.RoleAssistant, Content: []schema.ContentBlock{{Text: textPtr("reply")}}}
	g := &mockGenerator{
		mockClient: mockClient{name: "gen", models: []schema.Model{{Name: "m", OwnedBy: "gen"}}},
		response:   resp,
	}
	a, _ := NewAgent(WithClient(g))
	session := make(schema.Session, 0)
	msg, _ := schema.NewMessage(schema.RoleUser, "hello")
	result, err := a.WithSession(context.TODO(), newMockModel("m", "gen"), &session, msg)
	assert.NoError(err)
	assert.Equal("reply", *result.Content[0].Text)
}

// Test WithSession tool loop: model requests tool, agent runs it, feeds back
func Test_generator_006(t *testing.T) {
	assert := assert.New(t)

	finalResp := &schema.Message{
		Role:    schema.RoleAssistant,
		Content: []schema.ContentBlock{{Text: textPtr("The weather is sunny.")}},
		Result:  schema.ResultStop,
	}
	mockWeather := &mockTool{name: "get_weather", description: "Get weather", runResult: "sunny"}
	tk, _ := tool.NewToolkit(mockWeather)

	g := &mockToolCallGenerator{
		mockClient: mockClient{name: "gen", models: []schema.Model{{Name: "m", OwnedBy: "gen"}}},
		toolCalls: []schema.ToolCall{
			{ID: "call_1", Name: "get_weather", Input: json.RawMessage(`{"city":"London"}`)},
		},
		finalResp: finalResp,
	}
	a, _ := NewAgent(WithClient(g))
	session := make(schema.Session, 0)
	msg, _ := schema.NewMessage(schema.RoleUser, "What's the weather?")
	result, err := a.WithSession(context.TODO(), newMockModel("m", "gen"), &session, msg, WithToolkit(tk))
	assert.NoError(err)
	assert.Equal("The weather is sunny.", *result.Content[0].Text)
	// Generator should have been called twice: once for initial, once after tool results
	assert.Equal(2, g.callCount)
}

// Test WithSession without toolkit does not loop on tool calls
func Test_generator_007(t *testing.T) {
	assert := assert.New(t)

	toolCallResp := &schema.Message{
		Role:    schema.RoleAssistant,
		Content: []schema.ContentBlock{{ToolCall: &schema.ToolCall{ID: "c1", Name: "foo"}}},
		Result:  schema.ResultToolCall,
	}
	g := &mockToolCallGenerator{
		mockClient: mockClient{name: "gen", models: []schema.Model{{Name: "m", OwnedBy: "gen"}}},
		finalResp:  toolCallResp,
	}
	a, _ := NewAgent(WithClient(g))
	session := make(schema.Session, 0)
	msg, _ := schema.NewMessage(schema.RoleUser, "hello")
	result, err := a.WithSession(context.TODO(), newMockModel("m", "gen"), &session, msg)
	assert.NoError(err)
	// Without toolkit the tool call response is returned directly
	assert.Equal(schema.ResultToolCall, result.Result)
	assert.Equal(1, g.callCount)
}

// Test tool loop with tool error — error is fed back to the model
func Test_generator_008(t *testing.T) {
	assert := assert.New(t)

	finalResp := &schema.Message{
		Role:    schema.RoleAssistant,
		Content: []schema.ContentBlock{{Text: textPtr("Sorry, tool failed.")}},
		Result:  schema.ResultStop,
	}
	failTool := &mockTool{name: "fail_tool", description: "Always fails", runErr: llm.ErrInternalServerError}
	tk, _ := tool.NewToolkit(failTool)

	g := &mockToolCallGenerator{
		mockClient: mockClient{name: "gen", models: []schema.Model{{Name: "m", OwnedBy: "gen"}}},
		toolCalls: []schema.ToolCall{
			{ID: "c1", Name: "fail_tool", Input: json.RawMessage(`{}`)},
		},
		finalResp: finalResp,
	}
	a, _ := NewAgent(WithClient(g))
	session := make(schema.Session, 0)
	msg, _ := schema.NewMessage(schema.RoleUser, "do something")
	result, err := a.WithSession(context.TODO(), newMockModel("m", "gen"), &session, msg, WithToolkit(tk))
	assert.NoError(err)
	assert.Equal("Sorry, tool failed.", *result.Content[0].Text)
	assert.Equal(2, g.callCount)
}

// Test tool loop returns error when max iterations exceeded (default)
func Test_generator_009(t *testing.T) {
	assert := assert.New(t)

	mockWeather := &mockTool{name: "get_weather", description: "Get weather", runResult: "sunny"}
	tk, _ := tool.NewToolkit(mockWeather)

	g := &mockInfiniteToolCallGenerator{
		mockClient: mockClient{name: "gen", models: []schema.Model{{Name: "m", OwnedBy: "gen"}}},
		toolCalls: []schema.ToolCall{
			{ID: "c1", Name: "get_weather", Input: json.RawMessage(`{}`)},
		},
	}
	a, _ := NewAgent(WithClient(g))
	session := make(schema.Session, 0)
	msg, _ := schema.NewMessage(schema.RoleUser, "weather please")
	_, err := a.WithSession(context.TODO(), newMockModel("m", "gen"), &session, msg, WithToolkit(tk))
	assert.Error(err)
	assert.ErrorIs(err, llm.ErrInternalServerError)
	// The generator was called 1 (initial) + defaultMaxIterations times
	assert.Equal(1+defaultMaxIterations, g.callCount)
}

// Test tool loop respects WithMaxIterations option
func Test_generator_010(t *testing.T) {
	assert := assert.New(t)

	mockWeather := &mockTool{name: "get_weather", description: "Get weather", runResult: "sunny"}
	tk, _ := tool.NewToolkit(mockWeather)

	g := &mockInfiniteToolCallGenerator{
		mockClient: mockClient{name: "gen", models: []schema.Model{{Name: "m", OwnedBy: "gen"}}},
		toolCalls: []schema.ToolCall{
			{ID: "c1", Name: "get_weather", Input: json.RawMessage(`{}`)},
		},
	}
	a, _ := NewAgent(WithClient(g))
	session := make(schema.Session, 0)
	msg, _ := schema.NewMessage(schema.RoleUser, "weather please")
	_, err := a.WithSession(context.TODO(), newMockModel("m", "gen"), &session, msg, WithToolkit(tk), WithMaxIterations(3))
	assert.Error(err)
	assert.ErrorIs(err, llm.ErrInternalServerError)
	// The generator was called 1 (initial) + 3 (custom limit) times
	assert.Equal(1+3, g.callCount)
}

///////////////////////////////////////////////////////////////////////////////
// EMBEDDER TESTS

// Test Embedding returns error for unknown model
func Test_embedder_001(t *testing.T) {
	assert := assert.New(t)

	a, _ := NewAgent()
	_, err := a.Embedding(context.TODO(), schema.Model{Name: "m"}, "text")
	assert.Error(err)
}

// Test Embedding returns error for non-Embedder client
func Test_embedder_002(t *testing.T) {
	assert := assert.New(t)

	c := &mockClient{name: "basic", models: []schema.Model{{Name: "m", OwnedBy: "basic"}}}
	a, _ := NewAgent(WithClient(c))
	_, err := a.Embedding(context.TODO(), newMockModel("m", "basic"), "text")
	assert.Error(err)
}

// Test Embedding delegates to embedder
func Test_embedder_003(t *testing.T) {
	assert := assert.New(t)

	g := &mockGenerator{
		mockClient:  mockClient{name: "emb", models: []schema.Model{{Name: "m", OwnedBy: "emb"}}},
		embedResult: []float64{0.1, 0.2, 0.3},
	}
	a, _ := NewAgent(WithClient(g))
	result, err := a.Embedding(context.TODO(), newMockModel("m", "emb"), "text")
	assert.NoError(err)
	assert.Equal([]float64{0.1, 0.2, 0.3}, result)
}

// Test BatchEmbedding returns error for unknown model
func Test_embedder_004(t *testing.T) {
	assert := assert.New(t)

	a, _ := NewAgent()
	_, err := a.BatchEmbedding(context.TODO(), schema.Model{Name: "m"}, []string{"a", "b"})
	assert.Error(err)
}

// Test BatchEmbedding delegates to embedder
func Test_embedder_005(t *testing.T) {
	assert := assert.New(t)

	g := &mockGenerator{
		mockClient:  mockClient{name: "emb", models: []schema.Model{{Name: "m", OwnedBy: "emb"}}},
		embedResult: []float64{1.0, 2.0},
	}
	a, _ := NewAgent(WithClient(g))
	result, err := a.BatchEmbedding(context.TODO(), newMockModel("m", "emb"), []string{"a", "b"})
	assert.NoError(err)
	assert.Len(result, 2)
	assert.Equal([]float64{1.0, 2.0}, result[0])
	assert.Equal([]float64{1.0, 2.0}, result[1])
}

///////////////////////////////////////////////////////////////////////////////
// OPTION DISPATCH TESTS

// Test convertOptsForClient resolves WithSystemPrompt for gemini
func Test_opts_001(t *testing.T) {
	assert := assert.New(t)

	c := &mockClient{name: schema.Gemini}
	opts := []opt.Opt{WithSystemPrompt("Be concise")}
	resolved, err := convertOptsForClient(opts, c)
	assert.NoError(err)
	// The resolved options should contain the system prompt when applied
	o, err := opt.Apply(resolved...)
	assert.NoError(err)
	assert.Equal("Be concise", o.GetString(opt.SystemPromptKey))
}

// Test convertOptsForClient resolves WithSystemPrompt for anthropic
func Test_opts_002(t *testing.T) {
	assert := assert.New(t)

	c := &mockClient{name: schema.Anthropic}
	opts := []opt.Opt{WithSystemPrompt("Be concise")}
	resolved, err := convertOptsForClient(opts, c)
	assert.NoError(err)
	o, err := opt.Apply(resolved...)
	assert.NoError(err)
	assert.Equal("Be concise", o.GetString(opt.SystemPromptKey))
}

// Test convertOptsForClient resolves WithTemperature
func Test_opts_003(t *testing.T) {
	assert := assert.New(t)

	c := &mockClient{name: schema.Gemini}
	opts := []opt.Opt{WithTemperature(0.7)}
	resolved, err := convertOptsForClient(opts, c)
	assert.NoError(err)
	o, err := opt.Apply(resolved...)
	assert.NoError(err)
	assert.InDelta(0.7, o.GetFloat64(opt.TemperatureKey), 0.001)
}

// Test convertOptsForClient resolves WithMaxTokens
func Test_opts_004(t *testing.T) {
	assert := assert.New(t)

	c := &mockClient{name: schema.Anthropic}
	opts := []opt.Opt{WithMaxTokens(500)}
	resolved, err := convertOptsForClient(opts, c)
	assert.NoError(err)
	o, err := opt.Apply(resolved...)
	assert.NoError(err)
	assert.Equal(uint(500), o.GetUint(opt.MaxTokensKey))
}

// Test convertOptsForClient resolves WithTopK
func Test_opts_005(t *testing.T) {
	assert := assert.New(t)

	c := &mockClient{name: schema.Gemini}
	opts := []opt.Opt{WithTopK(40)}
	resolved, err := convertOptsForClient(opts, c)
	assert.NoError(err)
	o, err := opt.Apply(resolved...)
	assert.NoError(err)
	assert.Equal(uint(40), o.GetUint(opt.TopKKey))
}

// Test convertOptsForClient resolves WithTopP
func Test_opts_006(t *testing.T) {
	assert := assert.New(t)

	c := &mockClient{name: schema.Anthropic}
	opts := []opt.Opt{WithTopP(0.9)}
	resolved, err := convertOptsForClient(opts, c)
	assert.NoError(err)
	o, err := opt.Apply(resolved...)
	assert.NoError(err)
	assert.InDelta(0.9, o.GetFloat64(opt.TopPKey), 0.001)
}

// Test convertOptsForClient resolves WithStopSequences
func Test_opts_007(t *testing.T) {
	assert := assert.New(t)

	c := &mockClient{name: schema.Gemini}
	opts := []opt.Opt{WithStopSequences("END", "STOP")}
	resolved, err := convertOptsForClient(opts, c)
	assert.NoError(err)
	o, err := opt.Apply(resolved...)
	assert.NoError(err)
	assert.Equal([]string{"END", "STOP"}, o.GetStringArray(opt.StopSequencesKey))
}

// Test convertOptsForClient resolves WithThinking for gemini (boolean key)
func Test_opts_008(t *testing.T) {
	assert := assert.New(t)

	c := &mockClient{name: schema.Gemini}
	opts := []opt.Opt{WithThinking()}
	resolved, err := convertOptsForClient(opts, c)
	assert.NoError(err)
	o, err := opt.Apply(resolved...)
	assert.NoError(err)
	assert.True(o.GetBool(opt.ThinkingKey))
}

// Test convertOptsForClient resolves WithThinking for anthropic (budget key)
func Test_opts_009(t *testing.T) {
	assert := assert.New(t)

	c := &mockClient{name: schema.Anthropic}
	opts := []opt.Opt{WithThinking()}
	resolved, err := convertOptsForClient(opts, c)
	assert.NoError(err)
	o, err := opt.Apply(resolved...)
	assert.NoError(err)
	assert.Equal(uint(10240), o.GetUint(opt.ThinkingBudgetKey))
}

// Test convertOptsForClient resolves WithJSONOutput
func Test_opts_010(t *testing.T) {
	assert := assert.New(t)

	c := &mockClient{name: schema.Gemini}
	s := &jsonschema.Schema{Type: "object"}
	opts := []opt.Opt{WithJSONOutput(s)}
	resolved, err := convertOptsForClient(opts, c)
	assert.NoError(err)
	o, err := opt.Apply(resolved...)
	assert.NoError(err)
	assert.NotEmpty(o.GetString(opt.JSONSchemaKey))
}

// Test option dispatch for unsupported provider returns error
func Test_opts_011(t *testing.T) {
	assert := assert.New(t)

	c := &mockClient{name: "unknown"}
	opts := []opt.Opt{WithSystemPrompt("test")}
	_, err := convertOptsForClient(opts, c)
	assert.NoError(err) // convertOptsForClient itself doesn't error

	// The error comes when applying the resolved opts
	resolved, err := convertOptsForClient([]opt.Opt{WithSystemPrompt("test")}, c)
	assert.NoError(err)
	_, err = opt.Apply(resolved...)
	assert.Error(err)
}

// Test embedding option WithTaskType resolves for gemini
func Test_opts_012(t *testing.T) {
	assert := assert.New(t)

	c := &mockClient{name: schema.Gemini}
	opts := []opt.Opt{WithTaskType("RETRIEVAL_QUERY")}
	resolved, err := convertOptsForClient(opts, c)
	assert.NoError(err)
	o, err := opt.Apply(resolved...)
	assert.NoError(err)
	assert.Equal("RETRIEVAL_QUERY", o.GetString(opt.TaskTypeKey))
}

// Test embedding option WithTitle resolves for gemini
func Test_opts_013(t *testing.T) {
	assert := assert.New(t)

	c := &mockClient{name: schema.Gemini}
	opts := []opt.Opt{WithTitle("My Document")}
	resolved, err := convertOptsForClient(opts, c)
	assert.NoError(err)
	o, err := opt.Apply(resolved...)
	assert.NoError(err)
	assert.Equal("My Document", o.GetString(opt.TitleKey))
}

// Test embedding option WithOutputDimensionality resolves for gemini
func Test_opts_014(t *testing.T) {
	assert := assert.New(t)

	c := &mockClient{name: schema.Gemini}
	opts := []opt.Opt{WithOutputDimensionality(768)}
	resolved, err := convertOptsForClient(opts, c)
	assert.NoError(err)
	o, err := opt.Apply(resolved...)
	assert.NoError(err)
	assert.Equal(uint(768), o.GetUint(opt.OutputDimensionalityKey))
}

// Test embedding option returns error for unsupported provider
func Test_opts_015(t *testing.T) {
	assert := assert.New(t)

	c := &mockClient{name: schema.Anthropic}
	opts := []opt.Opt{WithTaskType("RETRIEVAL_QUERY")}
	resolved, err := convertOptsForClient(opts, c)
	assert.NoError(err)
	_, err = opt.Apply(resolved...)
	assert.Error(err)
}

// Test multiple options resolve together
func Test_opts_016(t *testing.T) {
	assert := assert.New(t)

	c := &mockClient{name: schema.Gemini}
	opts := []opt.Opt{
		WithSystemPrompt("Be helpful"),
		WithTemperature(0.5),
		WithMaxTokens(100),
	}
	resolved, err := convertOptsForClient(opts, c)
	assert.NoError(err)
	o, err := opt.Apply(resolved...)
	assert.NoError(err)
	assert.Equal("Be helpful", o.GetString(opt.SystemPromptKey))
	assert.InDelta(0.5, o.GetFloat64(opt.TemperatureKey), 0.001)
	assert.Equal(uint(100), o.GetUint(opt.MaxTokensKey))
}
