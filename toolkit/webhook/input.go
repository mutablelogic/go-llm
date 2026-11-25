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
	ID        uint64          `json:"id"`
	Webhook   uint64          `json:"webhook"`
	Received  string          `json:"received"`
	Event     string          `json:"event,omitempty"`
	Signature string          `json:"signature"`
	Test      bool            `json:"test,omitempty"`
	Query     json.RawMessage `json:"query,omitempty"`
	Values    json.RawMessage `json:"values,omitempty"`
	Bytes     int             `json:"bytes,omitempty"`
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// Agent is the name of the built-in agent which handles calls
	Agent = "webhook"

	// sentTokens is the most tokens of what was sent, as estimated, which
	// the calls which are handled at once share, so they fit
	sentTokens = 12000

	// queryShare is the share of a call's tokens which its query has, as a
	// fraction, and what it doesn't need is left for its values
	queryShare = 4

	// eventLimit is the most characters of a call's event, which a service
	// can take from what was sent
	eventLimit = 200

	// tokenBytes is how many bytes a token of JSON is, as estimated
	tokenBytes = 3
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Input returns the input of the agent which handles the calls, as JSON: the
// webhooks, and the calls, with what was sent, which is shortened, when it's
// long, without the tokens of webhooks' URLs, and what the
// system says, one line each, such as the time
func (c *Calls) Input(ctx context.Context, lines []string) (json.RawMessage, error) {
	input := agentInput{Webhooks: []webhookInput{}, Calls: []callInput{}, Context: strings.Join(lines, "\n")}
	for _, webhook := range c.Webhooks {
		input.Webhooks = append(input.Webhooks, webhookInput{ID: webhook.ID, Type: webhook.Type, Description: webhook.Description})
	}
	limit := uint(sentTokens / max(1, len(c.Calls)))
	for _, call := range c.Calls {
		result := callInput{
			ID:        call.ID,
			Webhook:   call.Webhook,
			Received:  call.Received.UTC().Format(time.RFC3339),
			Event:     shorten(schema.HideToken(call.Event), eventLimit),
			Signature: signature(call),
			Test:      call.Test,
		}

		// Its query, and its values, with the tokens its query left, or how
		// long what isn't text was
		values := limit
		if len(call.Query) > 0 {
			query, err := json.Marshal(hideTokens(call.Query))
			if err != nil {
				return nil, err
			} else if result.Query, err = toolkit.TruncateResult(ctx, query, max(1, limit/queryShare), estimate); err != nil {
				return nil, err
			}
			used, _ := estimate(ctx, string(result.Query))
			values = max(1, limit-min(used, limit))
		}
		if len(call.Values) == 0 {
			result.Bytes = len(call.Body)
		} else if data, err := hideTokenValues(call.Values); err != nil {
			return nil, err
		} else if result.Values, err = toolkit.TruncateResult(ctx, data, values, estimate); err != nil {
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

// shorten returns the text, up to the limit of characters, with an ellipsis,
// when it's longer
func shorten(text string, limit int) string {
	if runes := []rune(text); len(runes) > limit {
		return string(runes[:limit-1]) + "…"
	}
	return text
}
