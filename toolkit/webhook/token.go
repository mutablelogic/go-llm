package webhook

import (
	"bytes"
	"encoding/json"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/webhook/schema"
)

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// hideTokens returns the query, without the tokens of webhooks' URLs, in its
// names, or values
func hideTokens(query map[string][]string) map[string][]string {
	if query == nil {
		return nil
	}
	result := make(map[string][]string, len(query))
	for name, values := range query {
		name = schema.HideToken(name)
		for _, value := range values {
			result[name] = append(result[name], schema.HideToken(value))
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
		return schema.HideToken(value)
	case []any:
		for i := range value {
			value[i] = hideTokenValue(value[i])
		}
		return value
	case map[string]any:
		result := make(map[string]any, len(value))
		for name, value := range value {
			result[schema.HideToken(name)] = hideTokenValue(value)
		}
		return result
	}
	return value
}
