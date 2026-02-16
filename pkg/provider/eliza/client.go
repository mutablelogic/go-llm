/*
eliza implements a mock LLM provider based on the classic ELIZA chatbot
created by Joseph Weizenbaum at MIT in 1966. It simulates a
psychotherapist using pattern matching and transformation rules.
It requires no API key or network access.
*/
package eliza

import (
	"context"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Client implements the ELIZA chatbot as an LLM provider
type Client struct {
	languages map[string]*Language // keyed by model name
	engines   map[string]*Engine   // keyed by model name
	seed      int64
}

// Ensure Client implements the required interfaces
var _ llm.Client = (*Client)(nil)
var _ llm.Generator = (*Client)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// Provider name
	providerName = "eliza"
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// Opt is a functional option for configuring the ELIZA client
type Opt func(*Client) error

// New creates a new ELIZA client
func New(opts ...Opt) (*Client, error) {
	// Load all embedded language files
	languages, err := LoadLanguages()
	if err != nil {
		return nil, err
	}

	c := &Client{
		languages: languages,
		seed:      time.Now().UnixNano(),
	}

	// Apply options
	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, err
		}
	}

	// Create engines for each language
	c.engines = make(map[string]*Engine, len(c.languages))
	for name, lang := range c.languages {
		engine, err := NewEngine(lang, c.seed)
		if err != nil {
			return nil, err
		}
		c.engines[name] = engine
	}

	return c, nil
}

// WithSeed sets a specific random seed for reproducible responses
func WithSeed(seed int64) Opt {
	return func(c *Client) error {
		c.seed = seed
		return nil
	}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - llm.Client

// Name returns the provider name
func (*Client) Name() string {
	return providerName
}

// ListModels returns the available models
func (c *Client) ListModels(ctx context.Context, opts ...opt.Opt) ([]schema.Model, error) {
	models := make([]schema.Model, 0, len(c.languages))
	for _, lang := range c.languages {
		models = append(models, langModel(lang))
	}
	return models, nil
}

// GetModel returns an ELIZA model by name
func (c *Client) GetModel(ctx context.Context, name string, opts ...opt.Opt) (*schema.Model, error) {
	// Try exact match first
	if lang, ok := c.languages[name]; ok {
		return types.Ptr(langModel(lang)), nil
	}
	// Try provider name as alias (return first available model)
	if name == providerName && len(c.languages) > 0 {
		for _, lang := range c.languages {
			return types.Ptr(langModel(lang)), nil
		}
	}
	// List available model names for the error message
	names := make([]string, 0, len(c.languages))
	for n := range c.languages {
		names = append(names, n)
	}
	return nil, llm.ErrNotFound.Withf("model %q not found (available: %v)", name, names)
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - llm.Generator

// WithoutSession sends a single message and returns the response (stateless)
func (c *Client) WithoutSession(ctx context.Context, model schema.Model, message *schema.Message, opts ...opt.Opt) (*schema.Message, *schema.Usage, error) {
	if message == nil {
		return nil, nil, llm.ErrBadParameter.With("message is required")
	}

	engine, err := c.engineForModel(model.Name)
	if err != nil {
		return nil, nil, err
	}

	// Extract text from the message
	input := message.Text()
	if input == "" {
		return nil, nil, llm.ErrBadParameter.With("message text is required")
	}

	// Generate response using the engine
	response := engine.Response(input)

	// Create response message
	responseMsg := &schema.Message{
		Role: schema.RoleAssistant,
		Content: []schema.ContentBlock{
			{Text: types.Ptr(response)},
		},
		Result: schema.ResultStop,
	}

	// Estimate token usage (ELIZA doesn't have real tokens, but we estimate)
	inputTokens := uint(len(input)+3) / 4 // ~4 chars per token
	outputTokens := uint(len(response)+3) / 4

	usage := &schema.Usage{
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
	}

	// Handle streaming callback if provided
	options, err := opt.Apply(opts...)
	if err != nil {
		return nil, nil, err
	}
	if streamFn := options.GetStream(); streamFn != nil {
		streamFn(schema.RoleAssistant, response)
	}

	return responseMsg, usage, nil
}

// WithSession sends a message within a session and returns the response (stateful)
func (c *Client) WithSession(ctx context.Context, model schema.Model, session *schema.Conversation, message *schema.Message, opts ...opt.Opt) (*schema.Message, *schema.Usage, error) {
	if session == nil {
		return nil, nil, llm.ErrBadParameter.With("session is required")
	}
	if message == nil {
		return nil, nil, llm.ErrBadParameter.With("message is required")
	}

	engine, err := c.engineForModel(model.Name)
	if err != nil {
		return nil, nil, err
	}

	// Append the user message to the session
	session.Append(*message)

	// Extract text from the message
	input := message.Text()
	if input == "" {
		return nil, nil, llm.ErrBadParameter.With("message text is required")
	}

	// Generate response using the stateful engine
	response := engine.Response(input)

	// Create response message
	responseMsg := &schema.Message{
		Role: schema.RoleAssistant,
		Content: []schema.ContentBlock{
			{Text: types.Ptr(response)},
		},
		Result: schema.ResultStop,
	}

	// Estimate token usage
	inputTokens := uint(len(input)+3) / 4
	outputTokens := uint(len(response)+3) / 4

	// Set tokens on the input message
	if n := len(*session); n > 0 && (*session)[n-1].Tokens == 0 {
		(*session)[n-1].Tokens = (*session)[n-1].EstimateTokens()
	}

	// Set tokens on the response message and append
	responseMsg.Tokens = outputTokens
	session.Append(*responseMsg)

	usage := &schema.Usage{
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
	}

	// Handle streaming callback if provided
	options, err := opt.Apply(opts...)
	if err != nil {
		return nil, nil, err
	}
	if streamFn := options.GetStream(); streamFn != nil {
		streamFn(schema.RoleAssistant, response)
	}

	return responseMsg, usage, nil
}

// Reset clears the conversation memory in all ELIZA engines
func (c *Client) Reset() {
	for _, engine := range c.engines {
		engine.Reset()
	}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func (c *Client) engineForModel(name string) (*Engine, error) {
	if engine, ok := c.engines[name]; ok {
		return engine, nil
	}
	return nil, llm.ErrNotFound.Withf("no engine for model %q", name)
}

func langModel(lang *Language) schema.Model {
	return schema.Model{
		Name:        lang.Model,
		Description: lang.Description,
		Created:     time.Date(1966, 1, 1, 0, 0, 0, 0, time.UTC),
		OwnedBy:    providerName,
		Meta: map[string]any{
			"author":      "Joseph Weizenbaum",
			"institution": "MIT",
			"year":        1966,
			"language":    lang.LanguageCode,
		},
	}
}
