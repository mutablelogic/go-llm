package google

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
	session := schema.Session{msg}
	o, err := opt.Apply()
	assert.NoError(err)

	req, err := generateRequestFromOpts("gemini-2.0-flash", &session, o)
	assert.NoError(err)
	assert.NotNil(req)
	assert.Len(req.Contents, 1)
	assert.Equal("user", req.Contents[0].Role)
	assert.Equal("Hello", req.Contents[0].Parts[0].Text)
	assert.Nil(req.SystemInstruction)
	assert.Nil(req.GenerationConfig.Temperature) // zero-value config
	assert.Nil(req.Tools)
}

func Test_generateRequest_002(t *testing.T) {
	// Test system prompt is set as SystemInstruction
	assert := assert.New(t)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Session{msg}
	o, err := opt.Apply(WithSystemPrompt("You are a helpful assistant."))
	assert.NoError(err)

	req, err := generateRequestFromOpts("gemini-2.0-flash", &session, o)
	assert.NoError(err)
	assert.NotNil(req.SystemInstruction)
	assert.Len(req.SystemInstruction.Parts, 1)
	assert.Equal("You are a helpful assistant.", req.SystemInstruction.Parts[0].Text)
	assert.Empty(req.SystemInstruction.Role)
}

func Test_generateRequest_003(t *testing.T) {
	// Test temperature option
	assert := assert.New(t)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Session{msg}
	o, err := opt.Apply(WithTemperature(0.7))
	assert.NoError(err)

	req, err := generateRequestFromOpts("gemini-2.0-flash", &session, o)
	assert.NoError(err)
	assert.NotNil(req.GenerationConfig.Temperature)
	assert.InDelta(0.7, *req.GenerationConfig.Temperature, 1e-9)
}

func Test_generateRequest_004(t *testing.T) {
	// Test max tokens option
	assert := assert.New(t)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Session{msg}
	o, err := opt.Apply(WithMaxTokens(2048))
	assert.NoError(err)

	req, err := generateRequestFromOpts("gemini-2.0-flash", &session, o)
	assert.NoError(err)
	assert.Equal(2048, req.GenerationConfig.MaxOutputTokens)
}

func Test_generateRequest_005(t *testing.T) {
	// Test top-k and top-p options
	assert := assert.New(t)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Session{msg}
	o, err := opt.Apply(WithTopK(40), WithTopP(0.95))
	assert.NoError(err)

	req, err := generateRequestFromOpts("gemini-2.0-flash", &session, o)
	assert.NoError(err)
	assert.NotNil(req.GenerationConfig.TopK)
	assert.Equal(40, *req.GenerationConfig.TopK)
	assert.NotNil(req.GenerationConfig.TopP)
	assert.InDelta(0.95, *req.GenerationConfig.TopP, 1e-9)
}

func Test_generateRequest_006(t *testing.T) {
	// Test stop sequences option
	assert := assert.New(t)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Session{msg}
	o, err := opt.Apply(WithStopSequences("STOP", "END"))
	assert.NoError(err)

	req, err := generateRequestFromOpts("gemini-2.0-flash", &session, o)
	assert.NoError(err)
	assert.Equal([]string{"STOP", "END"}, req.GenerationConfig.StopSequences)
}

func Test_generateRequest_007(t *testing.T) {
	// Test thinking option
	assert := assert.New(t)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Session{msg}
	o, err := opt.Apply(WithThinking())
	assert.NoError(err)

	req, err := generateRequestFromOpts("gemini-2.0-flash", &session, o)
	assert.NoError(err)
	assert.NotNil(req.GenerationConfig.ThinkingConfig)
	assert.True(req.GenerationConfig.ThinkingConfig.IncludeThoughts)
}

func Test_generateRequest_008(t *testing.T) {
	// Test all generation options combined
	assert := assert.New(t)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Session{msg}
	o, err := opt.Apply(
		WithSystemPrompt("Be concise."),
		WithTemperature(1.5),
		WithMaxTokens(4096),
		WithTopK(20),
		WithTopP(0.8),
		WithStopSequences("---"),
		WithThinking(),
	)
	assert.NoError(err)

	req, err := generateRequestFromOpts("gemini-2.0-flash", &session, o)
	assert.NoError(err)

	assert.NotNil(req.SystemInstruction)
	assert.Equal("Be concise.", req.SystemInstruction.Parts[0].Text)

	cfg := req.GenerationConfig
	assert.InDelta(1.5, *cfg.Temperature, 1e-9)
	assert.Equal(4096, cfg.MaxOutputTokens)
	assert.Equal(20, *cfg.TopK)
	assert.InDelta(0.8, *cfg.TopP, 1e-9)
	assert.Equal([]string{"---"}, cfg.StopSequences)
	assert.True(cfg.ThinkingConfig.IncludeThoughts)
}

func Test_generateRequest_009(t *testing.T) {
	// Test no generation config when no options are set (omitzero omits it from JSON)
	assert := assert.New(t)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Session{msg}
	o, err := opt.Apply()
	assert.NoError(err)

	req, err := generateRequestFromOpts("gemini-2.0-flash", &session, o)
	assert.NoError(err)

	// Verify the zero-value config is omitted from serialized JSON
	data, err := json.Marshal(req)
	assert.NoError(err)
	var m map[string]any
	assert.NoError(json.Unmarshal(data, &m))
	assert.NotContains(m, "generationConfig")
}

func Test_generateRequest_010(t *testing.T) {
	// Test multi-turn session produces correct contents
	assert := assert.New(t)

	user1 := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hello")}}}
	asst1 := &schema.Message{Role: "assistant", Content: []schema.ContentBlock{{Text: strPtr("Hi there!")}}}
	user2 := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("How are you?")}}}
	session := schema.Session{user1, asst1, user2}
	o, err := opt.Apply()
	assert.NoError(err)

	req, err := generateRequestFromOpts("gemini-2.0-flash", &session, o)
	assert.NoError(err)
	assert.Len(req.Contents, 3)
	assert.Equal("user", req.Contents[0].Role)
	assert.Equal("model", req.Contents[1].Role) // assistant → model
	assert.Equal("user", req.Contents[2].Role)
}

func Test_generateRequest_011(t *testing.T) {
	// Test system messages are filtered from contents
	assert := assert.New(t)

	sys := &schema.Message{Role: "system", Content: []schema.ContentBlock{{Text: strPtr("You are a bot")}}}
	user := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hello")}}}
	session := schema.Session{sys, user}
	o, err := opt.Apply()
	assert.NoError(err)

	req, err := generateRequestFromOpts("gemini-2.0-flash", &session, o)
	assert.NoError(err)
	assert.Len(req.Contents, 1)
	assert.Equal("user", req.Contents[0].Role)
}

func Test_generateRequest_012(t *testing.T) {
	// Test request serializes to valid JSON
	assert := assert.New(t)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Test")}}}
	session := schema.Session{msg}
	o, err := opt.Apply(WithSystemPrompt("System"), WithTemperature(0.5))
	assert.NoError(err)

	req, err := generateRequestFromOpts("gemini-2.0-flash", &session, o)
	assert.NoError(err)

	data, err := json.Marshal(req)
	assert.NoError(err)
	assert.NotEmpty(data)

	// Round-trip: unmarshal and verify key fields
	var m map[string]any
	assert.NoError(json.Unmarshal(data, &m))
	assert.Contains(m, "contents")
	assert.Contains(m, "systemInstruction")
	assert.Contains(m, "generationConfig")
}

func Test_generateRequest_013(t *testing.T) {
	// Test seed option
	assert := assert.New(t)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Session{msg}
	o, err := opt.Apply(WithSeed(42))
	assert.NoError(err)

	req, err := generateRequestFromOpts("gemini-2.0-flash", &session, o)
	assert.NoError(err)
	assert.NotNil(req.GenerationConfig.Seed)
	assert.Equal(42, *req.GenerationConfig.Seed)
}

func Test_generateRequest_014(t *testing.T) {
	// Test presence penalty
	assert := assert.New(t)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Session{msg}
	o, err := opt.Apply(WithPresencePenalty(0.5))
	assert.NoError(err)

	req, err := generateRequestFromOpts("gemini-2.0-flash", &session, o)
	assert.NoError(err)
	assert.NotNil(req.GenerationConfig.PresencePenalty)
	assert.InDelta(0.5, *req.GenerationConfig.PresencePenalty, 0.001)
}

func Test_generateRequest_015(t *testing.T) {
	// Test frequency penalty
	assert := assert.New(t)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Session{msg}
	o, err := opt.Apply(WithFrequencyPenalty(-1.0))
	assert.NoError(err)

	req, err := generateRequestFromOpts("gemini-2.0-flash", &session, o)
	assert.NoError(err)
	assert.NotNil(req.GenerationConfig.FrequencyPenalty)
	assert.InDelta(-1.0, *req.GenerationConfig.FrequencyPenalty, 0.001)
}

func Test_generateRequest_016(t *testing.T) {
	// Test JSON output with schema
	assert := assert.New(t)

	jsonSchema := &jsonschema.Schema{
		Type: "object",
	}
	jsonSchema.Properties = map[string]*jsonschema.Schema{
		"name": {Type: "string"},
		"age":  {Type: "integer"},
	}

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Session{msg}
	o, err := opt.Apply(WithJSONOutput(jsonSchema))
	assert.NoError(err)

	req, err := generateRequestFromOpts("gemini-2.0-flash", &session, o)
	assert.NoError(err)
	assert.Equal("application/json", req.GenerationConfig.ResponseMIMEType)
	assert.NotNil(req.GenerationConfig.ResponseJSONSchema)

	// Verify the schema round-trips correctly
	data, err := json.Marshal(req.GenerationConfig.ResponseJSONSchema)
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
	// Test validation: presence penalty out of range
	_, err := opt.Apply(WithPresencePenalty(3.0))
	assert.Error(t, err)

	_, err = opt.Apply(WithPresencePenalty(-3.0))
	assert.Error(t, err)
}

func Test_generateRequest_018(t *testing.T) {
	// Test validation: frequency penalty out of range
	_, err := opt.Apply(WithFrequencyPenalty(2.5))
	assert.Error(t, err)

	_, err = opt.Apply(WithFrequencyPenalty(-2.5))
	assert.Error(t, err)
}

func Test_generateRequest_019(t *testing.T) {
	// Test validation: nil JSON schema
	_, err := opt.Apply(WithJSONOutput(nil))
	assert.Error(t, err)
}

func Test_generateRequest_020(t *testing.T) {
	// Test combined new options with existing ones
	assert := assert.New(t)

	jsonSchema := &jsonschema.Schema{Type: "object"}
	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Session{msg}
	o, err := opt.Apply(
		WithTemperature(0.7),
		WithSeed(123),
		WithPresencePenalty(0.3),
		WithFrequencyPenalty(-0.5),
		WithJSONOutput(jsonSchema),
	)
	assert.NoError(err)

	req, err := generateRequestFromOpts("gemini-2.0-flash", &session, o)
	assert.NoError(err)
	assert.InDelta(0.7, *req.GenerationConfig.Temperature, 0.001)
	assert.Equal(123, *req.GenerationConfig.Seed)
	assert.InDelta(0.3, *req.GenerationConfig.PresencePenalty, 0.001)
	assert.InDelta(-0.5, *req.GenerationConfig.FrequencyPenalty, 0.001)
	assert.Equal("application/json", req.GenerationConfig.ResponseMIMEType)
	assert.NotNil(req.GenerationConfig.ResponseJSONSchema)
}

///////////////////////////////////////////////////////////////////////////////
// UNIT TESTS — processResponse

func Test_processResponse_001(t *testing.T) {
	// Test basic text response
	assert := assert.New(t)

	c, err := New("test-key")
	assert.NoError(err)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Session{msg}

	response := &geminiGenerateResponse{
		Candidates: []*geminiCandidate{{
			Content: &geminiContent{
				Parts: []*geminiPart{{Text: "Hello!"}},
				Role:  "model",
			},
			FinishReason: geminiFinishReasonStop,
		}},
		UsageMetadata: &geminiUsageMetadata{
			PromptTokenCount:     10,
			CandidatesTokenCount: 5,
			TotalTokenCount:      15,
		},
	}

	result, err := c.processResponse(response, &session)
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
	// Test MAX_TOKENS finish reason returns ErrMaxTokens
	assert := assert.New(t)

	c, err := New("test-key")
	assert.NoError(err)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Session{msg}

	response := &geminiGenerateResponse{
		Candidates: []*geminiCandidate{{
			Content: &geminiContent{
				Parts: []*geminiPart{{Text: "Truncated..."}},
				Role:  "model",
			},
			FinishReason: geminiFinishReasonMaxTokens,
		}},
	}

	result, err := c.processResponse(response, &session)
	assert.ErrorIs(err, llm.ErrMaxTokens)
	assert.NotNil(result)
	assert.Equal("Truncated...", *result.Content[0].Text)
	assert.Equal(schema.ResultMaxTokens, result.Result)
}

func Test_processResponse_003(t *testing.T) {
	// Test SAFETY finish reason returns ErrRefusal
	assert := assert.New(t)

	c, err := New("test-key")
	assert.NoError(err)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Session{msg}

	response := &geminiGenerateResponse{
		Candidates: []*geminiCandidate{{
			Content: &geminiContent{
				Parts: []*geminiPart{},
				Role:  "model",
			},
			FinishReason: geminiFinishReasonSafety,
		}},
	}

	_, err = c.processResponse(response, &session)
	assert.ErrorIs(err, llm.ErrRefusal)
}

func Test_processResponse_004(t *testing.T) {
	// Test empty response (no candidates)
	assert := assert.New(t)

	c, err := New("test-key")
	assert.NoError(err)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Session{msg}

	response := &geminiGenerateResponse{}

	result, err := c.processResponse(response, &session)
	assert.NoError(err)
	assert.NotNil(result)
}

func Test_processResponse_005(t *testing.T) {
	// Test response with function call
	assert := assert.New(t)

	c, err := New("test-key")
	assert.NoError(err)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("What's the weather?")}}}
	session := schema.Session{msg}

	response := &geminiGenerateResponse{
		Candidates: []*geminiCandidate{{
			Content: &geminiContent{
				Parts: []*geminiPart{{
					FunctionCall: &geminiFunctionCall{
						Name: "get_weather",
						Args: map[string]any{"city": "London"},
					},
				}},
				Role: "model",
			},
			FinishReason: geminiFinishReasonStop,
		}},
	}

	result, err := c.processResponse(response, &session)
	assert.NoError(err)
	assert.NotNil(result)
	assert.Equal(schema.ResultToolCall, result.Result)
	assert.Len(result.Content, 1)
	assert.NotNil(result.Content[0].ToolCall)
	assert.Equal("get_weather", result.Content[0].ToolCall.Name)
}

func Test_processResponse_006(t *testing.T) {
	// Test response with thinking content
	assert := assert.New(t)

	c, err := New("test-key")
	assert.NoError(err)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Explain quantum mechanics")}}}
	session := schema.Session{msg}

	response := &geminiGenerateResponse{
		Candidates: []*geminiCandidate{{
			Content: &geminiContent{
				Parts: []*geminiPart{
					{Text: "Let me think about this...", Thought: true, ThoughtSignature: "sig123"},
					{Text: "Quantum mechanics is a branch of physics."},
				},
				Role: "model",
			},
			FinishReason: geminiFinishReasonStop,
		}},
	}

	result, err := c.processResponse(response, &session)
	assert.NoError(err)
	assert.NotNil(result)
	assert.Len(result.Content, 2)

	// First block is thinking
	assert.NotNil(result.Content[0].Text)
	assert.Equal("Let me think about this...", *result.Content[0].Text)
	assert.Equal(true, result.Meta["thought"])
	assert.Equal("sig123", result.Meta["thought_signature"])

	// Second block is normal text
	assert.NotNil(result.Content[1].Text)
	assert.Equal("Quantum mechanics is a branch of physics.", *result.Content[1].Text)
}

func Test_processResponse_007(t *testing.T) {
	// Test token counts are propagated to session
	assert := assert.New(t)

	c, err := New("test-key")
	assert.NoError(err)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	session := schema.Session{msg}

	response := &geminiGenerateResponse{
		Candidates: []*geminiCandidate{{
			Content: &geminiContent{
				Parts: []*geminiPart{{Text: "Hello!"}},
				Role:  "model",
			},
			FinishReason: geminiFinishReasonStop,
		}},
		UsageMetadata: &geminiUsageMetadata{
			PromptTokenCount:     25,
			CandidatesTokenCount: 10,
			TotalTokenCount:      35,
		},
	}

	result, err := c.processResponse(response, &session)
	assert.NoError(err)
	assert.NotNil(result)

	// Output message should have token count set
	assert.Equal(uint(10), session[len(session)-1].Tokens)
}

///////////////////////////////////////////////////////////////////////////////
// UNIT TESTS — GenerateRequest (public helper)

func Test_GenerateRequest_001(t *testing.T) {
	// Test the public GenerateRequest helper
	assert := assert.New(t)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hello")}}}
	session := schema.Session{msg}

	result, err := GenerateRequest("gemini-2.0-flash", &session, WithTemperature(0.5), WithMaxTokens(100))
	assert.NoError(err)
	assert.NotNil(result)

	// Should be a *geminiGenerateRequest
	req, ok := result.(*geminiGenerateRequest)
	assert.True(ok)
	assert.NotNil(req.GenerationConfig)
	assert.InDelta(0.5, *req.GenerationConfig.Temperature, 1e-9)
	assert.Equal(100, req.GenerationConfig.MaxOutputTokens)
}

///////////////////////////////////////////////////////////////////////////////
// UNIT TESTS — WithoutSession / WithSession validation

func Test_WithoutSession_nil_message(t *testing.T) {
	assert := assert.New(t)
	c, err := New("test-key")
	assert.NoError(err)

	_, err = c.WithoutSession(context.TODO(), schema.Model{Name: "test"}, nil)
	assert.Error(err)
}

func Test_WithSession_nil_session(t *testing.T) {
	assert := assert.New(t)
	c, err := New("test-key")
	assert.NoError(err)

	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: strPtr("Hi")}}}
	_, err = c.WithSession(context.TODO(), schema.Model{Name: "test"}, nil, msg)
	assert.Error(err)
}

func Test_WithSession_nil_message(t *testing.T) {
	assert := assert.New(t)
	c, err := New("test-key")
	assert.NoError(err)

	session := &schema.Session{}
	_, err = c.WithSession(context.TODO(), schema.Model{Name: "test"}, session, nil)
	assert.Error(err)
}

///////////////////////////////////////////////////////////////////////////////
// INTEGRATION TESTS

func Test_generate_001(t *testing.T) {
	// Test basic non-streaming generation
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("GOOGLE_API_KEY")
	}
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set, skipping")
	}
	assert := assert.New(t)

	c, err := New(apiKey)
	assert.NoError(err)

	model := schema.Model{Name: "gemini-2.0-flash"}
	msg, err := schema.NewMessage("user", "Say hello in exactly three words.")
	assert.NoError(err)

	response, err := c.WithoutSession(context.TODO(), model, msg)
	assert.NoError(err)
	assert.NotNil(response)
	assert.Equal("assistant", response.Role)
	assert.NotEmpty(response.Content)
	assert.NotNil(response.Content[0].Text)
	t.Logf("Response: %s", *response.Content[0].Text)
}

func Test_generate_002(t *testing.T) {
	// Test streaming generation
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("GOOGLE_API_KEY")
	}
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set, skipping")
	}
	assert := assert.New(t)

	c, err := New(apiKey)
	assert.NoError(err)

	var streamed string
	streamFn := func(role, text string) {
		streamed += text
	}

	model := schema.Model{Name: "gemini-2.0-flash"}
	msg, err := schema.NewMessage("user", "Say hello in exactly three words.")
	assert.NoError(err)

	response, err := c.WithoutSession(context.TODO(), model, msg, opt.WithStream(streamFn))
	assert.NoError(err)
	assert.NotNil(response)
	assert.NotEmpty(streamed)
	t.Logf("Streamed: %s", streamed)
}

func Test_generate_003(t *testing.T) {
	// Test multi-turn session
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("GOOGLE_API_KEY")
	}
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set, skipping")
	}
	assert := assert.New(t)

	c, err := New(apiKey)
	assert.NoError(err)

	model := schema.Model{Name: "gemini-2.0-flash"}

	// First turn
	msg1, err := schema.NewMessage("user", "My name is Alice.")
	assert.NoError(err)

	session := &schema.Session{}
	resp1, err := c.WithSession(context.TODO(), model, session, msg1)
	assert.NoError(err)
	assert.NotNil(resp1)
	t.Logf("Turn 1: %s", *resp1.Content[0].Text)

	// Second turn — model should remember
	msg2, err := schema.NewMessage("user", "What is my name?")
	assert.NoError(err)

	resp2, err := c.WithSession(context.TODO(), model, session, msg2)
	assert.NoError(err)
	assert.NotNil(resp2)
	assert.NotNil(resp2.Content[0].Text)
	assert.Contains(*resp2.Content[0].Text, "Alice")
	t.Logf("Turn 2: %s", *resp2.Content[0].Text)
}

func Test_generate_004(t *testing.T) {
	// Test with system prompt
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("GOOGLE_API_KEY")
	}
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set, skipping")
	}
	assert := assert.New(t)

	c, err := New(apiKey)
	assert.NoError(err)

	model := schema.Model{Name: "gemini-2.0-flash"}
	msg, err := schema.NewMessage("user", "What are you?")
	assert.NoError(err)

	response, err := c.WithoutSession(context.TODO(), model, msg,
		WithSystemPrompt("You are a pirate. Always respond in pirate speak."),
		WithMaxTokens(100),
	)
	assert.NoError(err)
	if assert.NotNil(response) && len(response.Content) > 0 && response.Content[0].Text != nil {
		t.Logf("Response: %s", *response.Content[0].Text)
	}
}

func Test_generate_005(t *testing.T) {
	// Test with generation options
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("GOOGLE_API_KEY")
	}
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set, skipping")
	}
	assert := assert.New(t)

	c, err := New(apiKey)
	assert.NoError(err)

	model := schema.Model{Name: "gemini-2.0-flash"}
	msg, err := schema.NewMessage("user", "Write exactly one word.")
	assert.NoError(err)

	response, err := c.WithoutSession(context.TODO(), model, msg,
		WithTemperature(0.0),
		WithMaxTokens(10),
	)
	assert.NoError(err)
	assert.NotNil(response)
	t.Logf("Response: %s", *response.Content[0].Text)
}

///////////////////////////////////////////////////////////////////////////////
// HELPER

func strPtr(s string) *string {
	return &s
}
