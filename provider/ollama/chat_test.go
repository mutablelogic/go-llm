package ollama

import (
	"encoding/json"
	"testing"

	// Packages
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
)

///////////////////////////////////////////////////////////////////////////////
// UNIT TESTS — chatRequestFromOpts

func Test_chatRequest_001(t *testing.T) {
	// Minimal request: model name and single user message
	a := assert.New(t)
	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: types.Ptr("Hello")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply()
	a.NoError(err)

	req, err := chatRequestFromOpts("llama3.2", &session, o)
	a.NoError(err)
	a.NotNil(req)
	a.Equal("llama3.2", req.Model)
	a.Len(req.Messages, 1)
	a.Equal("user", req.Messages[0].Role)
	a.Equal("Hello", req.Messages[0].Content)
	a.Nil(req.Options)
	a.Nil(req.Think)
	a.Nil(req.Stream)
	a.Empty(req.Format)
	a.Nil(req.Tools)
}

func Test_chatRequest_002(t *testing.T) {
	// System prompt is prepended as a system role message
	a := assert.New(t)
	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: types.Ptr("Hi")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply(opt.SetString(opt.SystemPromptKey, "You are helpful."))
	a.NoError(err)

	req, err := chatRequestFromOpts("llama3.2", &session, o)
	a.NoError(err)
	a.Len(req.Messages, 2)
	a.Equal("system", req.Messages[0].Role)
	a.Equal("You are helpful.", req.Messages[0].Content)
	a.Equal("user", req.Messages[1].Role)
}

func Test_chatRequest_003(t *testing.T) {
	// Temperature is placed in the options map
	a := assert.New(t)
	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: types.Ptr("Hi")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply(opt.SetFloat64(opt.TemperatureKey, 0.7))
	a.NoError(err)

	req, err := chatRequestFromOpts("llama3.2", &session, o)
	a.NoError(err)
	a.NotNil(req.Options)
	a.InDelta(0.7, req.Options["temperature"], 1e-9)
}

func Test_chatRequest_004(t *testing.T) {
	// Top P is placed in the options map
	a := assert.New(t)
	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: types.Ptr("Hi")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply(opt.SetFloat64(opt.TopPKey, 0.9))
	a.NoError(err)

	req, err := chatRequestFromOpts("llama3.2", &session, o)
	a.NoError(err)
	a.InDelta(0.9, req.Options["top_p"], 1e-9)
}

func Test_chatRequest_005(t *testing.T) {
	// Top K is placed in the options map
	a := assert.New(t)
	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: types.Ptr("Hi")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply(opt.SetUint(opt.TopKKey, 50))
	a.NoError(err)

	req, err := chatRequestFromOpts("llama3.2", &session, o)
	a.NoError(err)
	a.EqualValues(50, req.Options["top_k"])
}

func Test_chatRequest_006(t *testing.T) {
	// Max tokens maps to num_predict in the options map
	a := assert.New(t)
	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: types.Ptr("Hi")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply(opt.SetUint(opt.MaxTokensKey, 2048))
	a.NoError(err)

	req, err := chatRequestFromOpts("llama3.2", &session, o)
	a.NoError(err)
	a.EqualValues(2048, req.Options["num_predict"])
}

func Test_chatRequest_007(t *testing.T) {
	// Stop sequences are placed in the options map
	a := assert.New(t)
	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: types.Ptr("Hi")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply(opt.AddString(opt.StopSequencesKey, "STOP", "END"))
	a.NoError(err)

	req, err := chatRequestFromOpts("llama3.2", &session, o)
	a.NoError(err)
	a.Equal([]string{"STOP", "END"}, req.Options["stop"])
}

func Test_chatRequest_008(t *testing.T) {
	// Seed is placed in the options map
	a := assert.New(t)
	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: types.Ptr("Hi")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply(opt.SetUint(opt.SeedKey, 42))
	a.NoError(err)

	req, err := chatRequestFromOpts("llama3.2", &session, o)
	a.NoError(err)
	a.EqualValues(42, req.Options["seed"])
}

func Test_chatRequest_009(t *testing.T) {
	// Presence and frequency penalty are placed in the options map
	a := assert.New(t)
	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: types.Ptr("Hi")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply(
		opt.SetFloat64(opt.PresencePenaltyKey, 0.5),
		opt.SetFloat64(opt.FrequencyPenaltyKey, -0.5),
	)
	a.NoError(err)

	req, err := chatRequestFromOpts("llama3.2", &session, o)
	a.NoError(err)
	a.InDelta(0.5, req.Options["presence_penalty"], 1e-9)
	a.InDelta(-0.5, req.Options["frequency_penalty"], 1e-9)
}

func Test_chatRequest_010(t *testing.T) {
	// Thinking bool enables think field
	a := assert.New(t)
	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: types.Ptr("Hi")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply(opt.SetString(opt.ThinkingKey, "true"))
	a.NoError(err)

	req, err := chatRequestFromOpts("llama3.2", &session, o)
	a.NoError(err)
	a.NotNil(req.Think)
	a.Equal(true, req.Think.Value)
}

func Test_chatRequest_011(t *testing.T) {
	// Thinking effort string ("high") enables think field
	a := assert.New(t)
	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: types.Ptr("Hi")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply(opt.SetString(opt.ThinkingKey, "high"))
	a.NoError(err)

	req, err := chatRequestFromOpts("llama3.2", &session, o)
	a.NoError(err)
	a.NotNil(req.Think)
	a.Equal("high", req.Think.Value)
}

func Test_chatRequest_012(t *testing.T) {
	// JSON schema format is set on the request
	a := assert.New(t)
	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: types.Ptr("Hi")}}}
	session := schema.Conversation{msg}
	var outputSchema jsonschema.Schema
	err := json.Unmarshal([]byte(`{"type":"object","properties":{"answer":{"type":"string"}}}`), &outputSchema)
	a.NoError(err)
	o, err := opt.Apply(WithJSONOutput(&outputSchema))
	a.NoError(err)

	req, err := chatRequestFromOpts("llama3.2", &session, o)
	a.NoError(err)
	a.NotEmpty(req.Format)
	var m map[string]any
	a.NoError(json.Unmarshal(req.Format, &m))
	a.Equal("object", m["type"])
}

func Test_chatRequest_012a(t *testing.T) {
	// Nil schema is rejected by WithJSONOutput
	a := assert.New(t)

	_, err := opt.Apply(WithJSONOutput(nil))
	a.Error(err)
	a.ErrorIs(err, schema.ErrBadParameter)
}

func Test_chatRequest_013(t *testing.T) {
	// Nil session produces empty messages list
	a := assert.New(t)
	o, err := opt.Apply()
	a.NoError(err)

	req, err := chatRequestFromOpts("llama3.2", nil, o)
	a.NoError(err)
	a.Empty(req.Messages)
}

func Test_chatRequest_014(t *testing.T) {
	// All options combined serialize to valid JSON
	a := assert.New(t)
	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: types.Ptr("Test")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply(
		opt.SetString(opt.SystemPromptKey, "Be concise."),
		opt.SetFloat64(opt.TemperatureKey, 0.5),
		opt.SetFloat64(opt.TopPKey, 0.8),
		opt.SetUint(opt.TopKKey, 40),
		opt.SetUint(opt.MaxTokensKey, 1024),
		opt.AddString(opt.StopSequencesKey, "---"),
		opt.SetUint(opt.SeedKey, 7),
		opt.SetFloat64(opt.PresencePenaltyKey, 0.2),
		opt.SetFloat64(opt.FrequencyPenaltyKey, -0.2),
		opt.SetString(opt.ThinkingKey, "medium"),
	)
	a.NoError(err)

	req, err := chatRequestFromOpts("llama3.2", &session, o)
	a.NoError(err)

	// System prepended
	a.Len(req.Messages, 2)
	a.Equal("system", req.Messages[0].Role)

	// Options map
	a.InDelta(0.5, req.Options["temperature"], 1e-9)
	a.InDelta(0.8, req.Options["top_p"], 1e-9)
	a.EqualValues(40, req.Options["top_k"])
	a.EqualValues(1024, req.Options["num_predict"])
	a.Equal([]string{"---"}, req.Options["stop"])
	a.EqualValues(7, req.Options["seed"])
	a.InDelta(0.2, req.Options["presence_penalty"], 1e-9)
	a.InDelta(-0.2, req.Options["frequency_penalty"], 1e-9)

	// Think
	a.NotNil(req.Think)
	a.Equal("medium", req.Think.Value)

	// Valid JSON
	data, err := json.Marshal(req)
	a.NoError(err)
	var m map[string]any
	a.NoError(json.Unmarshal(data, &m))
	a.Equal("llama3.2", m["model"])
	a.Contains(m, "messages")
	a.Contains(m, "options")
}

func Test_chatRequest_015(t *testing.T) {
	// WithImageOutput is rejected by chatRequestFromOpts
	a := assert.New(t)
	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: types.Ptr("Hi")}}}
	session := schema.Conversation{msg}
	o, err := opt.Apply(WithImageOutput())
	a.NoError(err)

	_, err = chatRequestFromOpts("x/z-image-turbo", &session, o)
	a.Error(err)
}
