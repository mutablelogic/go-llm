package anthropic

import (
	"context"
	"crypto/sha256"
	"sync"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// tokenCounts are the tokens of text which were counted, by the model and
// the text's hash, so the same text isn't counted again, and, by model, the
// tokens of what's counted with text, which is a message, apart from its
// text
type tokenCounts struct {
	mu       sync.Mutex
	text     map[countKey]uint
	overhead map[string]uint
}

// countKey is the model, and the hash of the text, whose tokens were
// counted
type countKey struct {
	model string
	hash  [sha256.Size]byte
}

var _ llm.Measurer = (*Client)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// maxCounts is the most tokens of text which are kept, after which they're
// forgotten, and counted again
const maxCounts = 4096

// estimateBytes is how many bytes of text a token is, as an estimate, when
// the tokens can't be counted
const estimateBytes = 4

// overheadText is the text whose tokens are counted, to find the tokens of
// what's counted with text, which is one token
const overheadText = "."

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Measure returns how many tokens the text is, for the model, which must be
// a model of this client, by counting the tokens of a message from the user
// with the text, less those of the message apart from its text. Text which
// was counted before isn't counted again. Empty text is none. When the
// tokens can't be counted, such as when counting is rate limited, they're
// an estimate, of a token for every estimateBytes bytes, so what's sent to
// the model is still fitted to its context, unless the context is done.
func (c *Client) Measure(ctx context.Context, model llm.Model, text string) (uint, error) {
	m, ok := model.(*Model)
	if !ok || m.provider != c {
		return 0, llm.ErrBadParameter.With("not a model of this provider")
	} else if text == "" {
		return 0, nil
	}

	// The tokens of text which were counted before
	key := countKey{model: m.Name(), hash: sha256.Sum256([]byte(text))}
	if tokens, ok := c.counts.get(key); ok {
		return tokens, nil
	}

	// The tokens of what's counted with the text, which are counted once
	overhead, ok := c.counts.getOverhead(m.Name())
	if !ok {
		tokens, err := c.countTokens(ctx, m.Name(), overheadText)
		if err != nil {
			return estimate(ctx, text, err)
		}
		overhead = tokens - min(tokens, 1)
		c.counts.setOverhead(m.Name(), overhead)
	}

	// Count the tokens of the text, with what's counted with it
	tokens, err := c.countTokens(ctx, m.Name(), text)
	if err != nil {
		return estimate(ctx, text, err)
	}
	tokens -= min(tokens, overhead)
	c.counts.set(key, tokens)
	return tokens, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// estimate returns an estimate of the tokens of the text, which couldn't be
// counted, or the error, when the context is done
func estimate(ctx context.Context, text string, err error) (uint, error) {
	if ctx.Err() != nil {
		return 0, err
	}
	return uint(len(text)+estimateBytes-1) / estimateBytes, nil
}

// countTokens returns the tokens of a message from the user with the text,
// for the model, with POST /v1/messages/count_tokens
func (c *Client) countTokens(ctx context.Context, model, text string) (uint, error) {
	request := messagesRequest{Model: model, Messages: []message{{Role: "user", Content: []contentBlock{{Type: "text", Text: text}}}}}
	payload, err := client.NewJSONRequest(request)
	if err != nil {
		return 0, err
	}
	var response countResponse
	if err := c.DoWithContext(ctx, payload, &response, client.OptPath("messages", "count_tokens")); err != nil {
		return 0, err
	}
	return response.InputTokens, nil
}

// get returns the tokens of the text with the key, when they were counted
func (t *tokenCounts) get(key countKey) (uint, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	tokens, ok := t.text[key]
	return tokens, ok
}

// set keeps the tokens of the text with the key, forgetting the others,
// when there are too many
func (t *tokenCounts) set(key countKey, tokens uint) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.text == nil || len(t.text) >= maxCounts {
		t.text = make(map[countKey]uint)
	}
	t.text[key] = tokens
}

// getOverhead returns the tokens of what's counted with text, for the
// model, when they were counted
func (t *tokenCounts) getOverhead(model string) (uint, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	tokens, ok := t.overhead[model]
	return tokens, ok
}

// setOverhead keeps the tokens of what's counted with text, for the model
func (t *tokenCounts) setOverhead(model string, tokens uint) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.overhead == nil {
		t.overhead = make(map[string]uint)
	}
	t.overhead[model] = tokens
}
