/*
eliza implements a mock LLM provider based on the classic ELIZA chatbot,
created by Joseph Weizenbaum at MIT in 1966, which simulates a
psychotherapist using pattern matching and transformation rules. It has a
model for each of its languages, and makes no network calls at all, so it's
always reachable, which is useful for exercising provider code paths (like
connectivity checks, and generating messages) without depending on a real
LLM backend.
*/
package eliza

import (
	"context"
	"net/url"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Client implements the ELIZA chatbot as an LLM provider
type Client struct {
	languages map[string]*Language // keyed by model name
	seed      int64
}

// Opt is an option for creating the eliza client
type Opt func(*Client) error

var _ llm.Provider = (*Client)(nil)
var _ llm.Generator = (*Client)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New creates a new eliza client, with its languages. It takes no endpoint,
// since it makes no network calls at all.
func New(opts ...Opt) (*Client, error) {
	languages, err := LoadLanguages()
	if err != nil {
		return nil, err
	}
	c := &Client{languages: languages, seed: time.Now().UnixNano()}
	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, err
		}
	}

	// Check the rules of the languages compile
	for _, lang := range c.languages {
		if _, err := NewEngine(lang, c.seed); err != nil {
			return nil, err
		}
	}

	// Return success
	return c, nil
}

// WithSeed sets the seed of the replies, so they're the same each time
func WithSeed(seed int64) Opt {
	return func(c *Client) error {
		c.seed = seed
		return nil
	}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Name returns the provider name
func (*Client) Name() string {
	return llm.Eliza
}

// URL returns nil: eliza has no endpoint.
func (*Client) URL() *url.URL {
	return nil
}

// Self returns the underlying client implementation.
func (c *Client) Self() llm.Provider {
	return c
}

// Ping always succeeds: eliza makes no network calls.
func (*Client) Ping(ctx context.Context) error {
	return nil
}
