package httphandler_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	eliza "github.com/mutablelogic/go-llm/provider/eliza"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	test "github.com/mutablelogic/go-llm/registry/test"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// askEliza returns a request to ask eliza the question
func askEliza(question string) schema.AskRequest {
	return schema.AskRequest{
		Model:    types.Ptr(eliza.ModelName),
		Messages: []providerschema.MessageMeta{{Sender: "user", Content: []providerschema.ContentBlock{{Text: types.Ptr(question)}}}},
	}
}

func Test_Ask(t *testing.T) {
	client, err := eliza.New(eliza.WithSeed(42))
	require.NoError(t, err)
	server := test.NewServer(t, test.NewRegistry(t, []llm.Provider{client}))

	t.Run("JSON", func(t *testing.T) {
		// The model's reply, with its canonical name, and the tokens used
		var response schema.AskResponse
		require.Equal(t, http.StatusOK, do(t, server, http.MethodPost, "llm/ask", askEliza("I feel sad"), &response))
		assert.Equal(t, "eliza:eliza-1966-en", response.Model)
		assert.Equal(t, "assistant", response.Message.Role())
		assert.NotEmpty(t, response.Message.Text())
		assert.NotZero(t, response.Usage.InputTokens)
	})

	t.Run("Stream", func(t *testing.T) {
		// With text/event-stream, the reply's text as it's generated, and
		// then the reply
		data, err := json.Marshal(askEliza("I feel sad"))
		require.NoError(t, err)
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL+test.Prefix+"/llm/ask", bytes.NewReader(data))
		require.NoError(t, err)
		req.Header.Set("Content-Type", types.ContentTypeJSON)
		req.Header.Set("Accept", types.ContentTypeTextStream)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var name, streamed string
		var response schema.AskResponse
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(nil, 1<<20)
		for scanner.Scan() {
			line := scanner.Text()
			if value, ok := strings.CutPrefix(line, "event: "); ok {
				name = value
			} else if value, ok := strings.CutPrefix(line, "data: "); ok {
				switch name {
				case schema.EventText:
					var event schema.StreamEvent
					require.NoError(t, json.Unmarshal([]byte(value), &event))
					streamed += event.Text
				case schema.EventResult:
					require.NoError(t, json.Unmarshal([]byte(value), &response))
				case schema.EventError:
					t.Fatalf("error event: %s", value)
				}
			}
		}
		assert.NotEmpty(t, streamed)
		assert.Equal(t, streamed, response.Message.Text())
		assert.Equal(t, "eliza:eliza-1966-en", response.Model)
	})

	t.Run("Errors", func(t *testing.T) {
		// A request needs messages, a model which exists, and tools which do,
		// of which the registry has none
		req := askEliza("Hi")
		req.Messages = nil
		assert.Equal(t, http.StatusBadRequest, do(t, server, http.MethodPost, "llm/ask", req, nil))
		req = askEliza("Hi")
		req.Model = types.Ptr("missing")
		assert.Equal(t, http.StatusNotFound, do(t, server, http.MethodPost, "llm/ask", req, nil))
		req = askEliza("Hi")
		req.Tools = []string{"weatherapi:current"}
		assert.Equal(t, http.StatusNotFound, do(t, server, http.MethodPost, "llm/ask", req, nil))

		// And the system prompt as a field, rather than an option
		req = askEliza("Hi")
		req.Opt = map[string]json.RawMessage{"system_prompt": json.RawMessage(`"x"`)}
		assert.Equal(t, http.StatusBadRequest, do(t, server, http.MethodPost, "llm/ask", req, nil))
	})
}
