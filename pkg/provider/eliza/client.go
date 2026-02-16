/*
eliza implements a mock LLM provider based on the classic ELIZA chatbot
created by Joseph Weizenbaum at MIT in 1966. It simulates a
psychotherapist using pattern matching and transformation rules.
It requires no API key or network access.
*/
package eliza

import (
	"context"
	"fmt"
	"sort"
	"strings"
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

	// Create one engine per language
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
	// Try provider name as alias (prefer English, then sort for determinism)
	if name == providerName && len(c.languages) > 0 {
		if lang, ok := c.languages["eliza-1966-en"]; ok {
			return types.Ptr(langModel(lang)), nil
		}
		// Fall back to first model alphabetically
		keys := make([]string, 0, len(c.languages))
		for k := range c.languages {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return types.Ptr(langModel(c.languages[keys[0]])), nil
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

	// Create a fresh engine for this stateless request
	lang, ok := c.languages[model.Name]
	if !ok {
		return nil, nil, llm.ErrNotFound.Withf("no engine for model %q", model.Name)
	}
	engine, err := NewEngine(lang, c.seed)
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
		streamWords(streamFn, response)
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

	// Look up the engine for this model and infer memory from conversation history
	engine, ok := c.engines[model.Name]
	if !ok {
		return nil, nil, llm.ErrNotFound.Withf("no engine for model %q", model.Name)
	}

	// Extract and validate text before mutating the session
	input := message.Text()
	if input == "" {
		return nil, nil, llm.ErrBadParameter.With("message text is required")
	}

	// Infer memory from conversation history, then append the user message
	engine.InferMemory(*session)
	session.Append(*message)

	// Parse options
	options, err := opt.Apply(opts...)
	if err != nil {
		return nil, nil, err
	}

	// If thinking is enabled, emit memory as thinking output
	var thinkingText string
	if options.GetBool(opt.ThinkingKey) {
		thinkingText = formatMemory(engine.Memory())
		if streamFn := options.GetStream(); streamFn != nil && thinkingText != "" {
			streamFn(schema.RoleThinking, thinkingText)
		}
	}

	// Generate response using the stateful engine
	response := engine.Response(input)

	// Create response message
	content := make([]schema.ContentBlock, 0, 2)
	if thinkingText != "" {
		content = append(content, schema.ContentBlock{Thinking: types.Ptr(thinkingText)})
	}
	content = append(content, schema.ContentBlock{Text: types.Ptr(response)})
	responseMsg := &schema.Message{
		Role:    schema.RoleAssistant,
		Content: content,
		Result:  schema.ResultStop,
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
	if streamFn := options.GetStream(); streamFn != nil {
		streamWords(streamFn, response)
	}

	return responseMsg, usage, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func langModel(lang *Language) schema.Model {
	return schema.Model{
		Name:        lang.Model,
		Description: lang.Description,
		Created:     time.Date(1966, 1, 1, 0, 0, 0, 0, time.UTC),
		OwnedBy:     providerName,
		Meta: map[string]any{
			"author":      "Joseph Weizenbaum",
			"institution": "MIT",
			"year":        1966,
			"language":    lang.LanguageCode,
		},
	}
}

// streamWords delivers a response word-by-word to the streaming callback,
// simulating chunked delivery as real LLM providers do.
func streamWords(fn opt.StreamFn, response string) {
	words := strings.Fields(response)
	for i, word := range words {
		if i > 0 {
			word = " " + word
		}
		fn(schema.RoleAssistant, word)
	}
}

// formatMemory returns a human-readable summary of the engine's accumulated
// memory, suitable for emitting as thinking output.
func formatMemory(memory []string) string {
	if len(memory) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Recalled memories from conversation:\n")
	for i, m := range memory {
		fmt.Fprintf(&b, "  %d. %s\n", i+1, m)
	}
	return b.String()
}
