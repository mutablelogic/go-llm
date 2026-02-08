package mistral

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	schema "github.com/mutablelogic/go-llm/pkg/schema"
	assert "github.com/stretchr/testify/assert"
)

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
		Name    string          `json:"name"`
		Mistral json.RawMessage `json:"mistral"`
		Schema  json.RawMessage `json:"schema"`
	}
	if err := json.Unmarshal(data, &pair); err != nil {
		t.Fatalf("failed to unmarshal %s: %v", name, err)
	}
	return pair.Mistral, pair.Schema
}

func Test_marshal_schema_to_mistral_text_user(t *testing.T) {
	mistralJSON, schemaJSON := loadTestPair(t, "message_text_user.json")
	a := assert.New(t)
	msg := decodeSchemaMessage(t, schemaJSON)
	a.Equal("user", msg.Role)
	mms, err := mistralMessagesFromMessage(msg)
	a.NoError(err)
	a.Len(mms, 1)
	assertMistralMessageEquals(t, mistralJSON, &mms[0])
}

func Test_marshal_schema_to_mistral_text_assistant(t *testing.T) {
	mistralJSON, schemaJSON := loadTestPair(t, "message_text_assistant.json")
	a := assert.New(t)
	msg := decodeSchemaMessage(t, schemaJSON)
	a.Equal("assistant", msg.Role)
	mms, err := mistralMessagesFromMessage(msg)
	a.NoError(err)
	a.Len(mms, 1)
	assertMistralMessageEquals(t, mistralJSON, &mms[0])
}

func Test_marshal_schema_to_mistral_text_multipart(t *testing.T) {
	mistralJSON, schemaJSON := loadTestPair(t, "message_text_multipart.json")
	a := assert.New(t)
	msg := decodeSchemaMessage(t, schemaJSON)
	mms, err := mistralMessagesFromMessage(msg)
	a.NoError(err)
	a.Len(mms, 1)
	assertMistralMessageEquals(t, mistralJSON, &mms[0])
}

func Test_marshal_schema_to_mistral_image_url(t *testing.T) {
	mistralJSON, schemaJSON := loadTestPair(t, "message_image_url.json")
	a := assert.New(t)
	msg := decodeSchemaMessage(t, schemaJSON)
	a.Len(msg.Content, 2)
	a.NotNil(msg.Content[1].Attachment)
	a.NotNil(msg.Content[1].Attachment.URL)
	mms, err := mistralMessagesFromMessage(msg)
	a.NoError(err)
	a.Len(mms, 1)
	assertMistralMessageEquals(t, mistralJSON, &mms[0])
}

func Test_marshal_schema_to_mistral_tool_use(t *testing.T) {
	_, schemaJSON := loadTestPair(t, "message_tool_use.json")
	a := assert.New(t)
	msg := decodeSchemaMessage(t, schemaJSON)
	a.NotNil(msg.Content[1].ToolCall)
	mms, err := mistralMessagesFromMessage(msg)
	a.NoError(err)
	// Mistral keeps only tool_calls, text is dropped to avoid consecutive assistant messages
	a.Len(mms, 1)
	a.Nil(mms[0].Content)
	a.Len(mms[0].ToolCalls, 1)
	a.Equal("get_current_weather", mms[0].ToolCalls[0].Function.Name)
}

func Test_marshal_schema_to_mistral_multi_tool_use(t *testing.T) {
	mistralJSON, schemaJSON := loadTestPair(t, "message_multi_tool_use.json")
	a := assert.New(t)
	msg := decodeSchemaMessage(t, schemaJSON)
	a.Len(msg.Content, 2)
	mms, err := mistralMessagesFromMessage(msg)
	a.NoError(err)
	a.Len(mms, 1)
	a.Len(mms[0].ToolCalls, 2)
	assertMistralMessageEquals(t, mistralJSON, &mms[0])
}

func Test_marshal_schema_to_mistral_tool_result(t *testing.T) {
	_, schemaJSON := loadTestPair(t, "message_tool_result.json")
	a := assert.New(t)
	msg := decodeSchemaMessage(t, schemaJSON)
	a.NotNil(msg.Content[0].ToolResult)
	session := &schema.Session{msg}
	messages, err := mistralMessagesFromSession(session)
	a.NoError(err)
	a.Len(messages, 1)
	a.Equal(roleTool, messages[0].Role)
	a.Equal("cAbc12345", messages[0].ToolCallID)
}

func Test_marshal_schema_to_mistral_tool_error(t *testing.T) {
	_, schemaJSON := loadTestPair(t, "message_tool_error.json")
	a := assert.New(t)
	msg := decodeSchemaMessage(t, schemaJSON)
	a.NotNil(msg.Content[0].ToolResult)
	a.True(msg.Content[0].ToolResult.IsError)
	session := &schema.Session{msg}
	messages, err := mistralMessagesFromSession(session)
	a.NoError(err)
	a.Len(messages, 1)
	a.Equal(roleTool, messages[0].Role)
	a.Equal("cErr45678", messages[0].ToolCallID)
}

func Test_marshal_mistral_to_schema_response_text(t *testing.T) {
	mistralJSON, schemaJSON := loadTestPair(t, "response_text.json")
	a := assert.New(t)
	resp := decodeMistralResponse(t, mistralJSON)
	msg, err := messageFromMistralResponse(resp)
	a.NoError(err)
	a.Equal("assistant", msg.Role)
	a.Equal(schema.ResultStop, msg.Result)
	assertSchemaMessageEquals(t, schemaJSON, msg)
}

func Test_marshal_mistral_to_schema_response_tool_use(t *testing.T) {
	mistralJSON, schemaJSON := loadTestPair(t, "response_tool_use.json")
	a := assert.New(t)
	resp := decodeMistralResponse(t, mistralJSON)
	msg, err := messageFromMistralResponse(resp)
	a.NoError(err)
	a.Equal("assistant", msg.Role)
	a.Equal(schema.ResultToolCall, msg.Result)
	a.Len(msg.Content, 1)
	a.NotNil(msg.Content[0].ToolCall)
	assertSchemaMessageEquals(t, schemaJSON, msg)
}

func Test_marshal_mistral_to_schema_response_max_tokens(t *testing.T) {
	mistralJSON, _ := loadTestPair(t, "response_max_tokens.json")
	a := assert.New(t)
	resp := decodeMistralResponse(t, mistralJSON)
	msg, err := messageFromMistralResponse(resp)
	a.NoError(err)
	a.Equal(schema.ResultMaxTokens, msg.Result)
}

func Test_marshal_finish_reasons(t *testing.T) {
	tests := []struct {
		reason string
		result schema.ResultType
	}{
		{finishReasonStop, schema.ResultStop},
		{finishReasonLength, schema.ResultMaxTokens},
		{finishReasonModelLength, schema.ResultMaxTokens},
		{finishReasonToolCalls, schema.ResultToolCall},
		{finishReasonError, schema.ResultError},
		{"unknown_reason", schema.ResultOther},
		{"", schema.ResultOther},
	}
	for _, tt := range tests {
		t.Run(tt.reason, func(t *testing.T) {
			assert.Equal(t, tt.result, resultFromFinishReason(tt.reason))
		})
	}
}

func Test_marshal_roundtrip_text(t *testing.T) {
	_, schemaJSON := loadTestPair(t, "message_text_user.json")
	msg := decodeSchemaMessage(t, schemaJSON)
	roundTripMessage(t, msg)
}

func Test_marshal_roundtrip_tool_use(t *testing.T) {
	_, schemaJSON := loadTestPair(t, "message_tool_use.json")
	a := assert.New(t)
	msg := decodeSchemaMessage(t, schemaJSON)

	// This message has text + tool_call; Mistral keeps only tool_calls.
	mms, err := mistralMessagesFromMessage(msg)
	a.NoError(err)
	a.Len(mms, 1)

	// The single message carries the tool calls (text dropped)
	tcMsg := mms[0]
	a.Nil(tcMsg.Content)
	a.Len(tcMsg.ToolCalls, 1)
	a.Equal("get_current_weather", tcMsg.ToolCalls[0].Function.Name)

	// Roundtrip: parse the tool-call message back through the response parser
	resp := &chatCompletionResponse{
		Choices: []chatChoice{{
			Index:        0,
			Message:      tcMsg,
			FinishReason: finishReasonToolCalls,
		}},
	}
	result, err := messageFromMistralResponse(resp)
	a.NoError(err)
	a.Equal(schema.ResultToolCall, result.Result)
	a.Len(result.ToolCalls(), 1)
	a.Equal("get_current_weather", result.ToolCalls()[0].Name)
}

func Test_marshal_roundtrip_image_url(t *testing.T) {
	// Image URLs use the outbound multi-part content format which does not
	// round-trip through the inbound response parser (responses use plain
	// string content). Verify the outbound conversion works instead.
	mistralJSON, schemaJSON := loadTestPair(t, "message_image_url.json")
	a := assert.New(t)
	msg := decodeSchemaMessage(t, schemaJSON)
	mms, err := mistralMessagesFromMessage(msg)
	a.NoError(err)
	a.Len(mms, 1)
	assertMistralMessageEquals(t, mistralJSON, &mms[0])
}

func Test_marshal_session_multi_turn(t *testing.T) {
	a := assert.New(t)
	userText := "What is 2+2?"
	assistText := "4"
	followUp := "And 3+3?"
	session := &schema.Session{
		{Role: schema.RoleUser, Content: []schema.ContentBlock{{Text: &userText}}},
		{Role: schema.RoleAssistant, Content: []schema.ContentBlock{{Text: &assistText}}},
		{Role: schema.RoleUser, Content: []schema.ContentBlock{{Text: &followUp}}},
	}
	messages, err := mistralMessagesFromSession(session)
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
	session := &schema.Session{
		{Role: schema.RoleSystem, Content: []schema.ContentBlock{{Text: &sys}}},
		{Role: schema.RoleUser, Content: []schema.ContentBlock{{Text: &userText}}},
	}
	messages, err := mistralMessagesFromSession(session)
	a.NoError(err)
	a.Len(messages, 2)
	a.Equal("system", messages[0].Role)
	a.Equal("user", messages[1].Role)
}

func Test_marshal_session_nil(t *testing.T) {
	messages, err := mistralMessagesFromSession(nil)
	assert.NoError(t, err)
	assert.Nil(t, messages)
}

func Test_marshal_session_splits_tool_results(t *testing.T) {
	a := assert.New(t)
	result1 := json.RawMessage(`{"temp":22}`)
	result2 := json.RawMessage(`{"temp":18}`)
	session := &schema.Session{
		{
			Role: schema.RoleUser,
			Content: []schema.ContentBlock{
				{ToolResult: &schema.ToolResult{ID: "cCall1aaa", Content: result1}},
				{ToolResult: &schema.ToolResult{ID: "cCall2bbb", Content: result2}},
			},
		},
	}
	messages, err := mistralMessagesFromSession(session)
	a.NoError(err)
	a.Len(messages, 2)
	a.Equal(roleTool, messages[0].Role)
	a.Equal("cCall1aaa", messages[0].ToolCallID)
	a.Equal(roleTool, messages[1].Role)
	a.Equal("cCall2bbb", messages[1].ToolCallID)
}

// Test_marshal_session_remaps_invalid_tool_ids verifies that tool call IDs
// which don't conform to Mistral's 9-char alphanumeric requirement are
// replaced with valid generated IDs, and that the corresponding tool-result
// messages receive the same replacement IDs.
func Test_marshal_session_remaps_invalid_tool_ids(t *testing.T) {
	a := assert.New(t)
	session := &schema.Session{
		// Assistant message with two tool calls bearing invalid IDs
		{
			Role: schema.RoleAssistant,
			Content: []schema.ContentBlock{
				{Text: strPtr("checking")},
				{ToolCall: &schema.ToolCall{ID: "", Name: "get_state", Input: json.RawMessage(`{"id":"a"}`)}},
				{ToolCall: &schema.ToolCall{ID: "too_long_id_value", Name: "get_state", Input: json.RawMessage(`{"id":"b"}`)}},
			},
		},
		// Two tool-result messages with the same invalid IDs
		{
			Role: schema.RoleUser,
			Content: []schema.ContentBlock{
				{ToolResult: &schema.ToolResult{ID: "", Content: json.RawMessage(`"ok"`)}},
			},
		},
		{
			Role: schema.RoleUser,
			Content: []schema.ContentBlock{
				{ToolResult: &schema.ToolResult{ID: "too_long_id_value", Content: json.RawMessage(`"ok"`)}},
			},
		},
	}

	messages, err := mistralMessagesFromSession(session)
	a.NoError(err)
	// Tool calls take priority over text → 3 total (1 tool_calls + 2 tool results)
	a.Len(messages, 3)

	// messages[0] carries the tool calls with valid IDs (text dropped)
	a.Equal(roleAssistant, messages[0].Role)
	a.Nil(messages[0].Content)
	a.Len(messages[0].ToolCalls, 2)
	tc1ID := messages[0].ToolCalls[0].Id
	tc2ID := messages[0].ToolCalls[1].Id
	a.True(isValidMistralID(tc1ID), "tool call 1 ID should be valid: %q", tc1ID)
	a.True(isValidMistralID(tc2ID), "tool call 2 ID should be valid: %q", tc2ID)
	a.NotEqual(tc1ID, tc2ID, "two tool calls should get distinct IDs")

	// The tool-result messages should carry the same replacement IDs (in order)
	a.Equal(tc1ID, messages[1].ToolCallID)
	a.Equal(tc2ID, messages[2].ToolCallID)
}

// Test_marshal_session_preserves_valid_tool_ids ensures that tool call IDs
// already in valid Mistral format are passed through unchanged.
func Test_marshal_session_preserves_valid_tool_ids(t *testing.T) {
	a := assert.New(t)
	session := &schema.Session{
		{
			Role: schema.RoleAssistant,
			Content: []schema.ContentBlock{
				{ToolCall: &schema.ToolCall{ID: "Abcde1234", Name: "do_thing", Input: json.RawMessage(`{}`)}},
			},
		},
		{
			Role: schema.RoleUser,
			Content: []schema.ContentBlock{
				{ToolResult: &schema.ToolResult{ID: "Abcde1234", Content: json.RawMessage(`"ok"`)}},
			},
		},
	}

	messages, err := mistralMessagesFromSession(session)
	a.NoError(err)
	a.Len(messages, 2)
	a.Equal("Abcde1234", messages[0].ToolCalls[0].Id, "valid ID should be preserved")
	a.Equal("Abcde1234", messages[1].ToolCallID, "valid result ID should be preserved")
}

type rawAttachment struct {
	Type string `json:"type"`
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
			att := &schema.Attachment{Type: c.Attachment.Type}
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
			tc := &schema.ToolCall{
				ID:   c.ToolCall.ID,
				Name: c.ToolCall.Name,
			}
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

func decodeMistralResponse(t *testing.T, data json.RawMessage) *chatCompletionResponse {
	t.Helper()
	var resp chatCompletionResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("failed to unmarshal mistral response: %v", err)
	}
	return &resp
}

func assertMistralMessageEquals(t *testing.T, expectedJSON json.RawMessage, actual *mistralMessage) {
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

func roundTripMessage(t *testing.T, original *schema.Message) {
	t.Helper()
	a := assert.New(t)

	mms, err := mistralMessagesFromMessage(original)
	a.NoError(err)
	a.NotEmpty(mms)
	mm := mms[0]

	resp := &chatCompletionResponse{
		Choices: []chatChoice{
			{
				Index:        0,
				Message:      mm,
				FinishReason: finishReasonStop,
			},
		},
	}
	roundTripped, err := messageFromMistralResponse(resp)
	a.NoError(err)

	// Responses always come back as "assistant" role
	a.Equal(schema.RoleAssistant, roundTripped.Role)

	for _, orig := range original.Content {
		if orig.Text != nil {
			found := false
			for _, rt := range roundTripped.Content {
				if rt.Text != nil && *rt.Text == *orig.Text {
					found = true
					break
				}
			}
			a.True(found, "text %q should survive round-trip", *orig.Text)
		}
	}

	origToolCalls := 0
	rtToolCalls := 0
	for _, b := range original.Content {
		if b.ToolCall != nil {
			origToolCalls++
		}
	}
	for _, b := range roundTripped.Content {
		if b.ToolCall != nil {
			rtToolCalls++
		}
	}
	a.Equal(origToolCalls, rtToolCalls, "tool call count should survive round-trip")

	for _, orig := range original.Content {
		if orig.ToolCall != nil {
			found := false
			for _, rt := range roundTripped.Content {
				if rt.ToolCall != nil && rt.ToolCall.ID == orig.ToolCall.ID {
					a.Equal(orig.ToolCall.Name, rt.ToolCall.Name)
					found = true
					break
				}
			}
			a.True(found, "tool call %q should survive round-trip", orig.ToolCall.Name)
		}
	}
}
