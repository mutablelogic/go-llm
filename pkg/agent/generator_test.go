package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	anthropic "github.com/mutablelogic/go-llm/pkg/provider/anthropic"
	google "github.com/mutablelogic/go-llm/pkg/provider/google"
	mistral "github.com/mutablelogic/go-llm/pkg/provider/mistral"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
)

///////////////////////////////////////////////////////////////////////////////
// MOCK GENERATOR CLIENT

// mockGeneratorClient implements both llm.Client and llm.Generator
type mockGeneratorClient struct {
	mockClient
	tokens uint
}

var _ llm.Generator = (*mockGeneratorClient)(nil)

func (c *mockGeneratorClient) WithoutSession(_ context.Context, _ schema.Model, msg *schema.Message, opts ...opt.Opt) (*schema.Message, error) {
	// Check for streaming callback
	o, err := opt.Apply(opts...)
	if err != nil {
		return nil, err
	}
	streamFn := o.GetStream()

	text := fmt.Sprintf("response(%d): %s", len(msg.Content), msg.Text())
	if streamFn != nil {
		// Deliver the response in chunks
		for i, ch := range text {
			streamFn("assistant", string(ch))
			_ = i
		}
	}

	return &schema.Message{
		Role: schema.RoleAssistant,
		Content: []schema.ContentBlock{
			{Text: types.Ptr(text)},
		},
		Result: schema.ResultOK,
		Tokens: c.tokens,
	}, nil
}

func (c *mockGeneratorClient) WithSession(_ context.Context, _ schema.Model, _ *schema.Conversation, msg *schema.Message, _ ...opt.Opt) (*schema.Message, error) {
	return &schema.Message{
		Role: schema.RoleAssistant,
		Content: []schema.ContentBlock{
			{Text: types.Ptr("chat response: " + msg.Text())},
		},
		Result: schema.ResultOK,
	}, nil
}

///////////////////////////////////////////////////////////////////////////////
// UNIT TESTS

// Test Ask with basic text input
func Test_ask_001(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockGeneratorClient{
			mockClient: mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}},
			tokens:     42,
		}),
	)
	assert.NoError(err)

	resp, err := m.Ask(context.TODO(), schema.AskRequest{
		GeneratorMeta: schema.GeneratorMeta{Model: "model-1"},
		Text:          "hello",
	}, nil)
	assert.NoError(err)
	assert.NotNil(resp)
	assert.Equal(schema.RoleAssistant, resp.Role)
	assert.Len(resp.Content, 1)
	assert.Equal("response(1): hello", *resp.Content[0].Text)
	assert.Equal(schema.ResultOK, resp.Result)
	assert.Equal(uint(42), resp.OutputTokens)
}

// Test Ask with unknown model returns error
func Test_ask_002(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockGeneratorClient{
			mockClient: mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}},
		}),
	)
	assert.NoError(err)

	_, err = m.Ask(context.TODO(), schema.AskRequest{
		GeneratorMeta: schema.GeneratorMeta{Model: "nonexistent"},
		Text:          "hello",
	}, nil)
	assert.Error(err)
	assert.ErrorIs(err, llm.ErrNotFound)
}

// Test Ask with provider filter
func Test_ask_003(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockGeneratorClient{
			mockClient: mockClient{name: "provider-1", models: []schema.Model{{Name: "shared", OwnedBy: "provider-1"}}},
		}),
		WithClient(&mockGeneratorClient{
			mockClient: mockClient{name: "provider-2", models: []schema.Model{{Name: "shared", OwnedBy: "provider-2"}}},
		}),
	)
	assert.NoError(err)

	resp, err := m.Ask(context.TODO(), schema.AskRequest{
		GeneratorMeta: schema.GeneratorMeta{Provider: "provider-2", Model: "shared"},
		Text:          "hello",
	}, nil)
	assert.NoError(err)
	assert.NotNil(resp)
	assert.Equal("response(1): hello", *resp.Content[0].Text)
}

// Test Ask with non-generator client returns error
func Test_ask_004(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}}),
	)
	assert.NoError(err)

	_, err = m.Ask(context.TODO(), schema.AskRequest{
		GeneratorMeta: schema.GeneratorMeta{Model: "model-1"},
		Text:          "hello",
	}, nil)
	assert.Error(err)
	assert.ErrorIs(err, llm.ErrNotImplemented)
}

// Test Ask with streaming callback
func Test_ask_005(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockGeneratorClient{
			mockClient: mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}},
		}),
	)
	assert.NoError(err)

	var mu sync.Mutex
	var streamed string
	fn := func(role, text string) {
		mu.Lock()
		defer mu.Unlock()
		streamed += text
	}

	resp, err := m.Ask(context.TODO(), schema.AskRequest{
		GeneratorMeta: schema.GeneratorMeta{Model: "model-1"},
		Text:          "hello",
	}, fn)
	assert.NoError(err)
	assert.NotNil(resp)

	mu.Lock()
	assert.Equal("response(1): hello", streamed)
	mu.Unlock()
	assert.Equal("response(1): hello", *resp.Content[0].Text)
}

// Test Ask with system prompt for unknown provider returns error
func Test_ask_006(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockGeneratorClient{
			mockClient: mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}},
		}),
	)
	assert.NoError(err)

	// System prompt dispatch fails for unknown provider names
	_, err = m.Ask(context.TODO(), schema.AskRequest{
		GeneratorMeta: schema.GeneratorMeta{
			Model:        "model-1",
			SystemPrompt: "You are a pirate.",
		},
		Text: "hello",
	}, nil)
	assert.Error(err)
	assert.ErrorIs(err, llm.ErrNotImplemented)
}

// Test Ask with JSON format for unknown provider returns error
func Test_ask_007(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockGeneratorClient{
			mockClient: mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}},
		}),
	)
	assert.NoError(err)

	// JSON format dispatch fails for unknown provider names
	format := json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"}}}`)
	_, err = m.Ask(context.TODO(), schema.AskRequest{
		GeneratorMeta: schema.GeneratorMeta{
			Model:  "model-1",
			Format: format,
		},
		Text: "give me a name",
	}, nil)
	assert.Error(err)
	assert.ErrorIs(err, llm.ErrNotImplemented)
}

// Test Ask with empty text
func Test_ask_008(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockGeneratorClient{
			mockClient: mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}},
		}),
	)
	assert.NoError(err)

	resp, err := m.Ask(context.TODO(), schema.AskRequest{
		GeneratorMeta: schema.GeneratorMeta{Model: "model-1"},
		Text:          "",
	}, nil)
	assert.NoError(err)
	assert.NotNil(resp)
	assert.Equal("response(1): ", *resp.Content[0].Text)
}

///////////////////////////////////////////////////////////////////////////////
// INTEGRATION TESTS

func Test_ask_integration_gemini(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("GOOGLE_API_KEY")
	}
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set, skipping")
	}
	assert := assert.New(t)

	c, err := google.New(apiKey)
	assert.NoError(err)

	m, err := NewManager(WithClient(c))
	assert.NoError(err)

	resp, err := m.Ask(context.TODO(), schema.AskRequest{
		GeneratorMeta: schema.GeneratorMeta{Model: "gemini-2.0-flash"},
		Text:          "Say hello in exactly three words.",
	}, nil)
	assert.NoError(err)
	assert.NotNil(resp)
	assert.Equal(schema.RoleAssistant, resp.Role)
	assert.NotEmpty(resp.Content)
	assert.NotNil(resp.Content[0].Text)
	t.Logf("Response: %s", *resp.Content[0].Text)
	t.Logf("Output tokens: %d", resp.OutputTokens)
}

func Test_ask_integration_gemini_stream(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("GOOGLE_API_KEY")
	}
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set, skipping")
	}
	assert := assert.New(t)

	c, err := google.New(apiKey)
	assert.NoError(err)

	m, err := NewManager(WithClient(c))
	assert.NoError(err)

	var streamed string
	fn := func(role, text string) {
		streamed += text
	}

	resp, err := m.Ask(context.TODO(), schema.AskRequest{
		GeneratorMeta: schema.GeneratorMeta{Model: "gemini-2.0-flash"},
		Text:          "Say hello in exactly three words.",
	}, fn)
	assert.NoError(err)
	assert.NotNil(resp)
	assert.NotEmpty(streamed)
	t.Logf("Streamed: %s", streamed)
}

func Test_ask_integration_anthropic(t *testing.T) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY not set, skipping")
	}
	assert := assert.New(t)

	c, err := anthropic.New(apiKey)
	assert.NoError(err)

	m, err := NewManager(WithClient(c))
	assert.NoError(err)

	resp, err := m.Ask(context.TODO(), schema.AskRequest{
		GeneratorMeta: schema.GeneratorMeta{Model: "claude-sonnet-4-20250514"},
		Text:          "Say hello in exactly three words.",
	}, nil)
	assert.NoError(err)
	assert.NotNil(resp)
	assert.Equal(schema.RoleAssistant, resp.Role)
	assert.NotEmpty(resp.Content)
	assert.NotNil(resp.Content[0].Text)
	t.Logf("Response: %s", *resp.Content[0].Text)
	t.Logf("Output tokens: %d", resp.OutputTokens)
}

func Test_ask_integration_anthropic_stream(t *testing.T) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY not set, skipping")
	}
	assert := assert.New(t)

	c, err := anthropic.New(apiKey)
	assert.NoError(err)

	m, err := NewManager(WithClient(c))
	assert.NoError(err)

	var streamed string
	fn := func(role, text string) {
		streamed += text
	}

	resp, err := m.Ask(context.TODO(), schema.AskRequest{
		GeneratorMeta: schema.GeneratorMeta{Model: "claude-sonnet-4-20250514"},
		Text:          "Say hello in exactly three words.",
	}, fn)
	assert.NoError(err)
	assert.NotNil(resp)
	assert.NotEmpty(streamed)
	t.Logf("Streamed: %s", streamed)
}

func Test_ask_integration_mistral(t *testing.T) {
	apiKey := os.Getenv("MISTRAL_API_KEY")
	if apiKey == "" {
		t.Skip("MISTRAL_API_KEY not set, skipping")
	}
	assert := assert.New(t)

	c, err := mistral.New(apiKey)
	assert.NoError(err)

	m, err := NewManager(WithClient(c))
	assert.NoError(err)

	resp, err := m.Ask(context.TODO(), schema.AskRequest{
		GeneratorMeta: schema.GeneratorMeta{Model: "mistral-small-latest"},
		Text:          "Say hello in exactly three words.",
	}, nil)
	assert.NoError(err)
	assert.NotNil(resp)
	assert.Equal(schema.RoleAssistant, resp.Role)
	assert.NotEmpty(resp.Content)
	assert.NotNil(resp.Content[0].Text)
	t.Logf("Response: %s", *resp.Content[0].Text)
	t.Logf("Output tokens: %d", resp.OutputTokens)
}

func Test_ask_integration_mistral_stream(t *testing.T) {
	apiKey := os.Getenv("MISTRAL_API_KEY")
	if apiKey == "" {
		t.Skip("MISTRAL_API_KEY not set, skipping")
	}
	assert := assert.New(t)

	c, err := mistral.New(apiKey)
	assert.NoError(err)

	m, err := NewManager(WithClient(c))
	assert.NoError(err)

	var streamed string
	fn := func(role, text string) {
		streamed += text
	}

	resp, err := m.Ask(context.TODO(), schema.AskRequest{
		GeneratorMeta: schema.GeneratorMeta{Model: "mistral-small-latest"},
		Text:          "Say hello in exactly three words.",
	}, fn)
	assert.NoError(err)
	assert.NotNil(resp)
	assert.NotEmpty(streamed)
	t.Logf("Streamed: %s", streamed)
}

func Test_ask_integration_system_prompt(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("GOOGLE_API_KEY")
	}
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set, skipping")
	}
	assert := assert.New(t)

	c, err := google.New(apiKey)
	assert.NoError(err)

	m, err := NewManager(WithClient(c))
	assert.NoError(err)

	resp, err := m.Ask(context.TODO(), schema.AskRequest{
		GeneratorMeta: schema.GeneratorMeta{
			Model:        "gemini-2.0-flash",
			SystemPrompt: "You are a pirate. Always respond in pirate speak.",
		},
		Text: "Say hello.",
	}, nil)
	assert.NoError(err)
	assert.NotNil(resp)
	assert.NotEmpty(resp.Content)
	t.Logf("Pirate response: %s", *resp.Content[0].Text)
}

///////////////////////////////////////////////////////////////////////////////
// UNIT TESTS - JSON FORMAT

// Test Ask with invalid JSON schema returns error
func Test_ask_009(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockGeneratorClient{
			mockClient: mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}},
		}),
	)
	assert.NoError(err)

	_, err = m.Ask(context.TODO(), schema.AskRequest{
		GeneratorMeta: schema.GeneratorMeta{
			Model:  "model-1",
			Format: json.RawMessage(`{not valid json`),
		},
		Text: "hello",
	}, nil)
	assert.Error(err)
	assert.ErrorIs(err, llm.ErrBadParameter)
}

// Test Ask with attachments
func Test_ask_010(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockGeneratorClient{
			mockClient: mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}},
		}),
	)
	assert.NoError(err)

	resp, err := m.Ask(context.TODO(), schema.AskRequest{
		GeneratorMeta: schema.GeneratorMeta{Model: "model-1"},
		Text:          "describe this image",
		Attachments: []schema.Attachment{
			{Type: "image/png", Data: []byte("fake-image-data")},
		},
	}, nil)
	assert.NoError(err)
	assert.NotNil(resp)
	// Mock reports content block count: 1 text + 1 attachment = 2
	assert.Equal("response(2): describe this image", *resp.Content[0].Text)
}

// Test Ask with multiple attachments
func Test_ask_011(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockGeneratorClient{
			mockClient: mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}},
		}),
	)
	assert.NoError(err)

	resp, err := m.Ask(context.TODO(), schema.AskRequest{
		GeneratorMeta: schema.GeneratorMeta{Model: "model-1"},
		Text:          "compare these",
		Attachments: []schema.Attachment{
			{Type: "image/png", Data: []byte("image-1")},
			{Type: "image/jpeg", Data: []byte("image-2")},
			{Type: "application/pdf", Data: []byte("doc-1")},
		},
	}, nil)
	assert.NoError(err)
	assert.NotNil(resp)
	// 1 text + 3 attachments = 4
	assert.Equal("response(4): compare these", *resp.Content[0].Text)
}

// Test Ask with zero attachments (same as no attachments)
func Test_ask_012(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager(
		WithClient(&mockGeneratorClient{
			mockClient: mockClient{name: "provider-1", models: []schema.Model{{Name: "model-1", OwnedBy: "provider-1"}}},
		}),
	)
	assert.NoError(err)

	resp, err := m.Ask(context.TODO(), schema.AskRequest{
		GeneratorMeta: schema.GeneratorMeta{Model: "model-1"},
		Text:          "hello",
		Attachments:   []schema.Attachment{},
	}, nil)
	assert.NoError(err)
	assert.NotNil(resp)
	assert.Equal("response(1): hello", *resp.Content[0].Text)
}

///////////////////////////////////////////////////////////////////////////////
// INTEGRATION TESTS - JSON FORMAT

func Test_ask_integration_json_gemini(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("GOOGLE_API_KEY")
	}
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set, skipping")
	}
	assert := assert.New(t)

	c, err := google.New(apiKey)
	assert.NoError(err)

	m, err := NewManager(WithClient(c))
	assert.NoError(err)

	format := json.RawMessage(`{
		"type": "object",
		"properties": {
			"capital": { "type": "string" },
			"country": { "type": "string" }
		},
		"required": ["capital", "country"]
	}`)

	resp, err := m.Ask(context.TODO(), schema.AskRequest{
		GeneratorMeta: schema.GeneratorMeta{
			Model:  "gemini-2.0-flash",
			Format: format,
		},
		Text: "What is the capital of France? Respond with JSON.",
	}, nil)
	if !assert.NoError(err) || !assert.NotNil(resp) {
		return
	}
	assert.NotEmpty(resp.Content)
	assert.NotNil(resp.Content[0].Text)

	// Verify the response is valid JSON with expected fields
	var result map[string]any
	err = json.Unmarshal([]byte(*resp.Content[0].Text), &result)
	assert.NoError(err)
	assert.Contains(result, "capital")
	assert.Contains(result, "country")
	t.Logf("JSON response: %s", *resp.Content[0].Text)
}

func Test_ask_integration_json_anthropic(t *testing.T) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY not set, skipping")
	}
	assert := assert.New(t)

	c, err := anthropic.New(apiKey)
	assert.NoError(err)

	m, err := NewManager(WithClient(c))
	assert.NoError(err)

	format := json.RawMessage(`{
		"type": "object",
		"properties": {
			"capital": { "type": "string" },
			"country": { "type": "string" }
		},
		"required": ["capital", "country"],
		"additionalProperties": false
	}`)

	resp, err := m.Ask(context.TODO(), schema.AskRequest{
		GeneratorMeta: schema.GeneratorMeta{
			Model:  "claude-haiku-4-5-20251001",
			Format: format,
		},
		Text: "What is the capital of France? Respond with JSON.",
	}, nil)
	if !assert.NoError(err) || !assert.NotNil(resp) {
		return
	}
	assert.NotEmpty(resp.Content)
	assert.NotNil(resp.Content[0].Text)

	var result map[string]any
	err = json.Unmarshal([]byte(*resp.Content[0].Text), &result)
	assert.NoError(err)
	assert.Contains(result, "capital")
	assert.Contains(result, "country")
	t.Logf("JSON response: %s", *resp.Content[0].Text)
}

func Test_ask_integration_json_mistral(t *testing.T) {
	apiKey := os.Getenv("MISTRAL_API_KEY")
	if apiKey == "" {
		t.Skip("MISTRAL_API_KEY not set, skipping")
	}
	assert := assert.New(t)

	c, err := mistral.New(apiKey)
	assert.NoError(err)

	m, err := NewManager(WithClient(c))
	assert.NoError(err)

	format := json.RawMessage(`{
		"type": "object",
		"properties": {
			"capital": { "type": "string" },
			"country": { "type": "string" }
		},
		"required": ["capital", "country"]
	}`)

	resp, err := m.Ask(context.TODO(), schema.AskRequest{
		GeneratorMeta: schema.GeneratorMeta{
			Model:  "mistral-small-latest",
			Format: format,
		},
		Text: "What is the capital of France? Respond with JSON.",
	}, nil)
	if !assert.NoError(err) || !assert.NotNil(resp) {
		return
	}
	assert.NotEmpty(resp.Content)
	assert.NotNil(resp.Content[0].Text)

	var result map[string]any
	err = json.Unmarshal([]byte(*resp.Content[0].Text), &result)
	assert.NoError(err)
	assert.Contains(result, "capital")
	assert.Contains(result, "country")
	t.Logf("JSON response: %s", *resp.Content[0].Text)
}

///////////////////////////////////////////////////////////////////////////////
// INTEGRATION TESTS - ATTACHMENTS

func Test_ask_integration_attachment_gemini(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("GOOGLE_API_KEY")
	}
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set, skipping")
	}
	assert := assert.New(t)

	c, err := google.New(apiKey)
	assert.NoError(err)

	m, err := NewManager(WithClient(c))
	assert.NoError(err)

	data, err := os.ReadFile("../../etc/testdata/guggenheim.jpg")
	if !assert.NoError(err) {
		return
	}

	resp, err := m.Ask(context.TODO(), schema.AskRequest{
		GeneratorMeta: schema.GeneratorMeta{Model: "gemini-2.0-flash"},
		Text:          "Describe this image in one sentence.",
		Attachments: []schema.Attachment{
			{Type: "image/jpeg", Data: data},
		},
	}, nil)
	if !assert.NoError(err) || !assert.NotNil(resp) {
		return
	}
	assert.NotEmpty(resp.Content)
	assert.NotNil(resp.Content[0].Text)
	t.Logf("Image description: %s", *resp.Content[0].Text)
}

func Test_ask_integration_attachment_anthropic(t *testing.T) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY not set, skipping")
	}
	assert := assert.New(t)

	c, err := anthropic.New(apiKey)
	assert.NoError(err)

	m, err := NewManager(WithClient(c))
	assert.NoError(err)

	data, err := os.ReadFile("../../etc/testdata/guggenheim.jpg")
	if !assert.NoError(err) {
		return
	}

	resp, err := m.Ask(context.TODO(), schema.AskRequest{
		GeneratorMeta: schema.GeneratorMeta{Model: "claude-sonnet-4-20250514"},
		Text:          "Describe this image in one sentence.",
		Attachments: []schema.Attachment{
			{Type: "image/jpeg", Data: data},
		},
	}, nil)
	if !assert.NoError(err) || !assert.NotNil(resp) {
		return
	}
	assert.NotEmpty(resp.Content)
	assert.NotNil(resp.Content[0].Text)
	t.Logf("Image description: %s", *resp.Content[0].Text)
}

func Test_ask_integration_attachment_mistral(t *testing.T) {
	apiKey := os.Getenv("MISTRAL_API_KEY")
	if apiKey == "" {
		t.Skip("MISTRAL_API_KEY not set, skipping")
	}
	assert := assert.New(t)

	c, err := mistral.New(apiKey)
	assert.NoError(err)

	m, err := NewManager(WithClient(c))
	assert.NoError(err)

	data, err := os.ReadFile("../../etc/testdata/guggenheim.jpg")
	if !assert.NoError(err) {
		return
	}

	resp, err := m.Ask(context.TODO(), schema.AskRequest{
		GeneratorMeta: schema.GeneratorMeta{Model: "mistral-small-latest"},
		Text:          "Describe this image in one sentence.",
		Attachments: []schema.Attachment{
			{Type: "image/jpeg", Data: data},
		},
	}, nil)
	if !assert.NoError(err) || !assert.NotNil(resp) {
		return
	}
	assert.NotEmpty(resp.Content)
	assert.NotNil(resp.Content[0].Text)
	t.Logf("Image description: %s", *resp.Content[0].Text)
}
