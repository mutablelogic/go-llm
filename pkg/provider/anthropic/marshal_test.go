package anthropic

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

// loadTestPair loads a paired JSON test fixture and returns the raw anthropic and schema values
func loadTestPair(t *testing.T, name string) (json.RawMessage, json.RawMessage) {
	t.Helper()
	data, err := os.ReadFile(testdataPath(name))
	if err != nil {
		t.Fatalf("failed to read %s: %v", name, err)
	}
	var pair struct {
		Name      string          `json:"name"`
		Anthropic json.RawMessage `json:"anthropic"`
		Schema    json.RawMessage `json:"schema"`
	}
	if err := json.Unmarshal(data, &pair); err != nil {
		t.Fatalf("failed to unmarshal %s: %v", name, err)
	}
	return pair.Anthropic, pair.Schema
}

///////////////////////////////////////////////////////////////////////////////
// SCHEMA -> ANTHROPIC (outbound) MESSAGE TESTS

func Test_marshal_schema_to_anthropic_text_user(t *testing.T) {
	anthropicJSON, schemaJSON := loadTestPair(t, "message_text_user.json")
	assert := assert.New(t)

	msg := decodeSchemaMessage(t, schemaJSON)
	assert.Equal("user", msg.Role)

	am, err := anthropicMessageFromMessage(msg)
	assert.NoError(err)
	assertAnthropicMessageEquals(t, anthropicJSON, &am)
}

func Test_marshal_schema_to_anthropic_text_assistant(t *testing.T) {
	anthropicJSON, schemaJSON := loadTestPair(t, "message_text_assistant.json")
	assert := assert.New(t)

	msg := decodeSchemaMessage(t, schemaJSON)
	assert.Equal("assistant", msg.Role)

	am, err := anthropicMessageFromMessage(msg)
	assert.NoError(err)
	assertAnthropicMessageEquals(t, anthropicJSON, &am)
}

func Test_marshal_schema_to_anthropic_text_multipart(t *testing.T) {
	anthropicJSON, schemaJSON := loadTestPair(t, "message_text_multipart.json")
	assert := assert.New(t)

	msg := decodeSchemaMessage(t, schemaJSON)
	am, err := anthropicMessageFromMessage(msg)
	assert.NoError(err)
	assert.Len(am.Content, 2)
	assertAnthropicMessageEquals(t, anthropicJSON, &am)
}

func Test_marshal_schema_to_anthropic_thinking(t *testing.T) {
	anthropicJSON, schemaJSON := loadTestPair(t, "message_thinking.json")
	assert := assert.New(t)

	msg := decodeSchemaMessage(t, schemaJSON)
	assert.NotNil(msg.Meta)
	assert.Equal(true, msg.Meta["thought"])

	am, err := anthropicMessageFromMessage(msg)
	assert.NoError(err)
	assert.Equal(blockTypeThinking, am.Content[0].Type)
	assert.Equal("c2lnbmF0dXJlLWRhdGE=", am.Content[0].Signature)
	assert.Equal(blockTypeText, am.Content[1].Type)
	assertAnthropicMessageEquals(t, anthropicJSON, &am)
}

func Test_marshal_schema_to_anthropic_image_base64(t *testing.T) {
	anthropicJSON, schemaJSON := loadTestPair(t, "message_image_base64.json")
	assert := assert.New(t)

	msg := decodeSchemaMessage(t, schemaJSON)
	assert.Len(msg.Content, 2)
	assert.NotNil(msg.Content[1].Attachment)

	am, err := anthropicMessageFromMessage(msg)
	assert.NoError(err)
	assert.Len(am.Content, 2)
	assert.Equal(blockTypeImage, am.Content[1].Type)
	assert.NotNil(am.Content[1].Source)
	assert.Equal(sourceTypeBase64, am.Content[1].Source.Type)
	assertAnthropicMessageEquals(t, anthropicJSON, &am)
}

func Test_marshal_schema_to_anthropic_image_url(t *testing.T) {
	anthropicJSON, schemaJSON := loadTestPair(t, "message_image_url.json")
	assert := assert.New(t)

	msg := decodeSchemaMessage(t, schemaJSON)
	assert.NotNil(msg.Content[1].Attachment)
	assert.NotNil(msg.Content[1].Attachment.URL)

	am, err := anthropicMessageFromMessage(msg)
	assert.NoError(err)
	assert.NotNil(am.Content[1].Source)
	assert.Equal(sourceTypeURL, am.Content[1].Source.Type)
	assertAnthropicMessageEquals(t, anthropicJSON, &am)
}

func Test_marshal_schema_to_anthropic_document(t *testing.T) {
	anthropicJSON, schemaJSON := loadTestPair(t, "message_document.json")
	assert := assert.New(t)

	msg := decodeSchemaMessage(t, schemaJSON)
	assert.NotNil(msg.Content[1].Attachment)

	am, err := anthropicMessageFromMessage(msg)
	assert.NoError(err)
	assert.Equal(blockTypeDocument, am.Content[1].Type)
	assert.NotNil(am.Content[1].Source)
	assert.Equal("application/pdf", am.Content[1].Source.MediaType)
	assertAnthropicMessageEquals(t, anthropicJSON, &am)
}

func Test_marshal_schema_to_anthropic_tool_use(t *testing.T) {
	anthropicJSON, schemaJSON := loadTestPair(t, "message_tool_use.json")
	assert := assert.New(t)

	msg := decodeSchemaMessage(t, schemaJSON)
	assert.NotNil(msg.Content[1].ToolCall)

	am, err := anthropicMessageFromMessage(msg)
	assert.NoError(err)
	assert.Equal(blockTypeToolUse, am.Content[1].Type)
	assert.Equal("get_weather", am.Content[1].Name)
	assertAnthropicMessageEquals(t, anthropicJSON, &am)
}

func Test_marshal_schema_to_anthropic_tool_result(t *testing.T) {
	anthropicJSON, schemaJSON := loadTestPair(t, "message_tool_result.json")
	assert := assert.New(t)

	msg := decodeSchemaMessage(t, schemaJSON)
	assert.NotNil(msg.Content[0].ToolResult)

	am, err := anthropicMessageFromMessage(msg)
	assert.NoError(err)
	assert.Equal(blockTypeToolResult, am.Content[0].Type)
	assert.Equal("toolu_01A", am.Content[0].ToolUseID)
	assertAnthropicMessageEquals(t, anthropicJSON, &am)
}

func Test_marshal_schema_to_anthropic_tool_error(t *testing.T) {
	anthropicJSON, schemaJSON := loadTestPair(t, "message_tool_error.json")
	assert := assert.New(t)

	msg := decodeSchemaMessage(t, schemaJSON)
	assert.NotNil(msg.Content[0].ToolResult)
	assert.True(msg.Content[0].ToolResult.IsError)

	am, err := anthropicMessageFromMessage(msg)
	assert.NoError(err)
	assert.Equal(blockTypeToolResult, am.Content[0].Type)
	assert.True(am.Content[0].IsError)
	assertAnthropicMessageEquals(t, anthropicJSON, &am)
}

func Test_marshal_schema_to_anthropic_multi_tool_use(t *testing.T) {
	anthropicJSON, schemaJSON := loadTestPair(t, "message_multi_tool_use.json")
	assert := assert.New(t)

	msg := decodeSchemaMessage(t, schemaJSON)
	assert.Len(msg.Content, 2)

	am, err := anthropicMessageFromMessage(msg)
	assert.NoError(err)
	assert.Len(am.Content, 2)
	assert.Equal(blockTypeToolUse, am.Content[0].Type)
	assert.Equal(blockTypeToolUse, am.Content[1].Type)
	assertAnthropicMessageEquals(t, anthropicJSON, &am)
}

///////////////////////////////////////////////////////////////////////////////
// ANTHROPIC -> SCHEMA (inbound) RESPONSE TESTS

func Test_marshal_anthropic_to_schema_response_text(t *testing.T) {
	anthropicJSON, schemaJSON := loadTestPair(t, "response_text.json")
	assert := assert.New(t)

	resp := decodeAnthropicResponse(t, anthropicJSON)
	msg, err := messageFromAnthropicResponse(resp.Role, resp.Content, resp.StopReason)
	assert.NoError(err)
	assert.Equal("assistant", msg.Role)
	assert.Equal(schema.ResultStop, msg.Result)
	assertSchemaMessageEquals(t, schemaJSON, msg)
}

func Test_marshal_anthropic_to_schema_response_tool_use(t *testing.T) {
	anthropicJSON, schemaJSON := loadTestPair(t, "response_tool_use.json")
	assert := assert.New(t)

	resp := decodeAnthropicResponse(t, anthropicJSON)
	msg, err := messageFromAnthropicResponse(resp.Role, resp.Content, resp.StopReason)
	assert.NoError(err)
	assert.Equal("assistant", msg.Role)
	assert.Equal(schema.ResultToolCall, msg.Result)
	assert.Len(msg.Content, 2)
	assert.NotNil(msg.Content[0].Text)
	assert.NotNil(msg.Content[1].ToolCall)
	assertSchemaMessageEquals(t, schemaJSON, msg)
}

func Test_marshal_anthropic_to_schema_response_thinking(t *testing.T) {
	anthropicJSON, schemaJSON := loadTestPair(t, "response_thinking.json")
	assert := assert.New(t)

	resp := decodeAnthropicResponse(t, anthropicJSON)
	msg, err := messageFromAnthropicResponse(resp.Role, resp.Content, resp.StopReason)
	assert.NoError(err)
	assert.Equal("assistant", msg.Role)
	assert.Equal(schema.ResultStop, msg.Result)
	assert.NotNil(msg.Meta)
	assert.Equal(true, msg.Meta["thought"])
	assert.Equal("dGhvdWdodC1zaWc=", msg.Meta["thought_signature"])
	assertSchemaMessageEquals(t, schemaJSON, msg)
}

func Test_marshal_anthropic_to_schema_response_max_tokens(t *testing.T) {
	anthropicJSON, _ := loadTestPair(t, "response_max_tokens.json")
	assert := assert.New(t)

	resp := decodeAnthropicResponse(t, anthropicJSON)
	msg, err := messageFromAnthropicResponse(resp.Role, resp.Content, resp.StopReason)
	assert.NoError(err)
	assert.Equal(schema.ResultMaxTokens, msg.Result)
}

func Test_marshal_anthropic_to_schema_response_refusal(t *testing.T) {
	anthropicJSON, _ := loadTestPair(t, "response_refusal.json")
	assert := assert.New(t)

	resp := decodeAnthropicResponse(t, anthropicJSON)
	msg, err := messageFromAnthropicResponse(resp.Role, resp.Content, resp.StopReason)
	assert.NoError(err)
	assert.Equal(schema.ResultBlocked, msg.Result)
}

///////////////////////////////////////////////////////////////////////////////
// STOP REASON TESTS

func Test_marshal_stop_reasons(t *testing.T) {
	tests := []struct {
		reason string
		result schema.ResultType
	}{
		{stopReasonEndTurn, schema.ResultStop},
		{stopReasonStopSequence, schema.ResultStop},
		{stopReasonMaxTokens, schema.ResultMaxTokens},
		{stopReasonToolUse, schema.ResultToolCall},
		{stopReasonRefusal, schema.ResultBlocked},
		{stopReasonPauseTurn, schema.ResultOther},
		{"unknown_reason", schema.ResultOther},
		{"", schema.ResultOther},
	}
	for _, tt := range tests {
		t.Run(tt.reason, func(t *testing.T) {
			assert.Equal(t, tt.result, resultFromStopReason(tt.reason))
		})
	}
}

///////////////////////////////////////////////////////////////////////////////
// ROUND-TRIP TESTS (schema -> anthropic -> schema)

func Test_marshal_roundtrip_text(t *testing.T) {
	_, schemaJSON := loadTestPair(t, "message_text_user.json")
	msg := decodeSchemaMessage(t, schemaJSON)
	roundTripMessage(t, msg)
}

func Test_marshal_roundtrip_tool_use(t *testing.T) {
	_, schemaJSON := loadTestPair(t, "message_tool_use.json")
	msg := decodeSchemaMessage(t, schemaJSON)
	roundTripMessage(t, msg)
}

func Test_marshal_roundtrip_image_base64(t *testing.T) {
	_, schemaJSON := loadTestPair(t, "message_image_base64.json")
	msg := decodeSchemaMessage(t, schemaJSON)
	roundTripMessage(t, msg)
}

func Test_marshal_roundtrip_image_url(t *testing.T) {
	_, schemaJSON := loadTestPair(t, "message_image_url.json")
	msg := decodeSchemaMessage(t, schemaJSON)
	roundTripMessage(t, msg)
}

func Test_marshal_roundtrip_thinking(t *testing.T) {
	_, schemaJSON := loadTestPair(t, "message_thinking.json")
	msg := decodeSchemaMessage(t, schemaJSON)
	roundTripMessage(t, msg)
}

func Test_marshal_roundtrip_document(t *testing.T) {
	_, schemaJSON := loadTestPair(t, "message_document.json")
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

	messages, err := anthropicMessagesFromSession(session)
	assert.NoError(err)
	assert.Len(messages, 1)
	assert.Equal("user", messages[0].Role)
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

	messages, err := anthropicMessagesFromSession(session)
	assert.NoError(err)
	assert.Len(messages, 3)
	assert.Equal("user", messages[0].Role)
	assert.Equal("assistant", messages[1].Role)
	assert.Equal("user", messages[2].Role)
}

func Test_marshal_session_nil(t *testing.T) {
	messages, err := anthropicMessagesFromSession(nil)
	assert.NoError(t, err)
	assert.Nil(t, messages)
}

///////////////////////////////////////////////////////////////////////////////
// DECODE HELPERS

// decodeSchemaMessage unmarshals a schema.Message from JSON, handling
// the attachment URL and tool call input fields that need special treatment
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

// decodeAnthropicResponse unmarshals a messagesResponse from JSON
func decodeAnthropicResponse(t *testing.T, data json.RawMessage) *messagesResponse {
	t.Helper()
	var resp messagesResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("failed to unmarshal anthropic response: %v", err)
	}
	return &resp
}

///////////////////////////////////////////////////////////////////////////////
// ASSERTION HELPERS

// assertAnthropicMessageEquals marshals an anthropicMessage to JSON and compares
// it with the expected JSON, normalizing both sides
func assertAnthropicMessageEquals(t *testing.T, expectedJSON json.RawMessage, actual *anthropicMessage) {
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

// roundTripMessage converts schema -> anthropic -> schema and verifies the key fields survive
func roundTripMessage(t *testing.T, original *schema.Message) {
	t.Helper()
	assert := assert.New(t)

	// schema -> anthropic
	am, err := anthropicMessageFromMessage(original)
	assert.NoError(err)

	// anthropic -> schema (wrap as a response with end_turn stop reason)
	roundTripped, err := messageFromAnthropicResponse(am.Role, am.Content, stopReasonEndTurn)
	assert.NoError(err)

	// Verify role
	assert.Equal(original.Role, roundTripped.Role)

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
		if orig.Attachment != nil {
			assert.NotNil(rt.Attachment, "block %d: attachment should survive round-trip", i)
			if len(orig.Attachment.Data) > 0 {
				// Base64 attachments preserve media type
				assert.Equal(orig.Attachment.Type, rt.Attachment.Type)
			}
			if orig.Attachment.URL != nil {
				// URL attachments preserve URL but not media type
				assert.NotNil(rt.Attachment.URL)
				assert.Equal(orig.Attachment.URL.String(), rt.Attachment.URL.String())
			}
		}
		if orig.ToolCall != nil {
			assert.NotNil(rt.ToolCall, "block %d: tool_call should survive round-trip", i)
			assert.Equal(orig.ToolCall.Name, rt.ToolCall.Name)
			assert.Equal(orig.ToolCall.ID, rt.ToolCall.ID)
		}
		if orig.ToolResult != nil {
			assert.NotNil(rt.ToolResult, "block %d: tool_result should survive round-trip", i)
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
