package google

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	// Packages
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	assert "github.com/stretchr/testify/assert"
)

///////////////////////////////////////////////////////////////////////////////
// HELPERS

// testdataPath returns the absolute path to a file in testdata/
func testdataPath(name string) string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "testdata", name)
}

// loadTestPair loads a paired JSON test fixture and returns the raw google and schema values
func loadTestPair(t *testing.T, name string) (json.RawMessage, json.RawMessage) {
	t.Helper()
	data, err := os.ReadFile(testdataPath(name))
	if err != nil {
		t.Fatalf("failed to read %s: %v", name, err)
	}
	var pair struct {
		Name   string          `json:"name"`
		Google json.RawMessage `json:"google"`
		Schema json.RawMessage `json:"schema"`
	}
	if err := json.Unmarshal(data, &pair); err != nil {
		t.Fatalf("failed to unmarshal %s: %v", name, err)
	}
	return pair.Google, pair.Schema
}

///////////////////////////////////////////////////////////////////////////////
// SCHEMA → GOOGLE (outbound) MESSAGE TESTS

func Test_marshal_schema_to_google_text_user(t *testing.T) {
	googleJSON, schemaJSON := loadTestPair(t, "message_text_user.json")
	assert := assert.New(t)

	// Decode the schema message
	msg := decodeSchemaMessage(t, schemaJSON)
	assert.Equal("user", msg.Role)

	// Convert to google wire format
	content, err := geminiContentFromMessage(msg)
	assert.NoError(err)

	// Compare with expected google JSON
	assertGoogleContentEquals(t, googleJSON, content)
}

func Test_marshal_schema_to_google_text_assistant(t *testing.T) {
	googleJSON, schemaJSON := loadTestPair(t, "message_text_assistant.json")
	assert := assert.New(t)

	msg := decodeSchemaMessage(t, schemaJSON)
	assert.Equal("assistant", msg.Role)

	content, err := geminiContentFromMessage(msg)
	assert.NoError(err)
	assertGoogleContentEquals(t, googleJSON, content)
}

func Test_marshal_schema_to_google_text_multipart(t *testing.T) {
	googleJSON, schemaJSON := loadTestPair(t, "message_text_multipart.json")
	assert := assert.New(t)

	msg := decodeSchemaMessage(t, schemaJSON)
	content, err := geminiContentFromMessage(msg)
	assert.NoError(err)
	assert.Len(content.Parts, 2)
	assertGoogleContentEquals(t, googleJSON, content)
}

func Test_marshal_schema_to_google_thinking(t *testing.T) {
	googleJSON, schemaJSON := loadTestPair(t, "message_thinking.json")
	assert := assert.New(t)

	msg := decodeSchemaMessage(t, schemaJSON)
	assert.NotNil(msg.Meta)
	assert.Equal(true, msg.Meta["thought"])

	content, err := geminiContentFromMessage(msg)
	assert.NoError(err)
	assert.True(content.Parts[0].Thought)
	assert.Equal("c2lnbmF0dXJlLWRhdGE=", content.Parts[0].ThoughtSignature)
	assert.False(content.Parts[1].Thought)
	assertGoogleContentEquals(t, googleJSON, content)
}

func Test_marshal_schema_to_google_inline_data(t *testing.T) {
	googleJSON, schemaJSON := loadTestPair(t, "message_inline_data.json")
	assert := assert.New(t)

	msg := decodeSchemaMessage(t, schemaJSON)
	assert.Len(msg.Content, 2)
	assert.NotNil(msg.Content[1].Attachment)

	content, err := geminiContentFromMessage(msg)
	assert.NoError(err)
	assert.Len(content.Parts, 2)
	assert.NotNil(content.Parts[1].InlineData)
	assertGoogleContentEquals(t, googleJSON, content)
}

func Test_marshal_schema_to_google_file_data(t *testing.T) {
	googleJSON, schemaJSON := loadTestPair(t, "message_file_data.json")
	assert := assert.New(t)

	msg := decodeSchemaMessage(t, schemaJSON)
	assert.NotNil(msg.Content[1].Attachment)
	assert.NotNil(msg.Content[1].Attachment.URL)

	content, err := geminiContentFromMessage(msg)
	assert.NoError(err)
	assert.NotNil(content.Parts[1].FileData)
	assertGoogleContentEquals(t, googleJSON, content)
}

func Test_marshal_schema_to_google_function_call(t *testing.T) {
	googleJSON, schemaJSON := loadTestPair(t, "message_function_call.json")
	assert := assert.New(t)

	msg := decodeSchemaMessage(t, schemaJSON)
	assert.NotNil(msg.Content[1].ToolCall)

	content, err := geminiContentFromMessage(msg)
	assert.NoError(err)
	assert.NotNil(content.Parts[1].FunctionCall)
	assert.Equal("get_weather", content.Parts[1].FunctionCall.Name)
	assertGoogleContentEquals(t, googleJSON, content)
}

func Test_marshal_schema_to_google_function_response(t *testing.T) {
	googleJSON, schemaJSON := loadTestPair(t, "message_function_response.json")
	assert := assert.New(t)

	msg := decodeSchemaMessage(t, schemaJSON)
	assert.NotNil(msg.Content[0].ToolResult)

	content, err := geminiContentFromMessage(msg)
	assert.NoError(err)
	assert.NotNil(content.Parts[0].FunctionResponse)
	assert.Equal("get_weather", content.Parts[0].FunctionResponse.Name)
	assertGoogleContentEquals(t, googleJSON, content)
}

func Test_marshal_schema_to_google_tool_error(t *testing.T) {
	googleJSON, schemaJSON := loadTestPair(t, "message_tool_error.json")
	assert := assert.New(t)

	msg := decodeSchemaMessage(t, schemaJSON)
	assert.NotNil(msg.Content[0].ToolResult)
	assert.True(msg.Content[0].ToolResult.IsError)

	content, err := geminiContentFromMessage(msg)
	assert.NoError(err)
	assert.NotNil(content.Parts[0].FunctionResponse)

	// Should have error flag in response
	resp := content.Parts[0].FunctionResponse.Response
	assert.Equal(true, resp["error"])
	assertGoogleContentEquals(t, googleJSON, content)
}

func Test_marshal_schema_to_google_multi_function_call(t *testing.T) {
	googleJSON, schemaJSON := loadTestPair(t, "message_multi_function_call.json")
	assert := assert.New(t)

	msg := decodeSchemaMessage(t, schemaJSON)
	assert.Len(msg.Content, 2)

	content, err := geminiContentFromMessage(msg)
	assert.NoError(err)
	assert.Len(content.Parts, 2)
	assert.NotNil(content.Parts[0].FunctionCall)
	assert.NotNil(content.Parts[1].FunctionCall)
	assertGoogleContentEquals(t, googleJSON, content)
}

///////////////////////////////////////////////////////////////////////////////
// GOOGLE → SCHEMA (inbound) RESPONSE TESTS

func Test_marshal_google_to_schema_response_text(t *testing.T) {
	googleJSON, schemaJSON := loadTestPair(t, "response_text.json")
	assert := assert.New(t)

	resp := decodeGeminiResponse(t, googleJSON)
	msg, err := messageFromGeminiResponse(resp)
	assert.NoError(err)
	assert.Equal("assistant", msg.Role)
	assert.Equal(schema.ResultStop, msg.Result)
	assertSchemaMessageEquals(t, schemaJSON, msg)
}

func Test_marshal_google_to_schema_response_function_call(t *testing.T) {
	googleJSON, schemaJSON := loadTestPair(t, "response_function_call.json")
	assert := assert.New(t)

	resp := decodeGeminiResponse(t, googleJSON)
	msg, err := messageFromGeminiResponse(resp)
	assert.NoError(err)
	assert.Equal("assistant", msg.Role)
	assert.Equal(schema.ResultToolCall, msg.Result)
	assert.Len(msg.Content, 2)
	assert.NotNil(msg.Content[0].Text)
	assert.NotNil(msg.Content[1].ToolCall)
	assertSchemaMessageEquals(t, schemaJSON, msg)
}

func Test_marshal_google_to_schema_response_thinking(t *testing.T) {
	googleJSON, schemaJSON := loadTestPair(t, "response_thinking.json")
	assert := assert.New(t)

	resp := decodeGeminiResponse(t, googleJSON)
	msg, err := messageFromGeminiResponse(resp)
	assert.NoError(err)
	assert.Equal("assistant", msg.Role)
	assert.Equal(schema.ResultStop, msg.Result)
	assert.NotNil(msg.Meta)
	assert.Equal(true, msg.Meta["thought"])
	assert.Equal("dGhvdWdodC1zaWc=", msg.Meta["thought_signature"])
	assertSchemaMessageEquals(t, schemaJSON, msg)
}

func Test_marshal_google_to_schema_response_safety(t *testing.T) {
	googleJSON, _ := loadTestPair(t, "response_safety.json")
	assert := assert.New(t)

	resp := decodeGeminiResponse(t, googleJSON)
	msg, err := messageFromGeminiResponse(resp)
	assert.NoError(err)
	assert.Equal(schema.ResultBlocked, msg.Result)
}

func Test_marshal_google_to_schema_response_max_tokens(t *testing.T) {
	googleJSON, _ := loadTestPair(t, "response_max_tokens.json")
	assert := assert.New(t)

	resp := decodeGeminiResponse(t, googleJSON)
	msg, err := messageFromGeminiResponse(resp)
	assert.NoError(err)
	assert.Equal(schema.ResultMaxTokens, msg.Result)
}

func Test_marshal_google_to_schema_response_empty(t *testing.T) {
	googleJSON, _ := loadTestPair(t, "response_empty.json")
	assert := assert.New(t)

	resp := decodeGeminiResponse(t, googleJSON)
	msg, err := messageFromGeminiResponse(resp)
	assert.NoError(err)
	assert.Empty(msg.Role)
	assert.Empty(msg.Content)
}

///////////////////////////////////////////////////////////////////////////////
// FINISH REASON TESTS

func Test_marshal_finish_reasons(t *testing.T) {
	tests := []struct {
		reason string
		result schema.ResultType
	}{
		{geminiFinishReasonStop, schema.ResultStop},
		{geminiFinishReasonMaxTokens, schema.ResultMaxTokens},
		{geminiFinishReasonSafety, schema.ResultBlocked},
		{geminiFinishReasonRecitation, schema.ResultBlocked},
		{geminiFinishReasonBlocklist, schema.ResultBlocked},
		{geminiFinishReasonProhibitedContent, schema.ResultBlocked},
		{geminiFinishReasonSPII, schema.ResultBlocked},
		{geminiFinishReasonImageSafety, schema.ResultBlocked},
		{geminiFinishReasonImageRecitation, schema.ResultBlocked},
		{geminiFinishReasonImageProhibitedContent, schema.ResultBlocked},
		{geminiFinishReasonLanguage, schema.ResultBlocked},
		{geminiFinishReasonMalformedFunctionCall, schema.ResultError},
		{geminiFinishReasonUnexpectedToolCall, schema.ResultError},
		{geminiFinishReasonOther, schema.ResultOther},
		{"UNKNOWN_REASON", schema.ResultOther},
		{"", schema.ResultOther},
	}
	for _, tt := range tests {
		t.Run(tt.reason, func(t *testing.T) {
			assert.Equal(t, tt.result, resultFromGeminiFinishReason(tt.reason))
		})
	}
}

///////////////////////////////////////////////////////////////////////////////
// ROUND-TRIP TESTS (schema → google → schema)

func Test_marshal_roundtrip_text(t *testing.T) {
	_, schemaJSON := loadTestPair(t, "message_text_user.json")
	msg := decodeSchemaMessage(t, schemaJSON)
	roundTripMessage(t, msg)
}

func Test_marshal_roundtrip_function_call(t *testing.T) {
	_, schemaJSON := loadTestPair(t, "message_function_call.json")
	msg := decodeSchemaMessage(t, schemaJSON)
	roundTripMessage(t, msg)
}

func Test_marshal_roundtrip_inline_data(t *testing.T) {
	_, schemaJSON := loadTestPair(t, "message_inline_data.json")
	msg := decodeSchemaMessage(t, schemaJSON)
	roundTripMessage(t, msg)
}

func Test_marshal_roundtrip_file_data(t *testing.T) {
	_, schemaJSON := loadTestPair(t, "message_file_data.json")
	msg := decodeSchemaMessage(t, schemaJSON)
	roundTripMessage(t, msg)
}

func Test_marshal_roundtrip_thinking(t *testing.T) {
	_, schemaJSON := loadTestPair(t, "message_thinking.json")
	msg := decodeSchemaMessage(t, schemaJSON)
	roundTripMessage(t, msg)
}

///////////////////////////////////////////////////////////////////////////////
// SESSION CONVERSION TESTS

func Test_marshal_session_skips_system(t *testing.T) {
	assert := assert.New(t)

	sys := "You are a helpful assistant."
	userText := "Hello"
	session := &schema.Conversation{
		{Role: schema.RoleSystem, Content: []schema.ContentBlock{{Text: &sys}}},
		{Role: schema.RoleUser, Content: []schema.ContentBlock{{Text: &userText}}},
	}

	contents, err := geminiContentsFromSession(session)
	assert.NoError(err)
	assert.Len(contents, 1)
	assert.Equal("user", contents[0].Role)
}

func Test_marshal_session_multi_turn(t *testing.T) {
	assert := assert.New(t)

	userText := "What is 2+2?"
	assistText := "4"
	followUp := "And 3+3?"
	session := &schema.Conversation{
		{Role: schema.RoleUser, Content: []schema.ContentBlock{{Text: &userText}}},
		{Role: schema.RoleAssistant, Content: []schema.ContentBlock{{Text: &assistText}}},
		{Role: schema.RoleUser, Content: []schema.ContentBlock{{Text: &followUp}}},
	}

	contents, err := geminiContentsFromSession(session)
	assert.NoError(err)
	assert.Len(contents, 3)
	assert.Equal("user", contents[0].Role)
	assert.Equal("model", contents[1].Role)
	assert.Equal("user", contents[2].Role)
}

func Test_marshal_session_nil(t *testing.T) {
	contents, err := geminiContentsFromSession(nil)
	assert.NoError(t, err)
	assert.Nil(t, contents)
}

///////////////////////////////////////////////////////////////////////////////
// DECODE HELPERS

// decodeSchemaMessage unmarshals a schema.Message from JSON, handling
// the attachment URL and tool call input fields that need special treatment
func decodeSchemaMessage(t *testing.T, data json.RawMessage) *schema.Message {
	t.Helper()

	// We decode into a flexible intermediate format because schema.Message
	// uses *url.URL for attachments and json.RawMessage for tool inputs,
	// which don't deserialize from arbitrary JSON without help.
	var raw struct {
		Role    string `json:"role"`
		Content []struct {
			Text       *string        `json:"text,omitempty"`
			Thinking   *string        `json:"thinking,omitempty"`
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
		if c.Thinking != nil {
			block.Thinking = c.Thinking
		}
		if c.Attachment != nil {
			att := &schema.Attachment{Type: c.Attachment.Type}
			if c.Attachment.Data != "" {
				// The data in the fixture is base64-encoded; decode to raw bytes
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

// decodeGeminiResponse unmarshals a geminiGenerateResponse from JSON
func decodeGeminiResponse(t *testing.T, data json.RawMessage) *geminiGenerateResponse {
	t.Helper()
	var resp geminiGenerateResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("failed to unmarshal gemini response: %v", err)
	}
	return &resp
}

///////////////////////////////////////////////////////////////////////////////
// ASSERTION HELPERS

// assertGoogleContentEquals marshals a geminiContent to JSON and compares
// it with the expected JSON, normalizing both sides
func assertGoogleContentEquals(t *testing.T, expectedJSON json.RawMessage, actual *geminiContent) {
	t.Helper()

	actualJSON, err := json.Marshal(actual)
	if err != nil {
		t.Fatalf("failed to marshal actual content: %v", err)
	}

	// Normalize both by unmarshalling into map[string]any and comparing
	var expected, got any
	json.Unmarshal(expectedJSON, &expected)
	json.Unmarshal(actualJSON, &got)
	assert.Equal(t, expected, got)
}

// assertSchemaMessageEquals compares a schema.Message against expected JSON,
// checking the fields that the test fixtures specify (role, content blocks, result, meta)
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

// roundTripMessage converts schema→google→schema and verifies the key fields survive
func roundTripMessage(t *testing.T, original *schema.Message) {
	t.Helper()
	assert := assert.New(t)

	// schema → google
	content, err := geminiContentFromMessage(original)
	assert.NoError(err)

	// google → schema (wrap in a fake response)
	resp := &geminiGenerateResponse{
		Candidates: []*geminiCandidate{
			{
				Content:      content,
				FinishReason: geminiFinishReasonStop,
			},
		},
	}
	roundTripped, err := messageFromGeminiResponse(resp)
	assert.NoError(err)

	// Verify role (accounting for assistant↔model mapping)
	expectedRole := original.Role
	if expectedRole == "assistant" {
		expectedRole = "assistant" // maps back correctly
	}
	assert.Equal(expectedRole, roundTripped.Role)

	// Verify same number of content blocks
	assert.Equal(len(original.Content), len(roundTripped.Content))

	// Verify each block type survived
	for i := range original.Content {
		orig := &original.Content[i]
		rt := &roundTripped.Content[i]

		if orig.Text != nil {
			assert.NotNil(rt.Text, "block %d: text should survive round-trip", i)
			assert.Equal(*orig.Text, *rt.Text)
		}
		if orig.Thinking != nil {
			assert.NotNil(rt.Thinking, "block %d: thinking should survive round-trip", i)
			assert.Equal(*orig.Thinking, *rt.Thinking)
		}
		if orig.Attachment != nil {
			assert.NotNil(rt.Attachment, "block %d: attachment should survive round-trip", i)
			assert.Equal(orig.Attachment.Type, rt.Attachment.Type)
		}
		if orig.ToolCall != nil {
			assert.NotNil(rt.ToolCall, "block %d: tool_call should survive round-trip", i)
			assert.Equal(orig.ToolCall.Name, rt.ToolCall.Name)
			assert.NotEmpty(rt.ToolCall.ID, "block %d: tool_call ID should be generated", i)
		}
		if orig.ToolResult != nil {
			assert.NotNil(rt.ToolResult, "block %d: tool_result should survive round-trip", i)
			assert.Equal(orig.ToolResult.Name, rt.ToolResult.Name)
		}
	}

	// Verify thinking metadata survives
	if original.Meta != nil {
		if thought, ok := original.Meta["thought"].(bool); ok && thought {
			assert.Equal(true, roundTripped.Meta["thought"])
		}
		if sig, ok := original.Meta["thought_signature"].(string); ok {
			assert.Equal(sig, roundTripped.Meta["thought_signature"])
		}
	}
}
