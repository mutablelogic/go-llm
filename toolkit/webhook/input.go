package webhook

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	// Packages
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	schema "github.com/mutablelogic/go-llm/webhook/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// agentInput is the input of the agent which handles calls
type agentInput struct {
	Webhooks []webhookInput `json:"webhooks"`
	Calls    []callInput    `json:"calls"`
	Context  string         `json:"context"`
}

// webhookInput is a webhook, as the agent is told it
type webhookInput struct {
	ID          uint64 `json:"id"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

// callInput is a call, as the agent is told it, with what was sent, or how
// long it was, when it isn't text
type callInput struct {
	ID        uint64              `json:"id"`
	Webhook   uint64              `json:"webhook"`
	Received  string              `json:"received"`
	Event     string              `json:"event,omitempty"`
	Signature string              `json:"signature"`
	Test      bool                `json:"test,omitempty"`
	Query     map[string][]string `json:"query,omitempty"`
	Values    json.RawMessage     `json:"values,omitempty"`
	Bytes     int                 `json:"bytes,omitempty"`
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// Agent is the name of the built-in agent which handles calls
	Agent = "webhook"

	// valuesTokens is the most tokens of what was sent, as estimated, which
	// the calls which are handled at once share, so they fit
	valuesTokens = 12000

	// tokenBytes is how many bytes a token of JSON is, as estimated
	tokenBytes = 3
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Input returns the input of the agent which handles the calls, as JSON: the
// webhooks, and the calls, with what was sent, whose values are shortened,
// when they're long, without the tokens of webhooks' URLs, and what the
// system says, one line each, such as the time
func (c *Calls) Input(ctx context.Context, lines []string) (json.RawMessage, error) {
	input := agentInput{Webhooks: []webhookInput{}, Calls: []callInput{}, Context: strings.Join(lines, "\n")}
	for _, webhook := range c.Webhooks {
		input.Webhooks = append(input.Webhooks, webhookInput{ID: webhook.ID, Type: webhook.Type, Description: webhook.Description})
	}
	limit := uint(valuesTokens / max(1, len(c.Calls)))
	for _, call := range c.Calls {
		result := callInput{
			ID:        call.ID,
			Webhook:   call.Webhook,
			Received:  call.Received.UTC().Format(time.RFC3339),
			Event:     schema.HideToken(call.Event),
			Signature: signature(call),
			Test:      call.Test,
		}

		// Its query, when it's short, as it's part of a URL, which isn't
		// shortened, and its values, or how long what isn't text was
		query := hideTokens(call.Query)
		if data, err := json.Marshal(query); err != nil {
			return nil, err
		} else if uint(len(data)) <= limit*tokenBytes {
			result.Query = query
		}
		if len(call.Values) == 0 {
			result.Bytes = len(call.Body)
		} else if values, err := hideTokenValues(call.Values); err != nil {
			return nil, err
		} else if result.Values, err = toolkit.TruncateResult(ctx, values, limit, estimate); err != nil {
			return nil, err
		}
		input.Calls = append(input.Calls, result)
	}
	return json.Marshal(input)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// estimate returns how many tokens the text is, as estimated
func estimate(_ context.Context, text string) (uint, error) {
	return uint(len(text)+tokenBytes-1) / tokenBytes, nil
}
