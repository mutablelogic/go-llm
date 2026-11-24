package webhook

import (
	"bytes"
	"encoding/json"
	"regexp"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// hookToken is the token of a webhook's URL, which what was sent can have,
// such as when the webhook is registered. The slash before it can be escaped,
// as a URL's is, in another URL, more than once, or as JSON's is, in a string
// which is JSON.
var hookToken = regexp.MustCompile(`(?i)(hook(?:\\*/|\\+u002f|%(?:25)*2f))[a-z0-9_-]{43}`)

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// hideToken returns the text, without the tokens of webhooks' URLs
func hideToken(text string) string {
	return hookToken.ReplaceAllString(text, "${1}{token}")
}

// hideTokens returns the query, without the tokens of webhooks' URLs, in its
// names, or values
func hideTokens(query map[string][]string) map[string][]string {
	if query == nil {
		return nil
	}
	result := make(map[string][]string, len(query))
	for name, values := range query {
		name = hideToken(name)
		for _, value := range values {
			result[name] = append(result[name], hideToken(value))
		}
	}
	return result
}

// hideTokenValues returns the JSON, without the tokens of webhooks' URLs, in
// its strings, or its objects' names, however they're escaped
func hideTokenValues(data json.RawMessage) (json.RawMessage, error) {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, llm.ErrInternalServerError.Withf("what was sent isn't JSON: %v", err)
	}
	result, err := json.Marshal(hideTokenValue(value))
	if err != nil {
		return nil, llm.ErrInternalServerError.With(err.Error())
	}
	return result, nil
}

// hideTokenValue returns the JSON value, without the tokens of webhooks' URLs
func hideTokenValue(value any) any {
	switch value := value.(type) {
	case string:
		return hideToken(value)
	case []any:
		for i := range value {
			value[i] = hideTokenValue(value[i])
		}
		return value
	case map[string]any:
		result := make(map[string]any, len(value))
		for name, value := range value {
			result[hideToken(name)] = hideTokenValue(value)
		}
		return result
	}
	return value
}
