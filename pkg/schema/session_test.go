package schema_test

import (
	"encoding/json"
	"testing"

	"github.com/mutablelogic/go-llm/pkg/schema"
	"github.com/stretchr/testify/assert"
)

func TestSessionAppend(t *testing.T) {
	assert := assert.New(t)

	var session schema.Session

	// Append messages
	session.Append(schema.StringMessage("user", "Hello"))
	session.Append(schema.StringMessage("assistant", "Hi there!"))

	assert.Len(session, 2)
	assert.Equal("user", session[0].Role)
	assert.Equal("assistant", session[1].Role)
}

func TestSessionTokens(t *testing.T) {
	assert := assert.New(t)

	var session schema.Session

	// Append messages with tokens set
	msg1 := schema.StringMessage("user", "Hello")
	msg1.Tokens = 10
	session.Append(msg1)

	msg2 := schema.StringMessage("assistant", "Hi there!")
	msg2.Tokens = 15
	session.Append(msg2)

	// Total tokens should be sum
	assert.Equal(uint(25), session.Tokens())
}

func TestSessionTokensEmpty(t *testing.T) {
	assert := assert.New(t)

	var session schema.Session
	assert.Equal(uint(0), session.Tokens())
}

func TestSessionAppendWithOutput(t *testing.T) {
	assert := assert.New(t)

	var session schema.Session

	// First message
	msg1 := schema.StringMessage("user", "Hello")
	session.Append(msg1)

	// Append with output - simulates LLM response with token counts
	msg2 := schema.StringMessage("assistant", "Hi there!")
	session.AppendWithOuput(msg2, 10, 15) // 10 input tokens, 15 output tokens

	assert.Len(session, 2)
	// The last message before append should have its tokens adjusted
	assert.Equal(uint(10), session[0].Tokens)
	// The new message should have output tokens
	assert.Equal(uint(15), session[1].Tokens)
	// Total should be input + output
	assert.Equal(uint(25), session.Tokens())
}

func TestSessionMarshalJSON(t *testing.T) {
	assert := assert.New(t)

	var session schema.Session
	session.Append(schema.StringMessage("user", "Hello"))
	session.Append(schema.StringMessage("assistant", "Hi there!"))

	data, err := json.Marshal(session)
	assert.NoError(err)

	// Should be an array of messages
	var result []map[string]any
	err = json.Unmarshal(data, &result)
	assert.NoError(err)
	assert.Len(result, 2)

	assert.Equal("user", result[0]["role"])
	assert.Equal("Hello", result[0]["content"])
	assert.Equal("assistant", result[1]["role"])
	assert.Equal("Hi there!", result[1]["content"])
}

func TestSessionUnmarshalJSON(t *testing.T) {
	assert := assert.New(t)

	jsonData := `[
		{"role": "system", "content": "You are helpful"},
		{"role": "user", "content": "Hello"},
		{"role": "assistant", "content": "Hi!"}
	]`

	var session schema.Session
	err := json.Unmarshal([]byte(jsonData), &session)
	assert.NoError(err)
	assert.Len(session, 3)

	assert.Equal("system", session[0].Role)
	assert.Equal("user", session[1].Role)
	assert.Equal("assistant", session[2].Role)
}

func TestSessionRoundTrip(t *testing.T) {
	assert := assert.New(t)

	var original schema.Session
	original.Append(schema.StringMessage("system", "You are a helpful assistant"))
	original.Append(schema.StringMessage("user", "What is 2+2?"))
	original.Append(schema.StringMessage("assistant", "2+2 equals 4"))

	// Marshal
	data, err := json.Marshal(original)
	assert.NoError(err)

	// Unmarshal
	var decoded schema.Session
	err = json.Unmarshal(data, &decoded)
	assert.NoError(err)

	assert.Len(decoded, 3)
	assert.Equal(original[0].Role, decoded[0].Role)
	assert.Equal(original[1].Role, decoded[1].Role)
	assert.Equal(original[2].Role, decoded[2].Role)
}

func TestSessionWithToolMessages(t *testing.T) {
	assert := assert.New(t)

	var session schema.Session
	session.Append(schema.StringMessage("user", "What's the weather?"))
	session.Append(schema.ToolResultMessage("tool_123", `{"temp": 72}`, false))

	data, err := json.Marshal(session)
	assert.NoError(err)

	var result []map[string]any
	err = json.Unmarshal(data, &result)
	assert.NoError(err)
	assert.Len(result, 2)

	// First message is simple user message
	assert.Equal("user", result[0]["role"])

	// Second message is tool result
	assert.Equal("user", result[1]["role"])
	content := result[1]["content"].([]any)
	toolResult := content[0].(map[string]any)
	assert.Equal("tool_result", toolResult["type"])
}

func TestSessionTokenAccumulation(t *testing.T) {
	assert := assert.New(t)

	var session schema.Session

	// Simulate a multi-turn conversation with token tracking
	msg1 := schema.StringMessage("user", "Hello")
	session.Append(msg1)

	// First response: 5 input tokens used, 10 output tokens
	resp1 := schema.StringMessage("assistant", "Hi!")
	session.AppendWithOuput(resp1, 5, 10)

	assert.Equal(uint(15), session.Tokens())

	// Second user message
	msg2 := schema.StringMessage("user", "How are you?")
	session.Append(msg2)

	// Second response: 20 total input tokens (including history), 8 output tokens
	resp2 := schema.StringMessage("assistant", "I'm doing well!")
	session.AppendWithOuput(resp2, 20, 8)

	// Tokens should accumulate correctly
	assert.Equal(uint(28), session.Tokens())
}

func TestSessionString(t *testing.T) {
	assert := assert.New(t)

	var session schema.Session
	session.Append(schema.StringMessage("user", "Test"))

	// String() should return valid JSON
	str := session.String()
	assert.NotEmpty(str)

	var result []map[string]any
	err := json.Unmarshal([]byte(str), &result)
	assert.NoError(err)
}
