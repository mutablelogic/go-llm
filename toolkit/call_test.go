package toolkit_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	schema "github.com/mutablelogic/go-llm/toolkit/schema"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// echoTool returns its input, or the error when it's set
type echoTool struct {
	toolkit.Base
	name string
	err  error
}

func (t echoTool) Name() string                    { return t.name }
func (t echoTool) Description() string             { return "Returns its input" }
func (t echoTool) InputSchema() *jsonschema.Schema { return nil }
func (t echoTool) Run(ctx context.Context, input json.RawMessage) (any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if t.err != nil {
		return nil, t.err
	}
	return input, nil
}

func TestCallTool(t *testing.T) {
	ctx := context.Background()
	toolErr := errors.New("tool failed")
	tk, err := toolkit.New(ctx)
	require.NoError(t, err)
	require.NoError(t, tk.Add(toolkit.NewNamespace("test", "", echoTool{name: "echo"}, echoTool{name: "fail", err: toolErr})))

	t.Run("Run", func(t *testing.T) {
		result, err := tk.CallTool(ctx, "test:echo", json.RawMessage(`{"city":"London"}`))
		require.NoError(t, err)
		assert.JSONEq(t, `{"city":"London"}`, string(result))
	})

	t.Run("NoInput", func(t *testing.T) {
		result, err := tk.CallTool(ctx, "test:echo", nil)
		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("Error", func(t *testing.T) {
		// The tool's error is returned
		_, err := tk.CallTool(ctx, "test:fail", nil)
		assert.ErrorIs(t, err, toolErr)
	})

	t.Run("Name", func(t *testing.T) {
		// A name without a namespace refers to the only tool with the name
		result, err := tk.CallTool(ctx, "echo", json.RawMessage(`{"city":"London"}`))
		require.NoError(t, err)
		assert.JSONEq(t, `{"city":"London"}`, string(result))
	})

	t.Run("Ambiguous", func(t *testing.T) {
		tk, err := toolkit.New(ctx)
		require.NoError(t, err)
		require.NoError(t, tk.Add(toolkit.NewNamespace("a", "", echoTool{name: "echo"})))
		require.NoError(t, tk.Add(toolkit.NewNamespace("b", "", echoTool{name: "echo"})))
		_, err = tk.CallTool(ctx, "echo", nil)
		assert.ErrorIs(t, err, llm.ErrConflict)
		_, err = tk.CallTool(ctx, "b:echo", nil)
		assert.NoError(t, err)
	})

	t.Run("NotFound", func(t *testing.T) {
		for _, name := range []string{"test:other", "other", "other:echo", ""} {
			_, err := tk.CallTool(ctx, name, nil)
			assert.ErrorIs(t, err, llm.ErrNotFound, name)
		}
	})

	t.Run("InvalidInput", func(t *testing.T) {
		_, err := tk.CallTool(ctx, "test:echo", json.RawMessage(`{`))
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})

	t.Run("Cancelled", func(t *testing.T) {
		// The tool is run with the context
		ctx, cancel := context.WithCancel(ctx)
		cancel()
		_, err := tk.CallTool(ctx, "test:echo", nil)
		assert.ErrorIs(t, err, context.Canceled)
	})
}

// schemaTool has an input schema, from schema when it's set
type schemaTool struct {
	echoTool
	schema *jsonschema.Schema
}

type weatherInput struct {
	City string `json:"city" help:"City name"`
	Days int    `json:"days,omitempty" help:"Number of days" min:"1" max:"14"`
}

type nestedInput struct {
	Weather weatherInput `json:"weather"`
}

func (t schemaTool) InputSchema() *jsonschema.Schema {
	if t.schema != nil {
		return t.schema
	}
	return jsonschema.MustFor[weatherInput]()
}

func TestCallToolSchema(t *testing.T) {
	ctx := context.Background()
	tk, err := toolkit.New(ctx)
	require.NoError(t, err)
	require.NoError(t, tk.Add(toolkit.NewNamespace("test", "", schemaTool{echoTool: echoTool{name: "weather"}},
		schemaTool{echoTool: echoTool{name: "nested"}, schema: jsonschema.MustFor[nestedInput]().Property("weather")})))

	t.Run("Valid", func(t *testing.T) {
		for _, input := range []string{`{"city":"London"}`, `{"city":"London","days":3}`} {
			_, err := tk.CallTool(ctx, "test:weather", json.RawMessage(input))
			assert.NoError(t, err, input)
		}
	})

	t.Run("Invalid", func(t *testing.T) {
		// The tool isn't run when the input doesn't match its schema
		for name, input := range map[string]string{
			"MissingCity": `{"days":3}`,
			"WrongType":   `{"city":1}`,
			"TooManyDays": `{"city":"London","days":20}`,
			"NotAnObject": `"London"`,
			"Empty":       ``,
			"EmptyObject": `{}`,
		} {
			_, err := tk.CallTool(ctx, "test:weather", json.RawMessage(input))
			assert.ErrorIs(t, err, llm.ErrBadParameter, name)
		}
	})

	t.Run("Nulls", func(t *testing.T) {
		// A field which is null is left out, as models send null for the
		// fields they don't set, so an optional field which is null is
		// valid, and isn't given to the tool, but a required one isn't
		result, err := tk.CallTool(ctx, "test:weather", json.RawMessage(`{"city":"London","days":null}`))
		require.NoError(t, err)
		assert.JSONEq(t, `{"city":"London"}`, string(result))
		_, err = tk.CallTool(ctx, "test:weather", json.RawMessage(`{"city":null}`))
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})

	t.Run("Nullable", func(t *testing.T) {
		// A field which is null, which the schema allows, is kept, as is
		// one alongside it, and so is a null which the respond tool's
		// schema allows
		nullable, err := jsonschema.FromJSON(json.RawMessage(`{"type":"object","properties":{"note":{"type":["string","null"]}}}`))
		require.NoError(t, err)
		respond, err := toolkit.NewRespond(json.RawMessage(`{"type":"null"}`))
		require.NoError(t, err)
		tk, err := toolkit.New(ctx)
		require.NoError(t, err)
		require.NoError(t, tk.Add(toolkit.NewNamespace("nulls", "", schemaTool{echoTool{name: "note"}, nullable}, respond)))

		result, err := tk.CallTool(ctx, "nulls:note", json.RawMessage(`{"note":null}`))
		require.NoError(t, err)
		assert.JSONEq(t, `{"note":null}`, string(result))
		_, err = tk.CallTool(ctx, "nulls:"+respond.Name(), json.RawMessage(`{"result":null}`))
		require.NoError(t, err)
	})

	t.Run("NoSchemaNulls", func(t *testing.T) {
		// A tool without an input schema has its input, with its nulls
		tk, err := toolkit.New(ctx)
		require.NoError(t, err)
		require.NoError(t, tk.Add(toolkit.NewNamespace("plain", "", echoTool{name: "echo"})))
		result, err := tk.CallTool(ctx, "plain:echo", json.RawMessage(`{"city":null}`))
		require.NoError(t, err)
		assert.JSONEq(t, `{"city":null}`, string(result))
	})

	t.Run("UnresolvedSchema", func(t *testing.T) {
		// A schema which wasn't resolved, from Property, is resolved when
		// the tool is added, so it can be validated
		_, err := tk.CallTool(ctx, "test:nested", json.RawMessage(`{"city":"London"}`))
		assert.NoError(t, err)
		_, err = tk.CallTool(ctx, "test:nested", json.RawMessage(`{"days":3}`))
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})

	t.Run("NoSchema", func(t *testing.T) {
		// A tool without an input schema isn't validated
		tk, err := toolkit.New(ctx)
		require.NoError(t, err)
		require.NoError(t, tk.Add(toolkit.NewNamespace("test", "", echoTool{name: "echo"})))
		_, err = tk.CallTool(ctx, "test:echo", json.RawMessage(`"anything"`))
		assert.NoError(t, err)
	})
}

// resultTool returns its result
type resultTool struct {
	echoTool
	result any
}

func (t resultTool) Run(context.Context, json.RawMessage) (any, error) {
	return t.result, nil
}

// resource is a resource for tests, which fails to be read when err is set
type resource struct {
	err error
}

var _ llm.Resource = resource{}

func (resource) URI() string         { return "" }
func (resource) Name() string        { return "image.png" }
func (resource) Description() string { return "" }
func (resource) Type() string        { return "image/png" }
func (r resource) Read(context.Context) (llm.ReadFunc, error) {
	if r.err != nil {
		return nil, r.err
	}
	return llm.ReadBytes([]byte("png")), nil
}

func TestCallToolResult(t *testing.T) {
	ctx := context.Background()
	readErr := errors.New("can't read")
	tk, err := toolkit.New(ctx)
	require.NoError(t, err)
	require.NoError(t, tk.Add(toolkit.NewNamespace("test", "", resultTool{echoTool: echoTool{name: "json"}, result: json.RawMessage(`{"temperature_c":18}`)},
		resultTool{echoTool: echoTool{name: "text"}, result: "It's sunny"},
		resultTool{echoTool: echoTool{name: "resource"}, result: resource{}},
		resultTool{echoTool: echoTool{name: "struct"}, result: struct {
			A int `json:"a"`
		}{1}},
		resultTool{echoTool: echoTool{name: "bytes"}, result: []byte("png")},
		resultTool{echoTool: echoTool{name: "nil"}, result: nil},
		resultTool{echoTool: echoTool{name: "empty"}, result: json.RawMessage{}},
		resultTool{echoTool: echoTool{name: "invalid"}, result: json.RawMessage(`{`)},
		resultTool{echoTool: echoTool{name: "unencodable"}, result: make(chan int)},
		resultTool{echoTool: echoTool{name: "unreadable"}, result: resource{err: readErr}},
		resultTool{echoTool: echoTool{name: "resources"}, result: []llm.Resource{resource{}, resource{}}},
		resultTool{echoTool: echoTool{name: "result"}, result: toolkit.Result{Value: map[string]int{"pages": 2}, Resources: []llm.Resource{resource{}}}},
		resultTool{echoTool: echoTool{name: "one"}, result: []llm.Resource{resource{}}},
		resultTool{echoTool: echoTool{name: "alone"}, result: &toolkit.Result{Resources: []llm.Resource{resource{}}}})))

	t.Run("JSON", func(t *testing.T) {
		result, err := tk.CallTool(ctx, "test:json", nil)
		require.NoError(t, err)
		assert.JSONEq(t, `{"temperature_c":18}`, string(result))
	})

	t.Run("Encoded", func(t *testing.T) {
		// Text, a resource, and other results are encoded as JSON
		for name, want := range map[string]string{
			"test:text":     `"It's sunny"`,
			"test:resource": `{"type":"image/png","data":"cG5n"}`,
			"test:struct":   `{"a":1}`,
			"test:bytes":    `"cG5n"`,
		} {
			result, err := tk.CallTool(ctx, name, nil)
			require.NoError(t, err, name)
			assert.JSONEq(t, want, string(result), name)
		}
	})

	t.Run("NoResult", func(t *testing.T) {
		for _, name := range []string{"test:nil", "test:empty"} {
			result, err := tk.CallTool(ctx, name, nil)
			require.NoError(t, err, name)
			assert.Nil(t, result, name)
		}
	})

	t.Run("Invalid", func(t *testing.T) {
		// Invalid JSON, or a result which can't be encoded, is an error
		for _, name := range []string{"test:invalid", "test:unencodable"} {
			_, err := tk.CallTool(ctx, name, nil)
			assert.ErrorIs(t, err, llm.ErrInternalServerError, name)
		}
	})

	t.Run("Resources", func(t *testing.T) {
		// Without a store, resources are their types and data, with the
		// value the tool returned with them
		result, err := tk.CallTool(ctx, "test:resources", nil)
		require.NoError(t, err)
		assert.JSONEq(t, `{"resources":[{"type":"image/png","data":"cG5n"},{"type":"image/png","data":"cG5n"}]}`, string(result))
		result, err = tk.CallTool(ctx, "test:result", nil)
		require.NoError(t, err)
		assert.JSONEq(t, `{"result":{"pages":2},"resources":[{"type":"image/png","data":"cG5n"}]}`, string(result))

		// A list of one, or a Result with one, without a value, has the same
		// shape, however many resources it has
		for _, name := range []string{"test:one", "test:alone"} {
			result, err = tk.CallTool(ctx, name, nil)
			require.NoError(t, err, name)
			assert.JSONEq(t, `{"resources":[{"type":"image/png","data":"cG5n"}]}`, string(result), name)
		}

		// With a store, they're kept, and the result has links to them
		var stored []string
		stores := toolkit.WithStore(ctx, func(ctx context.Context, r llm.Resource) (schema.ResourceLink, error) {
			data, err := llm.ReadAll(ctx, r)
			if err != nil {
				return schema.ResourceLink{}, err
			}
			stored = append(stored, string(data))
			return schema.ResourceLink{URI: "resource://abc", Type: r.Type(), Size: int64(len(data))}, nil
		})
		for name, want := range map[string]string{
			"test:resource":  `{"resources":[{"uri":"resource://abc","type":"image/png","size":3}]}`,
			"test:resources": `{"resources":[{"uri":"resource://abc","type":"image/png","size":3},{"uri":"resource://abc","type":"image/png","size":3}]}`,
			"test:result":    `{"result":{"pages":2},"resources":[{"uri":"resource://abc","type":"image/png","size":3}]}`,
		} {
			result, err := tk.CallTool(stores, name, nil)
			require.NoError(t, err, name)
			assert.JSONEq(t, want, string(result), name)
		}
		assert.Len(t, stored, 4)

		// A resource which can't be kept is the tool's error
		failing := toolkit.WithStore(ctx, func(context.Context, llm.Resource) (schema.ResourceLink, error) {
			return schema.ResourceLink{}, readErr
		})
		_, err = tk.CallTool(failing, "test:resource", nil)
		assert.ErrorIs(t, err, readErr)
	})

	t.Run("Unreadable", func(t *testing.T) {
		// The error reading a resource is returned
		_, err := tk.CallTool(ctx, "test:unreadable", nil)
		assert.ErrorIs(t, err, readErr)
	})
}
