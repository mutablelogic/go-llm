package mistral

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	// Packages
	jsonschema "github.com/google/jsonschema-go/jsonschema"
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

	req, err := generateRequestFromOpts("mistral-small-latest", &session, o)
	assert.NoError(err)
	assert.NotNil(req)
	assert.Equal("mistral-small-latest", req.Model)
	assert.NotNil(req.MaxTokens)
	assert.Equal(defaultMaxTokens, *req.MaxTokens)
	assert.Len(req.Messages, 1)
	assert.Equal("user", req.Messages[0].Role)
	assert.Equal("Hello", req.Messages[0].Content)
	assert.Nil(req.Temperature)
	assert.Nil(req.TopP)
	assert.Nil(req.RandomSeed)
	assert.Nil(req.PresencePenalty)
	assert.Nil(req.FrequencyPenalty)
	assert.Nil(req.Tools)
	assert.Nil(req.ToolChoice)
	assert.Nil(req.ResponseFormat)
	assert.False(req.Stream)
	assert.False(req.SafePrompt)
}

func Test_generateRequest_002(t *testing.T) {
	// Test system prompt is prepended as a system role message
	assert := assert.New(t)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply(WithSystemPrompt("You are a helpful assistant."))
	assert.NoError(err)

	req, err := generateRequestFromOpts("mistral-small-latest", &session, o)
	assert.NoError(err)
	assert.Len(req.Messages, 2)
	assert.Equal(roleSystem, req.Messages[0].Role)
	assert.Equal("You are a helpful assistant.", req.Messages[0].Content)
	assert.Equal("user", req.Messages[1].Role)
}

func Test_generateRequest_003(t *testing.T) {
	// Test temperature option
	assert := assert.New(t)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply(WithTemperature(0.7))
	assert.NoError(err)

	req, err := generateRequestFromOpts("mistral-small-latest", &session, o)
	assert.NoError(err)
	assert.NotNil(req.Temperature)
	assert.InDelta(0.7, *req.Temperature, 1e-9)
}

func Test_generateRequest_004(t *testing.T) {
	// Test max tokens option overrides default
	assert := assert.New(t)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply(WithMaxTokens(4096))
	assert.NoError(err)

	req, err := generateRequestFromOpts("mistral-small-latest", &session, o)
	assert.NoError(err)
	assert.NotNil(req.MaxTokens)
	assert.Equal(4096, *req.MaxTokens)
}

func Test_generateRequest_005(t *testing.T) {
	// Test top-p option
	assert := assert.New(t)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply(WithTopP(0.95))
	assert.NoError(err)

	req, err := generateRequestFromOpts("mistral-small-latest", &session, o)
	assert.NoError(err)
	assert.NotNil(req.TopP)
	assert.InDelta(0.95, *req.TopP, 1e-9)
}

func Test_generateRequest_006(t *testing.T) {
	// Test stop sequences option
	assert := assert.New(t)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply(WithStopSequences("STOP", "END"))
	assert.NoError(err)

	req, err := generateRequestFromOpts("mistral-small-latest", &session, o)
	assert.NoError(err)
	assert.Equal([]string{"STOP", "END"}, req.Stop)
}

func Test_generateRequest_007(t *testing.T) {
	// Test seed option
	assert := assert.New(t)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply(WithSeed(42))
	assert.NoError(err)

	req, err := generateRequestFromOpts("mistral-small-latest", &session, o)
	assert.NoError(err)
	assert.NotNil(req.RandomSeed)
	assert.Equal(uint(42), *req.RandomSeed)
}

func Test_generateRequest_008(t *testing.T) {
	// Test presence and frequency penalty options
	assert := assert.New(t)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply(WithPresencePenalty(0.5), WithFrequencyPenalty(-1.0))
	assert.NoError(err)

	req, err := generateRequestFromOpts("mistral-small-latest", &session, o)
	assert.NoError(err)
	assert.NotNil(req.PresencePenalty)
	assert.InDelta(0.5, *req.PresencePenalty, 1e-9)
	assert.NotNil(req.FrequencyPenalty)
	assert.InDelta(-1.0, *req.FrequencyPenalty, 1e-9)
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
		WithTopP(0.8),
		WithStopSequences("---"),
		WithSeed(99),
		WithPresencePenalty(0.3),
		WithFrequencyPenalty(-0.5),
		WithSafePrompt(),
	)
	assert.NoError(err)

	req, err := generateRequestFromOpts("mistral-small-latest", &session, o)
	assert.NoError(err)

	// System prompt prepended
	assert.Len(req.Messages, 2)
	assert.Equal(roleSystem, req.Messages[0].Role)
	assert.Equal("Be concise.", req.Messages[0].Content)

	assert.InDelta(0.5, *req.Temperature, 1e-9)
	assert.Equal(2048, *req.MaxTokens)
	assert.InDelta(0.8, *req.TopP, 1e-9)
	assert.Equal([]string{"---"}, req.Stop)
	assert.Equal(uint(99), *req.RandomSeed)
	assert.InDelta(0.3, *req.PresencePenalty, 1e-9)
	assert.InDelta(-0.5, *req.FrequencyPenalty, 1e-9)
	assert.True(req.SafePrompt)
}

func Test_generateRequest_010(t *testing.T) {
	// Test request serializes to valid JSON with expected fields
	assert := assert.New(t)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Test")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply(WithSystemPrompt("System"), WithTemperature(0.5))
	assert.NoError(err)

	req, err := generateRequestFromOpts("mistral-small-latest", &session, o)
	assert.NoError(err)

	data, err := json.Marshal(req)
	assert.NoError(err)
	assert.NotEmpty(data)

	var m map[string]any
	assert.NoError(json.Unmarshal(data, &m))
	assert.Contains(m, "model")
	assert.Contains(m, "messages")
	assert.Contains(m, "max_tokens")
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

	req, err := generateRequestFromOpts("mistral-small-latest", &session, o)
	assert.NoError(err)
	assert.Len(req.Messages, 3)
	assert.Equal("user", req.Messages[0].Role)
	assert.Equal("assistant", req.Messages[1].Role)
	assert.Equal("user", req.Messages[2].Role)
}

func Test_generateRequest_012(t *testing.T) {
	// Test stream flag is not set by generateRequestFromOpts — it is set by generate()
	assert := assert.New(t)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply()
	assert.NoError(err)

	req, err := generateRequestFromOpts("mistral-small-latest", &session, o)
	assert.NoError(err)
	assert.False(req.Stream)
}

func Test_generateRequest_013(t *testing.T) {
	// Test tool choice auto
	assert := assert.New(t)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply(WithToolChoiceAuto())
	assert.NoError(err)

	req, err := generateRequestFromOpts("mistral-small-latest", &session, o)
	assert.NoError(err)
	assert.Equal(toolChoiceAuto, req.ToolChoice)
}

func Test_generateRequest_014(t *testing.T) {
	// Test tool choice none
	assert := assert.New(t)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply(WithToolChoiceNone())
	assert.NoError(err)

	req, err := generateRequestFromOpts("mistral-small-latest", &session, o)
	assert.NoError(err)
	assert.Equal(toolChoiceNone, req.ToolChoice)
}

func Test_generateRequest_015(t *testing.T) {
	// Test tool choice any
	assert := assert.New(t)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply(WithToolChoiceAny())
	assert.NoError(err)

	req, err := generateRequestFromOpts("mistral-small-latest", &session, o)
	assert.NoError(err)
	assert.Equal(toolChoiceAny, req.ToolChoice)
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

	req, err := generateRequestFromOpts("mistral-small-latest", &session, o)
	assert.NoError(err)
	assert.NotNil(req.ResponseFormat)
	assert.Equal(responseFormatJSONSchema, req.ResponseFormat.Type)
	assert.NotNil(req.ResponseFormat.JSONSchema)
	assert.Equal("json_output", req.ResponseFormat.JSONSchema.Name)

	// Verify the schema round-trips correctly
	var m map[string]any
	assert.NoError(json.Unmarshal(req.ResponseFormat.JSONSchema.Schema, &m))
	assert.Equal("object", m["type"])
	props, ok := m["properties"].(map[string]any)
	assert.True(ok)
	assert.Contains(props, "name")
	assert.Contains(props, "age")
}

func Test_generateRequest_017(t *testing.T) {
	// Test safe prompt option
	assert := assert.New(t)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply(WithSafePrompt())
	assert.NoError(err)

	req, err := generateRequestFromOpts("mistral-small-latest", &session, o)
	assert.NoError(err)
	assert.True(req.SafePrompt)
}

func Test_generateRequest_018(t *testing.T) {
	// Test no optional fields in JSON when no options set (except max_tokens default)
	assert := assert.New(t)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply()
	assert.NoError(err)

	req, err := generateRequestFromOpts("mistral-small-latest", &session, o)
	assert.NoError(err)

	data, err := json.Marshal(req)
	assert.NoError(err)
	var m map[string]any
	assert.NoError(json.Unmarshal(data, &m))

	// These optional fields should be absent
	assert.NotContains(m, "temperature")
	assert.NotContains(m, "top_p")
	assert.NotContains(m, "stop")
	assert.NotContains(m, "random_seed")
	assert.NotContains(m, "tools")
	assert.NotContains(m, "tool_choice")
	assert.NotContains(m, "response_format")
	assert.NotContains(m, "presence_penalty")
	assert.NotContains(m, "frequency_penalty")

	// Required fields should be present
	assert.Contains(m, "model")
	assert.Contains(m, "messages")
	assert.Contains(m, "max_tokens")
}

///////////////////////////////////////////////////////////////////////////////
// UNIT TESTS — option validation

func Test_generateRequest_validation_001(t *testing.T) {
	// Temperature out of range
	_, err := opt.Apply(WithTemperature(2.0))
	assert.Error(t, err)

	_, err = opt.Apply(WithTemperature(-0.1))
	assert.Error(t, err)
}

func Test_generateRequest_validation_002(t *testing.T) {
	// Max tokens below minimum
	_, err := opt.Apply(WithMaxTokens(0))
	assert.Error(t, err)
}

func Test_generateRequest_validation_003(t *testing.T) {
	// TopP out of range
	_, err := opt.Apply(WithTopP(1.5))
	assert.Error(t, err)

	_, err = opt.Apply(WithTopP(-0.1))
	assert.Error(t, err)
}

func Test_generateRequest_validation_004(t *testing.T) {
	// Nil JSON schema
	_, err := opt.Apply(WithJSONOutput(nil))
	assert.Error(t, err)
}

func Test_generateRequest_validation_005(t *testing.T) {
	// Empty stop sequences
	_, err := opt.Apply(WithStopSequences())
	assert.Error(t, err)
}

func Test_generateRequest_validation_006(t *testing.T) {
	// Presence penalty out of range
	_, err := opt.Apply(WithPresencePenalty(3.0))
	assert.Error(t, err)

	_, err = opt.Apply(WithPresencePenalty(-3.0))
	assert.Error(t, err)
}

func Test_generateRequest_validation_007(t *testing.T) {
	// Frequency penalty out of range
	_, err := opt.Apply(WithFrequencyPenalty(2.5))
	assert.Error(t, err)

	_, err = opt.Apply(WithFrequencyPenalty(-2.5))
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

	response := &chatCompletionResponse{
		Id:    "cmpl-001",
		Model: "mistral-small-latest",
		Choices: []chatChoice{{
			Index: 0,
			Message: mistralMessage{
				Role:    roleAssistant,
				Content: "Hello!",
			},
			FinishReason: finishReasonStop,
		}},
		Usage: chatUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
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
	// Test length finish reason returns ErrMaxTokens
	assert := assert.New(t)

	c, err := New("test-key")
	assert.NoError(err)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Conversation{msg}

	response := &chatCompletionResponse{
		Choices: []chatChoice{{
			Message: mistralMessage{
				Role:    roleAssistant,
				Content: "Truncated...",
			},
			FinishReason: finishReasonLength,
		}},
		Usage: chatUsage{PromptTokens: 10, CompletionTokens: 100},
	}

	result, _, err := c.processResponse(response, &session)
	assert.ErrorIs(err, llm.ErrMaxTokens)
	assert.NotNil(result)
	assert.Equal("Truncated...", *result.Content[0].Text)
	assert.Equal(schema.ResultMaxTokens, result.Result)
}

func Test_processResponse_003(t *testing.T) {
	// Test model_length finish reason returns ErrMaxTokens
	assert := assert.New(t)

	c, err := New("test-key")
	assert.NoError(err)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Conversation{msg}

	response := &chatCompletionResponse{
		Choices: []chatChoice{{
			Message: mistralMessage{
				Role:    roleAssistant,
				Content: "Model length...",
			},
			FinishReason: finishReasonModelLength,
		}},
	}

	result, _, err := c.processResponse(response, &session)
	assert.ErrorIs(err, llm.ErrMaxTokens)
	assert.NotNil(result)
}

func Test_processResponse_004(t *testing.T) {
	// Test tool_calls finish reason with tool call
	assert := assert.New(t)

	c, err := New("test-key")
	assert.NoError(err)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("What's the weather?")}}}
	session := schema.Conversation{msg}

	response := &chatCompletionResponse{
		Choices: []chatChoice{{
			Message: mistralMessage{
				Role:    roleAssistant,
				Content: "",
				ToolCalls: []mistralToolCall{{
					Id:   "call_001",
					Type: "function",
					Function: mistralFunction{
						Name:      "get_weather",
						Arguments: `{"city":"London"}`,
					},
				}},
			},
			FinishReason: finishReasonToolCalls,
		}},
		Usage: chatUsage{PromptTokens: 15, CompletionTokens: 20},
	}

	result, _, err := c.processResponse(response, &session)
	assert.NoError(err)
	assert.NotNil(result)
	assert.Equal(schema.ResultToolCall, result.Result)
	assert.Len(result.Content, 1)
	assert.NotNil(result.Content[0].ToolCall)
	assert.Equal("get_weather", result.Content[0].ToolCall.Name)
	assert.Equal("call_001", result.Content[0].ToolCall.ID)
}

func Test_processResponse_005(t *testing.T) {
	// Test error finish reason returns ErrInternalServerError
	assert := assert.New(t)

	c, err := New("test-key")
	assert.NoError(err)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Conversation{msg}

	response := &chatCompletionResponse{
		Choices: []chatChoice{{
			Message: mistralMessage{
				Role:    roleAssistant,
				Content: "",
			},
			FinishReason: finishReasonError,
		}},
	}

	result, _, err := c.processResponse(response, &session)
	assert.ErrorIs(err, llm.ErrInternalServerError)
	assert.NotNil(result)
}

func Test_processResponse_006(t *testing.T) {
	// Test token counts are propagated to session
	assert := assert.New(t)

	c, err := New("test-key")
	assert.NoError(err)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Conversation{msg}

	response := &chatCompletionResponse{
		Choices: []chatChoice{{
			Message: mistralMessage{
				Role:    roleAssistant,
				Content: "Hello!",
			},
			FinishReason: finishReasonStop,
		}},
		Usage: chatUsage{PromptTokens: 25, CompletionTokens: 10, TotalTokens: 35},
	}

	result, _, err := c.processResponse(response, &session)
	assert.NoError(err)
	assert.NotNil(result)

	// Output message should have token count set
	assert.Equal(uint(10), session[len(session)-1].Tokens)
}

func Test_processResponse_007(t *testing.T) {
	// Test empty response (no choices)
	assert := assert.New(t)

	c, err := New("test-key")
	assert.NoError(err)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Conversation{msg}

	response := &chatCompletionResponse{}

	result, _, err := c.processResponse(response, &session)
	assert.NoError(err)
	assert.NotNil(result)
}

///////////////////////////////////////////////////////////////////////////////
// UNIT TESTS — GenerateRequest (public helper)

func Test_GenerateRequest_001(t *testing.T) {
	// Test the public GenerateRequest helper
	assert := assert.New(t)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hello")}}}
	session := schema.Conversation{msg}

	result, err := GenerateRequest("mistral-small-latest", &session, WithTemperature(0.5), WithMaxTokens(100))
	assert.NoError(err)
	assert.NotNil(result)

	// Should be a *chatCompletionRequest
	req, ok := result.(*chatCompletionRequest)
	assert.True(ok)
	assert.NotNil(req.Temperature)
	assert.InDelta(0.5, *req.Temperature, 1e-9)
	assert.NotNil(req.MaxTokens)
	assert.Equal(100, *req.MaxTokens)
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
	apiKey := os.Getenv("MISTRAL_API_KEY")
	if apiKey == "" {
		t.Skip("MISTRAL_API_KEY not set, skipping")
	}
	assert := assert.New(t)

	c, err := New(apiKey)
	assert.NoError(err)

	model := schema.Model{Name: "mistral-small-latest"}
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
	apiKey := os.Getenv("MISTRAL_API_KEY")
	if apiKey == "" {
		t.Skip("MISTRAL_API_KEY not set, skipping")
	}
	assert := assert.New(t)

	c, err := New(apiKey)
	assert.NoError(err)

	var streamed string
	streamFn := func(role, text string) {
		streamed += text
	}

	model := schema.Model{Name: "mistral-small-latest"}
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
	apiKey := os.Getenv("MISTRAL_API_KEY")
	if apiKey == "" {
		t.Skip("MISTRAL_API_KEY not set, skipping")
	}
	assert := assert.New(t)

	c, err := New(apiKey)
	assert.NoError(err)

	model := schema.Model{Name: "mistral-small-latest"}

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
	apiKey := os.Getenv("MISTRAL_API_KEY")
	if apiKey == "" {
		t.Skip("MISTRAL_API_KEY not set, skipping")
	}
	assert := assert.New(t)

	c, err := New(apiKey)
	assert.NoError(err)

	model := schema.Model{Name: "mistral-small-latest"}
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
	apiKey := os.Getenv("MISTRAL_API_KEY")
	if apiKey == "" {
		t.Skip("MISTRAL_API_KEY not set, skipping")
	}
	assert := assert.New(t)

	c, err := New(apiKey)
	assert.NoError(err)

	model := schema.Model{Name: "mistral-small-latest"}
	msg, err := schema.NewMessage("user", "Write exactly one word.")
	assert.NoError(err)

	response, _, err := c.WithoutSession(context.TODO(), model, msg,
		WithTemperature(0.0),
		WithMaxTokens(10),
	)
	// With only 10 tokens the response may be truncated
	if err != nil {
		assert.ErrorIs(err, llm.ErrMaxTokens)
	}
	assert.NotNil(response)
	t.Logf("Response: %s", response.Text())
}

///////////////////////////////////////////////////////////////////////////////
// HELPER

func strPtr(s string) *string {
	return &s
}
