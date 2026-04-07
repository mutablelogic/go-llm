package ollama

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	// Packages
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	assert "github.com/stretchr/testify/assert"
)

///////////////////////////////////////////////////////////////////////////////
// HELPERS

func testdataPath(name string) string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "testdata", name)
}

func loadTestPair(t *testing.T, name string) (json.RawMessage, json.RawMessage) {
	t.Helper()
	data, err := os.ReadFile(testdataPath(name))
	if err != nil {
		t.Fatalf("failed to read %s: %v", name, err)
	}
	var pair struct {
		Name   string          `json:"name"`
		Ollama json.RawMessage `json:"ollama"`
		Schema json.RawMessage `json:"schema"`
	}
	if err := json.Unmarshal(data, &pair); err != nil {
		t.Fatalf("failed to unmarshal %s: %v", name, err)
	}
	return pair.Ollama, pair.Schema
}

type rawAttachment struct {
	Type string `json:"type"`
	Data string `json:"data,omitempty"`
	URL  string `json:"url,omitempty"`
}

type rawToolCall struct {
	ID    string `json:"id,omitempty"`
	Name  string `json:"name"`
	Input any    `json:"input,omitempty"`
}

type rawToolResult struct {
	ID      string `json:"id,omitempty"`
	Name    string `json:"name,omitempty"`
	Content any    `json:"content,omitempty"`
	IsError bool   `json:"is_error,omitempty"`
}

func decodeSchemaMessage(t *testing.T, data json.RawMessage) *schema.Message {
	t.Helper()
	var raw struct {
		Role    string `json:"role"`
		Content []struct {
			Text       *string        `json:"text,omitempty"`
			Attachment *rawAttachment `json:"attachment,omitempty"`
			ToolCall   *rawToolCall   `json:"tool_call,omitempty"`
			ToolResult *rawToolResult `json:"tool_result,omitempty"`
		} `json:"content"`
		Meta map[string]any `json:"meta,omitempty"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to unmarshal schema message: %v", err)
	}
	msg := &schema.Message{
		Role: raw.Role,
		Meta: raw.Meta,
	}
	for _, c := range raw.Content {
		var block schema.ContentBlock
		if c.Text != nil {
			block.Text = c.Text
		}
		if c.Attachment != nil {
			att := &schema.Attachment{ContentType: c.Attachment.Type}
			if c.Attachment.Data != "" {
				decoded, err := base64.StdEncoding.DecodeString(c.Attachment.Data)
				if err != nil {
					t.Fatalf("bad base64 in fixture: %v", err)
				}
				att.Data = decoded
			}
			if c.Attachment.URL != "" {
				u, err := url.Parse(c.Attachment.URL)
				if err != nil {
					t.Fatalf("bad url in fixture: %v", err)
				}
				att.URL = u
			}
			block.Attachment = att
		}
		if c.ToolCall != nil {
			tc := &schema.ToolCall{ID: c.ToolCall.ID, Name: c.ToolCall.Name}
			if c.ToolCall.Input != nil {
				tc.Input, _ = json.Marshal(c.ToolCall.Input)
			}
			block.ToolCall = tc
		}
		if c.ToolResult != nil {
			tr := &schema.ToolResult{
				ID:      c.ToolResult.ID,
				Name:    c.ToolResult.Name,
				IsError: c.ToolResult.IsError,
			}
			if c.ToolResult.Content != nil {
				tr.Content, _ = json.Marshal(c.ToolResult.Content)
			}
			block.ToolResult = tr
		}
		msg.Content = append(msg.Content, block)
	}
	return msg
}

func decodeOllamaResponse(t *testing.T, data json.RawMessage) *chatResponse {
	t.Helper()
	var resp chatResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("failed to unmarshal ollama response: %v", err)
	}
	return &resp
}

func assertOllamaMessageEquals(t *testing.T, expectedJSON json.RawMessage, actual *chatMessage) {
	t.Helper()
	actualJSON, err := json.Marshal(actual)
	if err != nil {
		t.Fatalf("failed to marshal actual message: %v", err)
	}
	var expected, got any
	json.Unmarshal(expectedJSON, &expected)
	json.Unmarshal(actualJSON, &got)
	assert.Equal(t, expected, got)
}

func assertSchemaMessageEquals(t *testing.T, expectedJSON json.RawMessage, actual *schema.Message) {
	t.Helper()
	var expected struct {
		Role    string         `json:"role"`
		Content []any          `json:"content"`
		Result  string         `json:"result,omitempty"`
		Meta    map[string]any `json:"meta,omitempty"`
	}
	if err := json.Unmarshal(expectedJSON, &expected); err != nil {
		t.Fatalf("failed to unmarshal expected schema: %v", err)
	}
	assert.Equal(t, expected.Role, actual.Role)
	if expected.Result != "" {
		assert.Equal(t, expected.Result, actual.Result.String())
	}
	if expected.Content != nil {
		assert.Equal(t, len(expected.Content), len(actual.Content))
	}
	if expected.Meta != nil {
		for k, v := range expected.Meta {
			assert.Equal(t, v, actual.Meta[k], "meta key %q", k)
		}
	}
}

///////////////////////////////////////////////////////////////////////////////
// SCHEMA -> OLLAMA MESSAGE TESTS

func Test_marshal_schema_to_ollama_text_user(t *testing.T) {
	ollamaJSON, schemaJSON := loadTestPair(t, "message_text_user.json")
	a := assert.New(t)
	msg := decodeSchemaMessage(t, schemaJSON)
	a.Equal("user", msg.Role)
	mms, err := ollamaChatMessagesFromMessage(msg)
	a.NoError(err)
	a.Len(mms, 1)
	assertOllamaMessageEquals(t, ollamaJSON, &mms[0])
}

func Test_marshal_schema_to_ollama_text_assistant(t *testing.T) {
	ollamaJSON, schemaJSON := loadTestPair(t, "message_text_assistant.json")
	a := assert.New(t)
	msg := decodeSchemaMessage(t, schemaJSON)
	a.Equal("assistant", msg.Role)
	mms, err := ollamaChatMessagesFromMessage(msg)
	a.NoError(err)
	a.Len(mms, 1)
	assertOllamaMessageEquals(t, ollamaJSON, &mms[0])
}

func Test_marshal_schema_to_ollama_text_multipart(t *testing.T) {
	ollamaJSON, schemaJSON := loadTestPair(t, "message_text_multipart.json")
	a := assert.New(t)
	msg := decodeSchemaMessage(t, schemaJSON)
	a.Len(msg.Content, 2)
	mms, err := ollamaChatMessagesFromMessage(msg)
	a.NoError(err)
	a.Len(mms, 1)
	a.Equal("First paragraph.\nSecond paragraph.", mms[0].Content)
	assertOllamaMessageEquals(t, ollamaJSON, &mms[0])
}

func Test_marshal_schema_to_ollama_tool_use(t *testing.T) {
	ollamaJSON, schemaJSON := loadTestPair(t, "message_tool_use.json")
	a := assert.New(t)
	msg := decodeSchemaMessage(t, schemaJSON)
	a.NotNil(msg.Content[1].ToolCall)
	mms, err := ollamaChatMessagesFromMessage(msg)
	a.NoError(err)
	a.Len(mms, 1)
	a.Equal("Let me check the weather for you.", mms[0].Content)
	a.Len(mms[0].ToolCalls, 1)
	a.Equal("get_current_weather", mms[0].ToolCalls[0].Function.Name)
	assertOllamaMessageEquals(t, ollamaJSON, &mms[0])
}

func Test_marshal_schema_to_ollama_multi_tool_use(t *testing.T) {
	ollamaJSON, schemaJSON := loadTestPair(t, "message_multi_tool_use.json")
	a := assert.New(t)
	msg := decodeSchemaMessage(t, schemaJSON)
	a.Len(msg.Content, 2)
	mms, err := ollamaChatMessagesFromMessage(msg)
	a.NoError(err)
	a.Len(mms, 1)
	a.Len(mms[0].ToolCalls, 2)
	assertOllamaMessageEquals(t, ollamaJSON, &mms[0])
}

func Test_marshal_schema_to_ollama_tool_result(t *testing.T) {
	_, schemaJSON := loadTestPair(t, "message_tool_result.json")
	a := assert.New(t)
	msg := decodeSchemaMessage(t, schemaJSON)
	a.NotNil(msg.Content[0].ToolResult)
	session := &schema.Conversation{msg}
	messages, err := ollamaMessagesFromSession(session)
	a.NoError(err)
	a.Len(messages, 1)
	a.Equal(schema.RoleTool, messages[0].Role)
	a.Equal("call_abc123", messages[0].ToolCallID)
	a.Equal("get_current_weather", messages[0].ToolName)
}

func Test_marshal_schema_to_ollama_tool_error(t *testing.T) {
	_, schemaJSON := loadTestPair(t, "message_tool_error.json")
	a := assert.New(t)
	msg := decodeSchemaMessage(t, schemaJSON)
	a.NotNil(msg.Content[0].ToolResult)
	a.True(msg.Content[0].ToolResult.IsError)
	session := &schema.Conversation{msg}
	messages, err := ollamaMessagesFromSession(session)
	a.NoError(err)
	a.Len(messages, 1)
	a.Equal(schema.RoleTool, messages[0].Role)
	a.Equal("call_err456", messages[0].ToolCallID)
	a.Equal("location not found", messages[0].Content)
}

func Test_marshal_schema_to_ollama_image_base64(t *testing.T) {
	ollamaJSON, schemaJSON := loadTestPair(t, "message_image_base64.json")
	a := assert.New(t)
	msg := decodeSchemaMessage(t, schemaJSON)
	a.Len(msg.Content, 2)
	a.NotNil(msg.Content[1].Attachment)
	a.Equal("image/png", msg.Content[1].Attachment.ContentType)
	a.NotEmpty(msg.Content[1].Attachment.Data)
	mms, err := ollamaChatMessagesFromMessage(msg)
	a.NoError(err)
	a.Len(mms, 1)
	a.Len(mms[0].Images, 1)
	assertOllamaMessageEquals(t, ollamaJSON, &mms[0])
}

///////////////////////////////////////////////////////////////////////////////
// OLLAMA RESPONSE -> SCHEMA MESSAGE TESTS

func Test_marshal_ollama_to_schema_response_text(t *testing.T) {
	ollamaJSON, schemaJSON := loadTestPair(t, "response_text.json")
	a := assert.New(t)
	resp := decodeOllamaResponse(t, ollamaJSON)
	msg, err := messageFromOllamaResponse(resp)
	a.NoError(err)
	a.Equal("assistant", msg.Role)
	a.Equal(schema.ResultStop, msg.Result)
	assertSchemaMessageEquals(t, schemaJSON, msg)
}

func Test_marshal_ollama_to_schema_response_tool_use(t *testing.T) {
	ollamaJSON, schemaJSON := loadTestPair(t, "response_tool_use.json")
	a := assert.New(t)
	resp := decodeOllamaResponse(t, ollamaJSON)
	msg, err := messageFromOllamaResponse(resp)
	a.NoError(err)
	a.Equal("assistant", msg.Role)
	a.Equal(schema.ResultToolCall, msg.Result)
	a.Len(msg.Content, 1)
	a.NotNil(msg.Content[0].ToolCall)
	a.Equal("get_current_weather", msg.Content[0].ToolCall.Name)
	assertSchemaMessageEquals(t, schemaJSON, msg)
}

func Test_marshal_ollama_to_schema_response_max_tokens(t *testing.T) {
	ollamaJSON, _ := loadTestPair(t, "response_max_tokens.json")
	a := assert.New(t)
	resp := decodeOllamaResponse(t, ollamaJSON)
	msg, err := messageFromOllamaResponse(resp)
	a.NoError(err)
	a.Equal(schema.ResultMaxTokens, msg.Result)
}

func Test_marshal_ollama_to_schema_response_thinking(t *testing.T) {
	ollamaJSON, schemaJSON := loadTestPair(t, "response_thinking.json")
	a := assert.New(t)
	resp := decodeOllamaResponse(t, ollamaJSON)
	msg, err := messageFromOllamaResponse(resp)
	a.NoError(err)
	a.Equal("assistant", msg.Role)
	a.Equal(schema.ResultStop, msg.Result)
	// First block is thinking, second is text
	a.Len(msg.Content, 2)
	a.NotNil(msg.Content[0].Thinking)
	a.Equal("Let me work through this step by step.", *msg.Content[0].Thinking)
	a.NotNil(msg.Content[1].Text)
	a.Equal("The answer is 42.", *msg.Content[1].Text)
	assertSchemaMessageEquals(t, schemaJSON, msg)
}

func Test_marshal_ollama_to_schema_response_image(t *testing.T) {
	ollamaJSON, schemaJSON := loadTestPair(t, "response_image.json")
	a := assert.New(t)
	resp := decodeOllamaResponse(t, ollamaJSON)
	msg, err := messageFromOllamaResponse(resp)
	a.NoError(err)
	a.Equal("assistant", msg.Role)
	a.Equal(schema.ResultStop, msg.Result)
	// Single attachment block with detected content type
	a.Len(msg.Content, 1)
	a.NotNil(msg.Content[0].Attachment)
	a.Equal("image/png", msg.Content[0].Attachment.ContentType)
	a.NotEmpty(msg.Content[0].Attachment.Data)
	assertSchemaMessageEquals(t, schemaJSON, msg)
}

///////////////////////////////////////////////////////////////////////////////
// DONE REASON MAPPING

func Test_marshal_done_reasons(t *testing.T) {
	tests := []struct {
		reason string
		result schema.ResultType
	}{
		{"stop", schema.ResultStop},
		{"length", schema.ResultMaxTokens},
		{"tool_calls", schema.ResultToolCall},
		{"unknown_reason", schema.ResultOther},
		{"", schema.ResultOther},
	}
	for _, tt := range tests {
		t.Run(tt.reason, func(t *testing.T) {
			assert.Equal(t, tt.result, resultFromDoneReason(tt.reason))
		})
	}
}

///////////////////////////////////////////////////////////////////////////////
// SESSION-LEVEL TESTS

func Test_marshal_session_nil(t *testing.T) {
	messages, err := ollamaMessagesFromSession(nil)
	assert.NoError(t, err)
	assert.Nil(t, messages)
}

func Test_marshal_session_multi_turn(t *testing.T) {
	a := assert.New(t)
	userText := "What is 2+2?"
	assistText := "4"
	followUp := "And 3+3?"
	session := &schema.Conversation{
		{Role: schema.RoleUser, Content: []schema.ContentBlock{{Text: &userText}}},
		{Role: schema.RoleAssistant, Content: []schema.ContentBlock{{Text: &assistText}}},
		{Role: schema.RoleUser, Content: []schema.ContentBlock{{Text: &followUp}}},
	}
	messages, err := ollamaMessagesFromSession(session)
	a.NoError(err)
	a.Len(messages, 3)
	a.Equal("user", messages[0].Role)
	a.Equal("assistant", messages[1].Role)
	a.Equal("user", messages[2].Role)
}

func Test_marshal_session_with_system(t *testing.T) {
	a := assert.New(t)
	sys := "You are a helpful assistant."
	userText := "Hello"
	session := &schema.Conversation{
		{Role: schema.RoleSystem, Content: []schema.ContentBlock{{Text: &sys}}},
		{Role: schema.RoleUser, Content: []schema.ContentBlock{{Text: &userText}}},
	}
	messages, err := ollamaMessagesFromSession(session)
	a.NoError(err)
	a.Len(messages, 2)
	a.Equal("system", messages[0].Role)
	a.Equal("user", messages[1].Role)
}

func Test_marshal_session_splits_tool_results(t *testing.T) {
	a := assert.New(t)
	result1 := json.RawMessage(`{"temp":22}`)
	result2 := json.RawMessage(`{"temp":18}`)
	session := &schema.Conversation{
		{
			Role: schema.RoleUser,
			Content: []schema.ContentBlock{
				{ToolResult: &schema.ToolResult{ID: "call_aaa", Name: "fn1", Content: result1}},
				{ToolResult: &schema.ToolResult{ID: "call_bbb", Name: "fn2", Content: result2}},
			},
		},
	}
	messages, err := ollamaMessagesFromSession(session)
	a.NoError(err)
	a.Len(messages, 2)
	a.Equal(schema.RoleTool, messages[0].Role)
	a.Equal("call_aaa", messages[0].ToolCallID)
	a.Equal(schema.RoleTool, messages[1].Role)
	a.Equal("call_bbb", messages[1].ToolCallID)
}

func Test_marshal_tool_result_json_string_content(t *testing.T) {
	a := assert.New(t)
	session := &schema.Conversation{
		{
			Role: schema.RoleUser,
			Content: []schema.ContentBlock{
				{ToolResult: &schema.ToolResult{
					ID:      "call_str",
					Name:    "fn",
					Content: json.RawMessage(`"location not found"`),
				}},
			},
		},
	}
	messages, err := ollamaMessagesFromSession(session)
	a.NoError(err)
	a.Len(messages, 1)
	a.Equal("location not found", messages[0].Content)
}

func Test_marshal_tool_result_json_object_content(t *testing.T) {
	a := assert.New(t)
	session := &schema.Conversation{
		{
			Role: schema.RoleUser,
			Content: []schema.ContentBlock{
				{ToolResult: &schema.ToolResult{
					ID:      "call_obj",
					Name:    "fn",
					Content: json.RawMessage(`{"temperature":22}`),
				}},
			},
		},
	}
	messages, err := ollamaMessagesFromSession(session)
	a.NoError(err)
	a.Len(messages, 1)
	a.Equal(`{"temperature":22}`, messages[0].Content)
}
