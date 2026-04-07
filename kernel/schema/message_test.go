package schema_test

import (
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	// Packages
	uuid "github.com/google/uuid"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	pg "github.com/mutablelogic/go-pg"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
)

type messageMockRow struct {
	values []any
}

func (r messageMockRow) Scan(dest ...any) error {
	if len(dest) != len(r.values) {
		return errors.New("unexpected scan arity")
	}
	for i := range dest {
		switch target := dest[i].(type) {
		case *string:
			*target = r.values[i].(string)
		case *[]schema.ContentBlock:
			*target = r.values[i].([]schema.ContentBlock)
		case *uint:
			*target = r.values[i].(uint)
		case *map[string]any:
			*target = r.values[i].(map[string]any)
		default:
			return errors.New("unsupported scan target")
		}
	}
	return nil
}

// testdataPath returns the absolute path to the etc/testdata directory
func testdataPath(name string) string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "etc", "testdata", name)
}

func Test_NewMessage_001(t *testing.T) {
	// Simple text message
	assert := assert.New(t)
	msg, err := schema.NewMessage("user", "Hello, world!")
	assert.NoError(err)
	assert.NotNil(msg)
	assert.Equal("user", msg.Role)
	assert.Len(msg.Content, 1)
	assert.NotNil(msg.Content[0].Text)
	assert.Equal("Hello, world!", *msg.Content[0].Text)
	assert.Equal("Hello, world!", msg.Text())
}

func Test_NewMessage_002(t *testing.T) {
	// Assistant text message
	assert := assert.New(t)
	msg, err := schema.NewMessage("assistant", "I can help with that.")
	assert.NoError(err)
	assert.Equal("assistant", msg.Role)
	assert.Equal("I can help with that.", msg.Text())
}

func Test_NewMessage_003(t *testing.T) {
	// System text message
	assert := assert.New(t)
	msg, err := schema.NewMessage("system", "You are a helpful assistant.")
	assert.NoError(err)
	assert.Equal("system", msg.Role)
	assert.Equal("You are a helpful assistant.", msg.Text())
}

func Test_NewMessage_004(t *testing.T) {
	// Empty text message
	assert := assert.New(t)
	msg, err := schema.NewMessage("user", "")
	assert.NoError(err)
	assert.NotNil(msg)
	assert.Equal("", msg.Text())
}

func Test_NewMessage_005(t *testing.T) {
	// Text with attachment (image file)
	assert := assert.New(t)

	f, err := os.Open(testdataPath("guggenheim.jpg"))
	if !assert.NoError(err) {
		t.FailNow()
	}
	defer f.Close()

	msg, err := schema.NewMessage("user", "What is in this image?", schema.WithAttachment(f))
	assert.NoError(err)
	assert.NotNil(msg)
	assert.Equal("user", msg.Role)
	assert.Len(msg.Content, 2)

	// First block is text
	assert.NotNil(msg.Content[0].Text)
	assert.Equal("What is in this image?", *msg.Content[0].Text)

	// Second block is attachment
	assert.NotNil(msg.Content[1].Attachment)
	assert.True(strings.HasPrefix(msg.Content[1].Attachment.ContentType, "image/jpeg"))
	assert.Greater(len(msg.Content[1].Attachment.Data), 0)
	assert.Nil(msg.Content[1].Attachment.URL)
}

func Test_NewMessage_006(t *testing.T) {
	// Text method concatenates multiple text blocks
	assert := assert.New(t)
	msg := &schema.Message{
		Role: "assistant",
		Content: []schema.ContentBlock{
			{Text: types.Ptr("Hello")},
			{Text: types.Ptr("World")},
		},
	}
	assert.Equal("Hello\nWorld", msg.Text())
}

func Test_NewMessage_007(t *testing.T) {
	// ToolCalls returns tool call blocks
	assert := assert.New(t)
	msg := &schema.Message{
		Role: "assistant",
		Content: []schema.ContentBlock{
			{Text: types.Ptr("Let me check that")},
			{ToolCall: &schema.ToolCall{ID: "call_1", Name: "get_weather"}},
			{ToolCall: &schema.ToolCall{ID: "call_2", Name: "get_time"}},
		},
	}
	calls := msg.ToolCalls()
	assert.Len(calls, 2)
	assert.Equal("get_weather", calls[0].Name)
	assert.Equal("get_time", calls[1].Name)
}

func Test_NewMessage_008(t *testing.T) {
	// String method doesn't panic
	assert := assert.New(t)
	msg, err := schema.NewMessage("user", "test")
	assert.NoError(err)
	assert.NotEmpty(msg.String())
}

func Test_NewMessage_009(t *testing.T) {
	// Text with URL attachment
	assert := assert.New(t)

	msg, err := schema.NewMessage("user", "Describe this image", schema.WithAttachmentURL("gs://my-bucket/image.png", "image/png"))
	assert.NoError(err)
	assert.NotNil(msg)
	assert.Len(msg.Content, 2)

	// First block is text
	assert.NotNil(msg.Content[0].Text)
	assert.Equal("Describe this image", *msg.Content[0].Text)

	// Second block is URL attachment
	att := msg.Content[1].Attachment
	assert.NotNil(att)
	assert.Equal("image/png", att.ContentType)
	assert.Nil(att.Data)
	assert.NotNil(att.URL)
	assert.Equal("gs://my-bucket/image.png", att.URL.String())
}

func Test_NewMessage_010(t *testing.T) {
	// Multiple attachments on one message
	assert := assert.New(t)

	f, err := os.Open(testdataPath("guggenheim.jpg"))
	if !assert.NoError(err) {
		t.FailNow()
	}
	defer f.Close()

	msg, err := schema.NewMessage("user", "Compare these images",
		schema.WithAttachment(f),
		schema.WithAttachmentURL("https://example.com/photo.png", "image/png"),
	)
	assert.NoError(err)
	assert.NotNil(msg)
	assert.Len(msg.Content, 3)

	// First block is text
	assert.NotNil(msg.Content[0].Text)
	assert.Equal("Compare these images", *msg.Content[0].Text)

	// Second block is inline data attachment
	assert.NotNil(msg.Content[1].Attachment)
	assert.True(strings.HasPrefix(msg.Content[1].Attachment.ContentType, "image/jpeg"))
	assert.Greater(len(msg.Content[1].Attachment.Data), 0)

	// Third block is URL attachment
	assert.NotNil(msg.Content[2].Attachment)
	assert.Equal("image/png", msg.Content[2].Attachment.ContentType)
	assert.Equal("https://example.com/photo.png", msg.Content[2].Attachment.URL.String())
}

func TestAttachmentMarshalJSONURLAsString(t *testing.T) {
	assert := assert.New(t)
	attachment := schema.Attachment{
		ContentType: "application/pdf",
		Data:        []byte("pdf"),
		URL:         urlFromString(t, "file:///Users/djt/Desktop/spec.pdf"),
	}

	data, err := json.Marshal(attachment)
	if !assert.NoError(err) {
		return
	}

	assert.JSONEq(`{"type":"application/pdf","data":"cGRm","url":"file:///Users/djt/Desktop/spec.pdf"}`, string(data))
}

func TestAttachmentUnmarshalJSONStringURL(t *testing.T) {
	assert := assert.New(t)
	var attachment schema.Attachment

	err := json.Unmarshal([]byte(`{"type":"application/pdf","url":"file:///Users/djt/Desktop/spec.pdf"}`), &attachment)
	if !assert.NoError(err) {
		return
	}

	if assert.NotNil(attachment.URL) {
		assert.Equal("file:///Users/djt/Desktop/spec.pdf", attachment.URL.String())
		assert.Equal("/Users/djt/Desktop/spec.pdf", attachment.URL.Path)
	}
}

func TestAttachmentUnmarshalLegacyObjectURL(t *testing.T) {
	assert := assert.New(t)
	var attachment schema.Attachment

	err := json.Unmarshal([]byte(`{"type":"application/pdf","url":{"Scheme":"file","Host":"","Path":"/Users/djt/Desktop/spec.pdf"}}`), &attachment)
	if !assert.NoError(err) {
		return
	}

	if assert.NotNil(attachment.URL) {
		assert.Equal("file:///Users/djt/Desktop/spec.pdf", attachment.URL.String())
		assert.Equal("/Users/djt/Desktop/spec.pdf", attachment.URL.Path)
	}
}

func Test_NewToolResult_001(t *testing.T) {
	// Simple tool result
	assert := assert.New(t)
	content := map[string]any{"temperature": 20, "unit": "celsius"}
	block := schema.NewToolResult("call_123", "get_weather", content)

	tr := block.ToolResult
	assert.NotNil(tr)
	assert.Equal("call_123", tr.ID)
	assert.Equal("get_weather", tr.Name)
	assert.JSONEq(`{"temperature":20,"unit":"celsius"}`, string(tr.Content))
	assert.False(tr.IsError)
}

func urlFromString(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse url %q: %v", raw, err)
	}
	return u
}

func Test_NewToolError_001(t *testing.T) {
	// Tool error
	assert := assert.New(t)
	block := schema.NewToolError("call_456", "get_weather", errors.New("city not found"))

	tr := block.ToolResult
	assert.NotNil(tr)
	assert.Equal("call_456", tr.ID)
	assert.Equal("get_weather", tr.Name)
	assert.True(tr.IsError)
	assert.Contains(string(tr.Content), "city not found")
}

func TestMessageInsertBindsSessionMessage(t *testing.T) {
	assert := assert.New(t)
	b := pg.NewBind("schema", "llm", "message.insert", "INSERT")
	sessionID := uuid.New()

	query, err := (schema.MessageInsert{
		Session: sessionID,
		Message: schema.Message{
			Role: schema.RoleAssistant,
			Content: []schema.ContentBlock{
				{Text: types.Ptr("hello")},
			},
			Tokens: 12,
			Result: schema.ResultStop,
			Meta:   map[string]any{"thought": true},
		},
	}).Insert(b)
	if !assert.NoError(err) {
		return
	}

	assert.Equal("INSERT", query)
	assert.Equal(sessionID, b.Get("session"))
	assert.Equal(schema.RoleAssistant, b.Get("role"))
	if content, ok := b.Get("content").([]schema.ContentBlock); assert.True(ok) && assert.Len(content, 1) {
		if assert.NotNil(content[0].Text) {
			assert.Equal("hello", *content[0].Text)
		}
	}
	assert.Equal(uint(12), b.Get("tokens"))
	assert.Equal(schema.ResultStop.String(), b.Get("result"))
	assert.Equal(map[string]any{"thought": true}, b.Get("meta"))
}

func TestMessageInsertUserMessageBindsNullResultAndTokens(t *testing.T) {
	assert := assert.New(t)
	b := pg.NewBind("schema", "llm", "message.insert", "INSERT")

	_, err := (schema.MessageInsert{
		Session: uuid.New(),
		Message: schema.Message{
			Role:    schema.RoleUser,
			Content: []schema.ContentBlock{{Text: types.Ptr("hi")}},
		},
	}).Insert(b)
	if !assert.NoError(err) {
		return
	}

	assert.Nil(b.Get("tokens"))
	assert.Nil(b.Get("result"))
	assert.Nil(b.Get("meta"))
}

func TestMessageInsertRequiresSessionAndRole(t *testing.T) {
	assert := assert.New(t)
	b := pg.NewBind("schema", "llm", "message.insert", "INSERT")

	_, err := (schema.MessageInsert{Message: schema.Message{Role: schema.RoleUser}}).Insert(b)
	if assert.Error(err) {
		assert.ErrorIs(err, schema.ErrBadParameter)
	}

	_, err = (schema.MessageInsert{Session: uuid.New()}).Insert(b)
	if assert.Error(err) {
		assert.ErrorIs(err, schema.ErrBadParameter)
	}
}

func TestMessageListRequestQuery(t *testing.T) {
	assert := assert.New(t)
	limit := uint64(25)
	values := (schema.MessageListRequest{
		OffsetLimit: pg.OffsetLimit{Offset: 5, Limit: &limit},
		Role:        schema.RoleAssistant,
		Text:        "release notes",
	}).Query()

	assert.Equal("5", values.Get("offset"))
	assert.Equal("25", values.Get("limit"))
	assert.Equal(schema.RoleAssistant, values.Get("role"))
	assert.Equal("release notes", values.Get("text"))
}

func TestMessageListRequestSelect(t *testing.T) {
	assert := assert.New(t)
	sessionID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	b := pg.NewBind("schema", "llm", "message.list", "LIST")
	b.Set("session", sessionID)

	query, err := (schema.MessageListRequest{
		Role: schema.RoleAssistant,
		Text: "release notes",
	}).Select(b, pg.List)
	if !assert.NoError(err) {
		return
	}

	assert.Equal("LIST", query)
	assert.Equal(sessionID, b.Get("session"))
	assert.Equal(schema.RoleAssistant, b.Get("role"))
	assert.Equal("%release notes%", b.Get("text"))
	assert.Contains(b.Get("where").(string), `message.session =`)
	assert.Contains(b.Get("where").(string), `message.role =`)
	assert.Contains(b.Get("where").(string), `message.content::text ILIKE`)
	assert.Equal(`ORDER BY message.created_at ASC, message.id ASC`, b.Get("orderby"))
}

func TestMessageListRequestSelectForUser(t *testing.T) {
	assert := assert.New(t)
	b := pg.NewBind("schema", "llm", "message.list_for_user", "LIST")
	b.Set("session", uuid.MustParse("11111111-1111-1111-1111-111111111111"))
	b.Set("user", uuid.MustParse("22222222-2222-2222-2222-222222222222"))

	query, err := (schema.MessageListRequest{Role: schema.RoleUser}).Select(b, pg.List)
	if !assert.NoError(err) {
		return
	}

	assert.Equal("LIST", query)
	assert.Contains(b.Get("where").(string), `AND message.role =`)
}

func TestMessageScanAndListScan(t *testing.T) {
	assert := assert.New(t)
	message := new(schema.Message)
	text := types.Ptr("hello")
	row := messageMockRow{values: []any{schema.RoleAssistant, []schema.ContentBlock{{Text: text}}, uint(7), schema.ResultStop.String(), map[string]any{"source": "test"}}}

	if !assert.NoError(message.Scan(row)) {
		return
	}

	assert.Equal(schema.RoleAssistant, message.Role)
	if assert.Len(message.Content, 1) && assert.NotNil(message.Content[0].Text) {
		assert.Equal("hello", *message.Content[0].Text)
	}
	assert.Equal(uint(7), message.Tokens)
	assert.Equal(schema.ResultStop, message.Result)
	assert.Equal(map[string]any{"source": "test"}, message.Meta)

	list := new(schema.MessageList)
	if !assert.NoError(list.Scan(messageMockRow{values: []any{schema.RoleUser, []schema.ContentBlock{{Text: types.Ptr("hi")}}, uint(3), "", map[string]any{}}})) {
		return
	}
	if assert.Len(list.Body, 1) {
		assert.Equal(schema.RoleUser, list.Body[0].Role)
		assert.Equal(schema.ResultStop, list.Body[0].Result)
	}

	if !assert.NoError(list.ScanCount(messageMockRow{values: []any{uint(42)}})) {
		return
	}
	assert.Equal(uint(42), list.Count)
}

func TestMessageListRequestRequiresSessionBinding(t *testing.T) {
	assert := assert.New(t)
	b := pg.NewBind("schema", "llm", "message.list", "LIST")

	_, err := (schema.MessageListRequest{}).Select(b, pg.List)
	if assert.Error(err) {
		assert.ErrorIs(err, schema.ErrBadParameter)
	}
}
