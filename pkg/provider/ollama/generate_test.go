package ollama

import (
	"encoding/json"
	"testing"

	// Packages
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
)

///////////////////////////////////////////////////////////////////////////////
// UNIT TESTS — generateRequestFromOpts

func Test_generateRequest_001(t *testing.T) {
	// Minimal request: model name and single user text message
	a := assert.New(t)
	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: types.Ptr("Hello")}}}
	o, err := opt.Apply()
	a.NoError(err)

	req, err := generateRequestFromOpts("llama3.2", msg, o)
	a.NoError(err)
	a.NotNil(req)
	a.Equal("llama3.2", req.Model)
	a.Equal("Hello", req.Prompt)
	a.Empty(req.System)
	a.Nil(req.Options)
	a.Empty(req.Format)
	a.Nil(req.Images)
}

func Test_generateRequest_002(t *testing.T) {
	// System prompt is set on the request
	a := assert.New(t)
	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: types.Ptr("Hi")}}}
	o, err := opt.Apply(opt.SetString(opt.SystemPromptKey, "You are helpful."))
	a.NoError(err)

	req, err := generateRequestFromOpts("llama3.2", msg, o)
	a.NoError(err)
	a.Equal("You are helpful.", req.System)
}

func Test_generateRequest_003(t *testing.T) {
	// Temperature is placed in the options map
	a := assert.New(t)
	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: types.Ptr("Hi")}}}
	o, err := opt.Apply(opt.SetFloat64(opt.TemperatureKey, 0.7))
	a.NoError(err)

	req, err := generateRequestFromOpts("llama3.2", msg, o)
	a.NoError(err)
	a.NotNil(req.Options)
	a.InDelta(0.7, req.Options["temperature"], 1e-9)
}

func Test_generateRequest_004(t *testing.T) {
	// Top P is placed in the options map
	a := assert.New(t)
	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: types.Ptr("Hi")}}}
	o, err := opt.Apply(opt.SetFloat64(opt.TopPKey, 0.9))
	a.NoError(err)

	req, err := generateRequestFromOpts("llama3.2", msg, o)
	a.NoError(err)
	a.InDelta(0.9, req.Options["top_p"], 1e-9)
}

func Test_generateRequest_005(t *testing.T) {
	// Top K is placed in the options map
	a := assert.New(t)
	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: types.Ptr("Hi")}}}
	o, err := opt.Apply(opt.SetUint(opt.TopKKey, 40))
	a.NoError(err)

	req, err := generateRequestFromOpts("llama3.2", msg, o)
	a.NoError(err)
	a.EqualValues(40, req.Options["top_k"])
}

func Test_generateRequest_006(t *testing.T) {
	// Max tokens maps to num_predict in the options map
	a := assert.New(t)
	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: types.Ptr("Hi")}}}
	o, err := opt.Apply(opt.SetUint(opt.MaxTokensKey, 2048))
	a.NoError(err)

	req, err := generateRequestFromOpts("llama3.2", msg, o)
	a.NoError(err)
	a.EqualValues(2048, req.Options["num_predict"])
}

func Test_generateRequest_007(t *testing.T) {
	// Stop sequences are placed in the options map
	a := assert.New(t)
	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: types.Ptr("Hi")}}}
	o, err := opt.Apply(opt.AddString(opt.StopSequencesKey, "STOP", "END"))
	a.NoError(err)

	req, err := generateRequestFromOpts("llama3.2", msg, o)
	a.NoError(err)
	a.Equal([]string{"STOP", "END"}, req.Options["stop"])
}

func Test_generateRequest_008(t *testing.T) {
	// Seed is placed in the options map
	a := assert.New(t)
	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: types.Ptr("Hi")}}}
	o, err := opt.Apply(opt.SetUint(opt.SeedKey, 99))
	a.NoError(err)

	req, err := generateRequestFromOpts("llama3.2", msg, o)
	a.NoError(err)
	a.EqualValues(99, req.Options["seed"])
}

func Test_generateRequest_009(t *testing.T) {
	// Presence and frequency penalty are placed in the options map
	a := assert.New(t)
	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: types.Ptr("Hi")}}}
	o, err := opt.Apply(
		opt.SetFloat64(opt.PresencePenaltyKey, 0.3),
		opt.SetFloat64(opt.FrequencyPenaltyKey, -0.3),
	)
	a.NoError(err)

	req, err := generateRequestFromOpts("llama3.2", msg, o)
	a.NoError(err)
	a.InDelta(0.3, req.Options["presence_penalty"], 1e-9)
	a.InDelta(-0.3, req.Options["frequency_penalty"], 1e-9)
}

func Test_generateRequest_010(t *testing.T) {
	// JSON schema format is set on the request
	a := assert.New(t)
	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: types.Ptr("Hi")}}}
	schemaJSON := `{"type":"object","properties":{"answer":{"type":"string"}}}`
	o, err := opt.Apply(opt.SetString(opt.JSONSchemaKey, schemaJSON))
	a.NoError(err)

	req, err := generateRequestFromOpts("llama3.2", msg, o)
	a.NoError(err)
	a.NotEmpty(req.Format)
	var m map[string]any
	a.NoError(json.Unmarshal(req.Format, &m))
	a.Equal("object", m["type"])
}

func Test_generateRequest_011(t *testing.T) {
	// Multipart text is joined with newlines into the prompt
	a := assert.New(t)
	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{
		{Text: types.Ptr("First.")},
		{Text: types.Ptr("Second.")},
	}}
	o, err := opt.Apply()
	a.NoError(err)

	req, err := generateRequestFromOpts("llama3.2", msg, o)
	a.NoError(err)
	a.Equal("First.\nSecond.", req.Prompt)
}

func Test_generateRequest_012(t *testing.T) {
	// Image attachments populate the Images field
	a := assert.New(t)
	pngBytes := []byte("\x89PNG\r\n\x1a\n" + "fake")
	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{
		{Text: types.Ptr("What is this?")},
		{Attachment: &schema.Attachment{ContentType: "image/png", Data: pngBytes}},
	}}
	o, err := opt.Apply()
	a.NoError(err)

	req, err := generateRequestFromOpts("llama3.2", msg, o)
	a.NoError(err)
	a.Equal("What is this?", req.Prompt)
	a.Len(req.Images, 1)
	a.Equal(pngBytes, req.Images[0])
}

func Test_generateRequest_013(t *testing.T) {
	// All options combined serialize to valid JSON
	a := assert.New(t)
	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: types.Ptr("Test")}}}
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
	)
	a.NoError(err)

	req, err := generateRequestFromOpts("llama3.2", msg, o)
	a.NoError(err)

	a.Equal("Be concise.", req.System)
	a.InDelta(0.5, req.Options["temperature"], 1e-9)
	a.InDelta(0.8, req.Options["top_p"], 1e-9)
	a.EqualValues(40, req.Options["top_k"])
	a.EqualValues(1024, req.Options["num_predict"])
	a.Equal([]string{"---"}, req.Options["stop"])
	a.EqualValues(7, req.Options["seed"])
	a.InDelta(0.2, req.Options["presence_penalty"], 1e-9)
	a.InDelta(-0.2, req.Options["frequency_penalty"], 1e-9)

	// Valid JSON
	data, err := json.Marshal(req)
	a.NoError(err)
	var m map[string]any
	a.NoError(json.Unmarshal(data, &m))
	a.Equal("llama3.2", m["model"])
	a.Contains(m, "system")
	a.Contains(m, "options")
}

func Test_generateRequest_014(t *testing.T) {
	// Tools (toolkit) are rejected by generateRequestFromOpts
	a := assert.New(t)
	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: types.Ptr("Hi")}}}
	o, err := opt.Apply(opt.SetAny(opt.ToolkitKey, struct{}{}))
	a.NoError(err)

	_, err = generateRequestFromOpts("llama3.2", msg, o)
	a.Error(err)
}

func Test_generateRequest_015(t *testing.T) {
	// Tool choice is rejected by generateRequestFromOpts
	a := assert.New(t)
	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: types.Ptr("Hi")}}}
	o, err := opt.Apply(opt.SetString(opt.ToolChoiceKey, "auto"))
	a.NoError(err)

	_, err = generateRequestFromOpts("llama3.2", msg, o)
	a.Error(err)
}

func Test_generateRequest_016(t *testing.T) {
	// Thinking is rejected by generateRequestFromOpts
	a := assert.New(t)
	msg := &schema.Message{Role: "user", Content: []schema.ContentBlock{{Text: types.Ptr("Hi")}}}
	o, err := opt.Apply(opt.SetString(opt.ThinkingKey, "high"))
	a.NoError(err)

	_, err = generateRequestFromOpts("llama3.2", msg, o)
	a.Error(err)
}
