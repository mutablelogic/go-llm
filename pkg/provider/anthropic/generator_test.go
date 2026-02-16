package anthropic

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	// Packages
	"github.com/google/jsonschema-go/jsonschema"
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	assert "github.com/stretchr/testify/assert"
)

///////////////////////////////////////////////////////////////////////////////
// UNIT TESTS — generateRequestFromOpts

func Test_generateRequest_001(t *testing.T) {
	// Test minimal request with a single user message
	assert := assert.New(t)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hello")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply()
	assert.NoError(err)

	req, err := generateRequestFromOpts("claude-sonnet-4-20250514", &session, o)
	assert.NoError(err)
	assert.NotNil(req)
	assert.Equal("claude-sonnet-4-20250514", req.Model)
	assert.Equal(defaultMaxTokens, req.MaxTokens)
	assert.Len(req.Messages, 1)
	assert.Equal("user", req.Messages[0].Role)
	assert.Equal("Hello", req.Messages[0].Content[0].Text)
	assert.Nil(req.System)
	assert.Nil(req.Temperature)
	assert.Nil(req.Thinking)
	assert.Nil(req.ToolChoice)
	assert.Nil(req.Tools)
	assert.False(req.Stream)
}

func Test_generateRequest_002(t *testing.T) {
	// Test system prompt is set as plain string
	assert := assert.New(t)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply(WithSystemPrompt("You are a helpful assistant."))
	assert.NoError(err)

	req, err := generateRequestFromOpts("claude-sonnet-4-20250514", &session, o)
	assert.NoError(err)
	assert.Equal("You are a helpful assistant.", req.System)
}

func Test_generateRequest_003(t *testing.T) {
	// Test cached system prompt produces array with cache_control
	assert := assert.New(t)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply(WithCachedSystemPrompt("Cached prompt"))
	assert.NoError(err)

	req, err := generateRequestFromOpts("claude-sonnet-4-20250514", &session, o)
	assert.NoError(err)

	// Should be a []textBlockParam
	blocks, ok := req.System.([]textBlockParam)
	assert.True(ok)
	assert.Len(blocks, 1)
	assert.Equal("text", blocks[0].Type)
	assert.Equal("Cached prompt", blocks[0].Text)
	assert.NotNil(blocks[0].CacheControl)
	assert.Equal("ephemeral", blocks[0].CacheControl.Type)
}

func Test_generateRequest_004(t *testing.T) {
	// Test temperature option
	assert := assert.New(t)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply(WithTemperature(0.7))
	assert.NoError(err)

	req, err := generateRequestFromOpts("claude-sonnet-4-20250514", &session, o)
	assert.NoError(err)
	assert.NotNil(req.Temperature)
	assert.InDelta(0.7, *req.Temperature, 1e-9)
}

func Test_generateRequest_005(t *testing.T) {
	// Test max tokens option overrides default
	assert := assert.New(t)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply(WithMaxTokens(4096))
	assert.NoError(err)

	req, err := generateRequestFromOpts("claude-sonnet-4-20250514", &session, o)
	assert.NoError(err)
	assert.Equal(4096, req.MaxTokens)
}

func Test_generateRequest_006(t *testing.T) {
	// Test top-k and top-p options
	assert := assert.New(t)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply(WithTopK(40), WithTopP(0.95))
	assert.NoError(err)

	req, err := generateRequestFromOpts("claude-sonnet-4-20250514", &session, o)
	assert.NoError(err)
	assert.NotNil(req.TopK)
	assert.Equal(uint(40), *req.TopK)
	assert.NotNil(req.TopP)
	assert.InDelta(0.95, *req.TopP, 1e-9)
}

func Test_generateRequest_007(t *testing.T) {
	// Test stop sequences option
	assert := assert.New(t)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply(WithStopSequences("STOP", "END"))
	assert.NoError(err)

	req, err := generateRequestFromOpts("claude-sonnet-4-20250514", &session, o)
	assert.NoError(err)
	assert.Equal([]string{"STOP", "END"}, req.StopSequences)
}

func Test_generateRequest_008(t *testing.T) {
	// Test thinking option
	assert := assert.New(t)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply(WithThinking(2048))
	assert.NoError(err)

	req, err := generateRequestFromOpts("claude-sonnet-4-20250514", &session, o)
	assert.NoError(err)
	assert.NotNil(req.Thinking)
	assert.Equal("enabled", req.Thinking.Type)
	assert.Equal(uint(2048), req.Thinking.BudgetTokens)
}

func Test_generateRequest_009(t *testing.T) {
	// Test all generation options combined
	assert := assert.New(t)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply(
		WithSystemPrompt("Be concise."),
		WithTemperature(0.5),
		WithMaxTokens(2048),
		WithTopK(20),
		WithTopP(0.8),
		WithStopSequences("---"),
		WithServiceTier("auto"),
	)
	assert.NoError(err)

	req, err := generateRequestFromOpts("claude-sonnet-4-20250514", &session, o)
	assert.NoError(err)

	assert.Equal("Be concise.", req.System)
	assert.InDelta(0.5, *req.Temperature, 1e-9)
	assert.Equal(2048, req.MaxTokens)
	assert.Equal(uint(20), *req.TopK)
	assert.InDelta(0.8, *req.TopP, 1e-9)
	assert.Equal([]string{"---"}, req.StopSequences)
	assert.Equal("auto", req.ServiceTier)
}

func Test_generateRequest_010(t *testing.T) {
	// Test request serializes to valid JSON with expected fields
	assert := assert.New(t)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Test")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply(WithSystemPrompt("System"), WithTemperature(0.5))
	assert.NoError(err)

	req, err := generateRequestFromOpts("claude-sonnet-4-20250514", &session, o)
	assert.NoError(err)

	data, err := json.Marshal(req)
	assert.NoError(err)
	assert.NotEmpty(data)

	var m map[string]any
	assert.NoError(json.Unmarshal(data, &m))
	assert.Contains(m, "model")
	assert.Contains(m, "messages")
	assert.Contains(m, "max_tokens")
	assert.Contains(m, "system")
	assert.Contains(m, "temperature")
}

func Test_generateRequest_011(t *testing.T) {
	// Test multi-turn session produces correct messages
	assert := assert.New(t)

	user1 := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hello")}}}
	asst1 := &schema.Message{Role: "assistant", Content: []schema.ContentBlock{{Text: strPtr("Hi there!")}}}
	user2 := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("How are you?")}}}
	session := schema.Conversation{user1, asst1, user2}
	o, err := opt.Apply()
	assert.NoError(err)

	req, err := generateRequestFromOpts("claude-sonnet-4-20250514", &session, o)
	assert.NoError(err)
	assert.Len(req.Messages, 3)
	assert.Equal("user", req.Messages[0].Role)
	assert.Equal("assistant", req.Messages[1].Role)
	assert.Equal("user", req.Messages[2].Role)
}

func Test_generateRequest_012(t *testing.T) {
	// Test system messages are filtered from messages
	assert := assert.New(t)

	sys := &schema.Message{Role: "system", Content: []schema.ContentBlock{{Text: strPtr("You are a bot")}}}
	user := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hello")}}}
	session := schema.Conversation{sys, user}
	o, err := opt.Apply()
	assert.NoError(err)

	req, err := generateRequestFromOpts("claude-sonnet-4-20250514", &session, o)
	assert.NoError(err)
	assert.Len(req.Messages, 1)
	assert.Equal("user", req.Messages[0].Role)
}

func Test_generateRequest_013(t *testing.T) {
	// Test stream flag is not set in request by default — it is set by generate()
	// when a stream callback is provided via opt.WithStream(fn)
	assert := assert.New(t)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply()
	assert.NoError(err)

	req, err := generateRequestFromOpts("claude-sonnet-4-20250514", &session, o)
	assert.NoError(err)
	assert.False(req.Stream)
}

func Test_generateRequest_014(t *testing.T) {
	// Test tool choice auto
	assert := assert.New(t)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply(WithToolChoiceAuto())
	assert.NoError(err)

	req, err := generateRequestFromOpts("claude-sonnet-4-20250514", &session, o)
	assert.NoError(err)
	assert.NotNil(req.ToolChoice)
	assert.Equal("auto", req.ToolChoice.Type)
}

func Test_generateRequest_015(t *testing.T) {
	// Test tool choice with specific tool name
	assert := assert.New(t)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply(WithToolChoice("get_weather"))
	assert.NoError(err)

	req, err := generateRequestFromOpts("claude-sonnet-4-20250514", &session, o)
	assert.NoError(err)
	assert.NotNil(req.ToolChoice)
	assert.Equal("tool", req.ToolChoice.Type)
	assert.Equal("get_weather", req.ToolChoice.Name)
}

func Test_generateRequest_016(t *testing.T) {
	// Test JSON output with schema
	assert := assert.New(t)

	jsonSchema := &jsonschema.Schema{Type: "object"}
	jsonSchema.Properties = map[string]*jsonschema.Schema{
		"name": {Type: "string"},
		"age":  {Type: "integer"},
	}

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply(WithJSONOutput(jsonSchema))
	assert.NoError(err)

	req, err := generateRequestFromOpts("claude-sonnet-4-20250514", &session, o)
	assert.NoError(err)
	assert.NotNil(req.OutputConfig)
	assert.NotNil(req.OutputConfig.Format)
	assert.Equal("json_schema", req.OutputConfig.Format.Type)
	assert.NotNil(req.OutputConfig.Format.Schema)

	// Verify the schema round-trips correctly
	data, err := json.Marshal(req.OutputConfig.Format.Schema)
	assert.NoError(err)
	var m map[string]any
	assert.NoError(json.Unmarshal(data, &m))
	assert.Equal("object", m["type"])
	props, ok := m["properties"].(map[string]any)
	assert.True(ok)
	assert.Contains(props, "name")
	assert.Contains(props, "age")
}

func Test_generateRequest_017(t *testing.T) {
	// Test user metadata
	assert := assert.New(t)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply(WithUser("user-123"))
	assert.NoError(err)

	req, err := generateRequestFromOpts("claude-sonnet-4-20250514", &session, o)
	assert.NoError(err)
	assert.NotNil(req.Metadata)
	assert.Equal("user-123", req.Metadata.UserId)
}

func Test_generateRequest_018(t *testing.T) {
	// Test output config
	assert := assert.New(t)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply(WithOutputConfig("low"))
	assert.NoError(err)

	req, err := generateRequestFromOpts("claude-sonnet-4-20250514", &session, o)
	assert.NoError(err)
	assert.NotNil(req.OutputConfig)
	assert.Equal("low", req.OutputConfig.Effort)
}

func Test_generateRequest_019(t *testing.T) {
	// Test no optional fields in JSON when no options set
	assert := assert.New(t)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply()
	assert.NoError(err)

	req, err := generateRequestFromOpts("claude-sonnet-4-20250514", &session, o)
	assert.NoError(err)

	data, err := json.Marshal(req)
	assert.NoError(err)
	var m map[string]any
	assert.NoError(json.Unmarshal(data, &m))

	// These optional fields should be absent
	assert.NotContains(m, "system")
	assert.NotContains(m, "temperature")
	assert.NotContains(m, "thinking")
	assert.NotContains(m, "top_k")
	assert.NotContains(m, "top_p")
	assert.NotContains(m, "stop_sequences")
	assert.NotContains(m, "tool_choice")
	assert.NotContains(m, "tools")
	assert.NotContains(m, "metadata")
	assert.NotContains(m, "output_format")
	assert.NotContains(m, "output_config")
	assert.NotContains(m, "service_tier")

	// Required fields should be present
	assert.Contains(m, "model")
	assert.Contains(m, "messages")
	assert.Contains(m, "max_tokens")
}

func Test_generateRequest_020(t *testing.T) {
	// Test that max_tokens is auto-bumped when thinking budget exceeds it
	assert := assert.New(t)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply(WithThinking(10240))
	assert.NoError(err)

	req, err := generateRequestFromOpts("claude-sonnet-4-20250514", &session, o)
	assert.NoError(err)
	assert.NotNil(req.Thinking)
	assert.Equal(uint(10240), req.Thinking.BudgetTokens)
	// max_tokens must be greater than budget_tokens
	assert.Greater(req.MaxTokens, int(req.Thinking.BudgetTokens))
	assert.Equal(10240+defaultMaxTokens, req.MaxTokens)
}

func Test_generateRequest_021(t *testing.T) {
	// Test that explicit max_tokens larger than budget is preserved
	assert := assert.New(t)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply(WithThinking(4096), WithMaxTokens(16384))
	assert.NoError(err)

	req, err := generateRequestFromOpts("claude-sonnet-4-20250514", &session, o)
	assert.NoError(err)
	assert.Equal(16384, req.MaxTokens)
	assert.Equal(uint(4096), req.Thinking.BudgetTokens)
}

///////////////////////////////////////////////////////////////////////////////
// UNIT TESTS — option validation

func Test_generateRequest_validation_001(t *testing.T) {
	// Temperature out of range
	_, err := opt.Apply(WithTemperature(1.5))
	assert.Error(t, err)

	_, err = opt.Apply(WithTemperature(-0.1))
	assert.Error(t, err)
}

func Test_generateRequest_validation_002(t *testing.T) {
	// Thinking budget below minimum
	_, err := opt.Apply(WithThinking(512))
	assert.Error(t, err)
}

func Test_generateRequest_validation_003(t *testing.T) {
	// Max tokens below minimum
	_, err := opt.Apply(WithMaxTokens(0))
	assert.Error(t, err)
}

func Test_generateRequest_validation_004(t *testing.T) {
	// TopP out of range
	_, err := opt.Apply(WithTopP(1.5))
	assert.Error(t, err)
}

func Test_generateRequest_validation_005(t *testing.T) {
	// Nil JSON schema
	_, err := opt.Apply(WithJSONOutput(nil))
	assert.Error(t, err)
}

func Test_generateRequest_validation_006(t *testing.T) {
	// Empty stop sequences
	_, err := opt.Apply(WithStopSequences())
	assert.Error(t, err)
}

func Test_generateRequest_validation_007(t *testing.T) {
	// Invalid output config value
	_, err := opt.Apply(WithOutputConfig("invalid"))
	assert.Error(t, err)
}

///////////////////////////////////////////////////////////////////////////////
// UNIT TESTS — processResponse

func Test_processResponse_001(t *testing.T) {
	// Test basic text response
	assert := assert.New(t)

	c, err := New("test-key")
	assert.NoError(err)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Conversation{msg}

	response := &messagesResponse{
		Id:         "msg_001",
		Model:      "claude-sonnet-4-20250514",
		Role:       "assistant",
		StopReason: stopReasonEndTurn,
		Content: []anthropicContentBlock{
			{Type: blockTypeText, Text: "Hello!"},
		},
		Usage: messagesUsage{InputTokens: 10, OutputTokens: 5},
	}

	result, _, err := c.processResponse(response, &session)
	assert.NoError(err)
	assert.NotNil(result)
	assert.Equal("assistant", result.Role)
	assert.Len(result.Content, 1)
	assert.Equal("Hello!", *result.Content[0].Text)
	assert.Equal(schema.ResultStop, result.Result)

	// Session should now have 2 messages
	assert.Len(session, 2)
}

func Test_processResponse_002(t *testing.T) {
	// Test max_tokens stop reason returns ErrMaxTokens
	assert := assert.New(t)

	c, err := New("test-key")
	assert.NoError(err)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Conversation{msg}

	response := &messagesResponse{
		Role:       "assistant",
		StopReason: stopReasonMaxTokens,
		Content: []anthropicContentBlock{
			{Type: blockTypeText, Text: "Truncated..."},
		},
		Usage: messagesUsage{InputTokens: 10, OutputTokens: 100},
	}

	result, _, err := c.processResponse(response, &session)
	assert.ErrorIs(err, llm.ErrMaxTokens)
	assert.NotNil(result)
	assert.Equal("Truncated...", *result.Content[0].Text)
	assert.Equal(schema.ResultMaxTokens, result.Result)
}

func Test_processResponse_003(t *testing.T) {
	// Test refusal stop reason returns ErrRefusal with no message
	assert := assert.New(t)

	c, err := New("test-key")
	assert.NoError(err)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Conversation{msg}

	response := &messagesResponse{
		Role:       "assistant",
		StopReason: stopReasonRefusal,
		Content:    []anthropicContentBlock{},
	}

	result, _, err := c.processResponse(response, &session)
	assert.ErrorIs(err, llm.ErrRefusal)
	assert.Nil(result)

	// Session should still have only the original message
	assert.Len(session, 1)
}

func Test_processResponse_004(t *testing.T) {
	// Test tool_use stop reason with tool call
	assert := assert.New(t)

	c, err := New("test-key")
	assert.NoError(err)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("What's the weather?")}}}
	session := schema.Conversation{msg}

	response := &messagesResponse{
		Role:       "assistant",
		StopReason: stopReasonToolUse,
		Content: []anthropicContentBlock{
			{
				Type:  blockTypeToolUse,
				ID:    "toolu_001",
				Name:  "get_weather",
				Input: json.RawMessage(`{"city":"London"}`),
			},
		},
		Usage: messagesUsage{InputTokens: 15, OutputTokens: 20},
	}

	result, _, err := c.processResponse(response, &session)
	assert.NoError(err)
	assert.NotNil(result)
	assert.Equal(schema.ResultToolCall, result.Result)
	assert.Len(result.Content, 1)
	assert.NotNil(result.Content[0].ToolCall)
	assert.Equal("get_weather", result.Content[0].ToolCall.Name)
	assert.Equal("toolu_001", result.Content[0].ToolCall.ID)
}

func Test_processResponse_005(t *testing.T) {
	// Test response with thinking content
	assert := assert.New(t)

	c, err := New("test-key")
	assert.NoError(err)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Explain physics")}}}
	session := schema.Conversation{msg}

	response := &messagesResponse{
		Role:       "assistant",
		StopReason: stopReasonEndTurn,
		Content: []anthropicContentBlock{
			{Type: blockTypeThinking, Thinking: "Let me think about this...", Signature: "sig123"},
			{Type: blockTypeText, Text: "Physics is the study of matter and energy."},
		},
		Usage: messagesUsage{InputTokens: 10, OutputTokens: 30},
	}

	result, _, err := c.processResponse(response, &session)
	assert.NoError(err)
	assert.NotNil(result)
	assert.Len(result.Content, 2)
}

func Test_processResponse_006(t *testing.T) {
	// Test token counts are propagated to session
	assert := assert.New(t)

	c, err := New("test-key")
	assert.NoError(err)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Conversation{msg}

	response := &messagesResponse{
		Role:       "assistant",
		StopReason: stopReasonEndTurn,
		Content: []anthropicContentBlock{
			{Type: blockTypeText, Text: "Hello!"},
		},
		Usage: messagesUsage{InputTokens: 25, OutputTokens: 10},
	}

	result, _, err := c.processResponse(response, &session)
	assert.NoError(err)
	assert.NotNil(result)

	// Output message should have token count set
	assert.Equal(uint(10), session[len(session)-1].Tokens)
}

func Test_processResponse_007(t *testing.T) {
	// Test pause_turn stop reason returns ErrPauseTurn
	assert := assert.New(t)

	c, err := New("test-key")
	assert.NoError(err)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Conversation{msg}

	response := &messagesResponse{
		Role:       "assistant",
		StopReason: stopReasonPauseTurn,
		Content: []anthropicContentBlock{
			{Type: blockTypeText, Text: "Partial..."},
		},
		Usage: messagesUsage{InputTokens: 10, OutputTokens: 5},
	}

	result, _, err := c.processResponse(response, &session)
	assert.ErrorIs(err, llm.ErrPauseTurn)
	assert.NotNil(result)
}

///////////////////////////////////////////////////////////////////////////////
// UNIT TESTS — GenerateRequest (public helper)

func Test_GenerateRequest_001(t *testing.T) {
	// Test the public GenerateRequest helper
	assert := assert.New(t)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hello")}}}
	session := schema.Conversation{msg}

	result, err := GenerateRequest("claude-sonnet-4-20250514", &session, WithTemperature(0.5), WithMaxTokens(100))
	assert.NoError(err)
	assert.NotNil(result)

	// Should be a *messagesRequest
	req, ok := result.(*messagesRequest)
	assert.True(ok)
	assert.InDelta(0.5, *req.Temperature, 1e-9)
	assert.Equal(100, req.MaxTokens)
}

///////////////////////////////////////////////////////////////////////////////
// UNIT TESTS — WithoutSession / WithSession validation

func Test_WithoutSession_nil_message(t *testing.T) {
	assert := assert.New(t)
	c, err := New("test-key")
	assert.NoError(err)

	_, _, err = c.WithoutSession(context.TODO(), schema.Model{Name: "test"}, nil)
	assert.Error(err)
}

func Test_WithSession_nil_session(t *testing.T) {
	assert := assert.New(t)
	c, err := New("test-key")
	assert.NoError(err)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	_, _, err = c.WithSession(context.TODO(), schema.Model{Name: "test"}, nil, msg)
	assert.Error(err)
}

func Test_WithSession_nil_message(t *testing.T) {
	assert := assert.New(t)
	c, err := New("test-key")
	assert.NoError(err)

	session := &schema.Conversation{}
	_, _, err = c.WithSession(context.TODO(), schema.Model{Name: "test"}, session, nil)
	assert.Error(err)
}

///////////////////////////////////////////////////////////////////////////////
// INTEGRATION TESTS

func Test_generate_001(t *testing.T) {
	// Test basic non-streaming generation
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY not set, skipping")
	}
	assert := assert.New(t)

	c, err := New(apiKey)
	assert.NoError(err)

	model := schema.Model{Name: "claude-haiku-4-5-20251001"}
	msg, err := schema.NewMessage("user", "Say hello in exactly three words.")
	assert.NoError(err)

	response, _, err := c.WithoutSession(context.TODO(), model, msg)
	assert.NoError(err)
	assert.NotNil(response)
	assert.Equal("assistant", response.Role)
	assert.NotEmpty(response.Content)
	t.Logf("Response: %s", response.Text())
}

func Test_generate_002(t *testing.T) {
	// Test streaming generation
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY not set, skipping")
	}
	assert := assert.New(t)

	c, err := New(apiKey)
	assert.NoError(err)

	var streamed string
	streamFn := func(role, text string) {
		streamed += text
	}

	model := schema.Model{Name: "claude-haiku-4-5-20251001"}
	msg, err := schema.NewMessage("user", "Say hello in exactly three words.")
	assert.NoError(err)

	response, _, err := c.WithoutSession(context.TODO(), model, msg, opt.WithStream(streamFn))
	assert.NoError(err)
	assert.NotNil(response)
	assert.NotEmpty(streamed)
	t.Logf("Streamed: %s", streamed)
}

func Test_generate_003(t *testing.T) {
	// Test multi-turn session
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY not set, skipping")
	}
	assert := assert.New(t)

	c, err := New(apiKey)
	assert.NoError(err)

	model := schema.Model{Name: "claude-haiku-4-5-20251001"}

	// First turn
	msg1, err := schema.NewMessage("user", "My name is Alice.")
	assert.NoError(err)

	session := &schema.Conversation{}
	resp1, _, err := c.WithSession(context.TODO(), model, session, msg1)
	assert.NoError(err)
	assert.NotNil(resp1)
	t.Logf("Turn 1: %s", resp1.Text())

	// Second turn — model should remember
	msg2, err := schema.NewMessage("user", "What is my name?")
	assert.NoError(err)

	resp2, _, err := c.WithSession(context.TODO(), model, session, msg2)
	assert.NoError(err)
	assert.NotNil(resp2)
	assert.Contains(resp2.Text(), "Alice")
	t.Logf("Turn 2: %s", resp2.Text())
}

func Test_generate_004(t *testing.T) {
	// Test with system prompt
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY not set, skipping")
	}
	assert := assert.New(t)

	c, err := New(apiKey)
	assert.NoError(err)

	model := schema.Model{Name: "claude-haiku-4-5-20251001"}
	msg, err := schema.NewMessage("user", "What are you?")
	assert.NoError(err)

	response, _, err := c.WithoutSession(context.TODO(), model, msg,
		WithSystemPrompt("You are a pirate. Always respond in pirate speak."),
		WithMaxTokens(200),
	)
	assert.NoError(err)
	if assert.NotNil(response) {
		t.Logf("Response: %s", response.Text())
	}
}

func Test_generate_005(t *testing.T) {
	// Test with generation options
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY not set, skipping")
	}
	assert := assert.New(t)

	c, err := New(apiKey)
	assert.NoError(err)

	model := schema.Model{Name: "claude-haiku-4-5-20251001"}
	msg, err := schema.NewMessage("user", "Write exactly one word.")
	assert.NoError(err)

	response, _, err := c.WithoutSession(context.TODO(), model, msg,
		WithTemperature(0.0),
		WithMaxTokens(10),
	)
	assert.NoError(err)
	assert.NotNil(response)
	t.Logf("Response: %s", response.Text())
}

///////////////////////////////////////////////////////////////////////////////
// HELPER

func strPtr(s string) *string {
	return &s
}
