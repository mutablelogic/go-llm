package resource_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	resource "github.com/mutablelogic/go-llm/pkg/toolkit/resource"
	assert "github.com/stretchr/testify/assert"
)

// errResource is a test helper: an llm.Resource whose MarshalJSON always fails.
// It is used to exercise the marshalResource error path via WithNamespace/WithDescription.
type errResource struct{}

func (e *errResource) URI() string                            { return "err:test" }
func (e *errResource) Name() string                           { return "test" }
func (e *errResource) Description() string                    { return "" }
func (e *errResource) Type() string                           { return "application/octet-stream" }
func (e *errResource) Read(_ context.Context) ([]byte, error) { return nil, nil }
func (e *errResource) MarshalJSON() ([]byte, error)           { return nil, errors.New("marshal error") }

var _ llm.Resource = (*errResource)(nil)

///////////////////////////////////////////////////////////////////////////////
// TEXT

func Test_Text_001(t *testing.T) {
	// Create a text resource and check interface fields
	assert := assert.New(t)
	r, err := resource.Text("greeting", "hello world")
	assert.NoError(err)
	assert.NotNil(r)
	assert.Equal("greeting", r.Name())
	assert.Equal("text:greeting", r.URI())
	assert.Equal("text/plain", r.Type())
	assert.Equal("", r.Description())
	data, err := r.Read(context.Background())
	assert.NoError(err)
	assert.Equal([]byte("hello world"), data)
}

func Test_Text_002(t *testing.T) {
	// Invalid name returns error
	assert := assert.New(t)
	_, err := resource.Text("", "hello")
	assert.Error(err)
	_, err = resource.Text("has space", "hello")
	assert.Error(err)
}

func Test_Text_003(t *testing.T) {
	// MarshalJSON + Unmarshal round-trip preserves content
	assert := assert.New(t)
	r, err := resource.Text("note", "some text")
	assert.NoError(err)
	b, err := json.Marshal(r)
	assert.NoError(err)
	r2, err := resource.Unmarshal(b)
	assert.NoError(err)
	assert.Equal(r.Name(), r2.Name())
	assert.Equal(r.Type(), r2.Type())
	data, err := r2.Read(context.Background())
	assert.NoError(err)
	assert.Equal([]byte("some text"), data)
}

///////////////////////////////////////////////////////////////////////////////
// JSON

func Test_JSON_001(t *testing.T) {
	// Create a JSON resource from a struct and check interface fields
	assert := assert.New(t)
	v := map[string]any{"key": "value", "count": 42.0}
	r, err := resource.JSON("result", v)
	assert.NoError(err)
	assert.NotNil(r)
	assert.Equal("result", r.Name())
	assert.Equal("json:result", r.URI())
	assert.Equal("application/json", r.Type())
}

func Test_JSON_002(t *testing.T) {
	// MarshalJSON + Unmarshal round-trip preserves raw JSON
	assert := assert.New(t)
	raw := json.RawMessage(`{"x":1,"y":2}`)
	r, err := resource.JSON("coords", raw)
	assert.NoError(err)
	b, err := json.Marshal(r)
	assert.NoError(err)
	r2, err := resource.Unmarshal(b)
	assert.NoError(err)
	assert.Equal(r.Name(), r2.Name())
	assert.Equal(r.Type(), r2.Type())
	data, err := r2.Read(context.Background())
	assert.NoError(err)
	assert.JSONEq(string(raw), string(data))
}

func Test_JSON_003(t *testing.T) {
	// Invalid name returns error
	assert := assert.New(t)
	_, err := resource.JSON("bad name", nil)
	assert.Error(err)
}

///////////////////////////////////////////////////////////////////////////////
// DATA

func Test_Data_001(t *testing.T) {
	// Create data resource from raw bytes; MIME type is content-sniffed
	assert := assert.New(t)
	content := []byte("hello binary")
	r, err := resource.Data("blob", content)
	assert.NoError(err)
	assert.NotNil(r)
	assert.Equal("blob", r.Name())
	// "hello binary" is sniffed as text/plain, so Data() returns a textResource.
	assert.Equal("text:blob", r.URI())
	assert.NotEmpty(r.Type())
	data, err := r.Read(context.Background())
	assert.NoError(err)
	assert.Equal(content, data)
}

func Test_Data_002(t *testing.T) {
	// Read from guggenheim.jpg — name derived from filename, MIME is image/jpeg
	assert := assert.New(t)
	f, err := os.Open("testdata/guggenheim.jpg")
	assert.NoError(err)
	defer f.Close()
	r, err := resource.Read(f)
	assert.NoError(err)
	assert.Equal("guggenheim", r.Name())
	assert.Equal("image/jpeg", r.Type())
	data, err := r.Read(context.Background())
	assert.NoError(err)
	assert.NotEmpty(data)
}

func Test_Data_003(t *testing.T) {
	// MarshalJSON + Unmarshal round-trip for binary data preserves bytes
	assert := assert.New(t)
	content := []byte{0x00, 0x01, 0x02, 0x03}
	r, err := resource.Data("payload", content)
	assert.NoError(err)
	b, err := json.Marshal(r)
	assert.NoError(err)
	r2, err := resource.Unmarshal(b)
	assert.NoError(err)
	assert.Equal(r.Name(), r2.Name())
	assert.Equal(r.Type(), r2.Type())
	data, err := r2.Read(context.Background())
	assert.NoError(err)
	assert.Equal(content, data)
}

///////////////////////////////////////////////////////////////////////////////
// NAMESPACE & DESCRIPTION WRAPPERS

func Test_WithNamespace_001(t *testing.T) {
	// WithNamespace prefixes the name and preserves other fields
	assert := assert.New(t)
	r, err := resource.Text("item", "content")
	assert.NoError(err)
	nr := resource.WithNamespace("myns", r)
	assert.Equal("myns.item", nr.Name())
	assert.Equal(r.Type(), nr.Type())
}

func Test_WithNamespace_002(t *testing.T) {
	// MarshalJSON of a namespaced resource emits the namespaced name
	assert := assert.New(t)
	r, err := resource.Text("item", "content")
	assert.NoError(err)
	nr := resource.WithNamespace("myns", r)
	b, err := json.Marshal(nr)
	assert.NoError(err)
	var v map[string]any
	assert.NoError(json.Unmarshal(b, &v))
	assert.Equal("myns.item", v["name"])
}

func Test_WithDescription_001(t *testing.T) {
	// WithDescription overrides description; MarshalJSON + Unmarshal round-trip
	assert := assert.New(t)
	r, err := resource.Text("doc", "body text")
	assert.NoError(err)
	dr := resource.WithDescription("a description", r)
	assert.Equal("a description", dr.Description())
	b, err := json.Marshal(dr)
	assert.NoError(err)
	r2, err := resource.Unmarshal(b)
	assert.NoError(err)
	assert.Equal("a description", r2.Description())
	assert.Equal("doc", r2.Name())
}

func Test_WithURI_001(t *testing.T) {
	// WithURI overrides the URI while preserving name, type, and data
	assert := assert.New(t)
	r, err := resource.Text("item", "content")
	assert.NoError(err)
	ur := resource.WithURI("custom:my-uri", r)
	assert.Equal("custom:my-uri", ur.URI())
	assert.Equal("item", ur.Name())
	assert.Equal("text/plain", ur.Type())
	data, err := ur.Read(context.Background())
	assert.NoError(err)
	assert.Equal([]byte("content"), data)
}

func Test_WithURI_002(t *testing.T) {
	// MarshalJSON emits the overridden URI
	assert := assert.New(t)
	r, err := resource.Text("item", "content")
	assert.NoError(err)
	ur := resource.WithURI("custom:my-uri", r)
	b, err := json.Marshal(ur)
	assert.NoError(err)
	var v map[string]any
	assert.NoError(json.Unmarshal(b, &v))
	assert.Equal("custom:my-uri", v["uri"])
	assert.Equal("item", v["name"])
}

func Test_WithURI_003(t *testing.T) {
	// WithURI combined with WithNamespace: both overrides appear in marshaled JSON
	assert := assert.New(t)
	r, err := resource.Text("item", "content")
	assert.NoError(err)
	ur := resource.WithURI("custom:my-uri", resource.WithNamespace("myns", r))
	b, err := json.Marshal(ur)
	assert.NoError(err)
	var v map[string]any
	assert.NoError(json.Unmarshal(b, &v))
	assert.Equal("custom:my-uri", v["uri"])
	assert.Equal("myns.item", v["name"])
}

func Test_WithURI_004(t *testing.T) {
	// WithURI combined with WithDescription: both overrides appear in marshaled JSON
	assert := assert.New(t)
	r, err := resource.Text("item", "content")
	assert.NoError(err)
	ur := resource.WithURI("custom:my-uri", resource.WithDescription("my description", r))
	b, err := json.Marshal(ur)
	assert.NoError(err)
	var v map[string]any
	assert.NoError(json.Unmarshal(b, &v))
	assert.Equal("custom:my-uri", v["uri"])
	assert.Equal("my description", v["description"])
}

///////////////////////////////////////////////////////////////////////////////
// DATA — additional coverage

func Test_Data_004_invalid_name(t *testing.T) {
	assert := assert.New(t)
	_, err := resource.Data("bad name!", []byte("x"))
	assert.Error(err)
}

func Test_Data_005_mime_from_extension(t *testing.T) {
	assert := assert.New(t)
	// Pass a filename with a .jpg extension so MIME is resolved from extension.
	content, err := os.ReadFile("testdata/guggenheim.jpg")
	assert.NoError(err)
	r, err := resource.Data("photo.jpg", content)
	assert.NoError(err)
	assert.Equal("photo", r.Name())
	assert.Equal("image/jpeg", r.Type())
}

func Test_Data_006_description(t *testing.T) {
	assert := assert.New(t)
	r, err := resource.Data("blob", []byte("x"))
	assert.NoError(err)
	// dataResource.Description() always returns "".
	assert.Equal("", r.Description())
}

func Test_Data_007_read_from_non_file_reader(t *testing.T) {
	// Read from a non-*os.File reader; name defaults to "data".
	assert := assert.New(t)
	sr := strings.NewReader("hello from reader")
	r, err := resource.Read(sr)
	assert.NoError(err)
	assert.Equal("data", r.Name())
	data, err := r.Read(context.Background())
	assert.NoError(err)
	assert.Equal([]byte("hello from reader"), data)
}

func Test_Data_008_read_error(t *testing.T) {
	// errReader always returns an error on Read.
	assert := assert.New(t)
	_, err := resource.Read(&errReader{})
	assert.Error(err)
}

///////////////////////////////////////////////////////////////////////////////
// JSON — additional coverage

func Test_JSON_004_description(t *testing.T) {
	assert := assert.New(t)
	r, err := resource.JSON("result", map[string]any{"k": "v"})
	assert.NoError(err)
	assert.Equal("", r.Description())
}

func Test_JSON_005_unmarshalable_value(t *testing.T) {
	// Passing a channel triggers json.Marshal error inside JSON().
	assert := assert.New(t)
	ch := make(chan int)
	_, err := resource.JSON("bad", ch)
	assert.Error(err)
}

///////////////////////////////////////////////////////////////////////////////
// Unmarshal — additional coverage

func Test_Unmarshal_001_malformed_json(t *testing.T) {
	assert := assert.New(t)
	_, err := resource.Unmarshal([]byte(`not json`))
	assert.Error(err)
}

func Test_Unmarshal_002_bad_text_data(t *testing.T) {
	// text field is not a JSON string — triggers text/plain error path.
	assert := assert.New(t)
	bad := []byte(`{"uri":"text:x","name":"x","type":"text/plain","text":123}`)
	_, err := resource.Unmarshal(bad)
	assert.Error(err)
}

func Test_Unmarshal_003_bad_binary_data(t *testing.T) {
	// blob field is not a JSON base64 bytes array — triggers default error path.
	assert := assert.New(t)
	bad := []byte(`{"uri":"data:x","name":"x","type":"application/octet-stream","blob":"not-base64-array"}`)
	_, err := resource.Unmarshal(bad)
	assert.Error(err)
}

///////////////////////////////////////////////////////////////////////////////
// marshalResource error path

func Test_MarshalResource_001_inner_marshal_error(t *testing.T) {
	// Wrap a resource whose MarshalJSON fails inside WithNamespace; marshalling
	// the wrapper should propagate the inner error.
	assert := assert.New(t)
	nr := resource.WithNamespace("ns", &errResource{})
	_, err := json.Marshal(nr)
	assert.Error(err)
}

///////////////////////////////////////////////////////////////////////////////
// helpers

// errReader returns an error on every Read call.
type errReader struct{}

func (e *errReader) Read(_ []byte) (int, error) { return 0, errors.New("read error") }
