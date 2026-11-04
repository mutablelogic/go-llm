package httpclient_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	// Packages
	test "github.com/mutablelogic/go-llm/registry/test"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	httpclient "github.com/mutablelogic/go-llm/toolkit/httpclient"
	httphandler "github.com/mutablelogic/go-llm/toolkit/httphandler"
	schema "github.com/mutablelogic/go-llm/toolkit/schema"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// tool is a tool for tests
type tool struct {
	toolkit.Base
	name string
}

func (t tool) Name() string                    { return t.name }
func (t tool) Description() string             { return "A tool for tests" }
func (t tool) InputSchema() *jsonschema.Schema { return nil }
func (t tool) Run(context.Context, json.RawMessage) (any, error) {
	return nil, nil
}

// echoTool returns its input, which must have a city
type echoTool struct {
	tool
}

type echoInput struct {
	City string `json:"city" help:"City name"`
}

func (echoTool) InputSchema() *jsonschema.Schema { return jsonschema.MustFor[echoInput]() }
func (echoTool) Run(_ context.Context, input json.RawMessage) (any, error) {
	return input, nil
}

// newClient returns a client for a test server serving a toolkit with tools
// in three namespaces
func newClient(t *testing.T) (*httpclient.Client, context.Context) {
	t.Helper()
	ctx := context.Background()
	tools, err := toolkit.New(ctx)
	require.NoError(t, err)
	require.NoError(t, tools.Add(toolkit.NewNamespace("a", "", tool{name: "one"}, tool{name: "two"})))
	require.NoError(t, tools.Add(toolkit.NewNamespace("b", "", tool{name: "three"})))
	require.NoError(t, tools.Add(toolkit.NewNamespace("c", "", echoTool{tool{name: "echo"}})))

	server := test.NewServerFunc(t, func(router *httprouter.Router) error {
		return httphandler.RegisterHandlers(tools, router)
	})
	client, err := httpclient.New(server.URL + test.Prefix)
	require.NoError(t, err)
	return client, ctx
}

func TestListTools(t *testing.T) {
	client, ctx := newClient(t)

	t.Run("All", func(t *testing.T) {
		list, err := client.ListTools(ctx, schema.ToolListRequest{})
		require.NoError(t, err)
		assert.Equal(t, uint64(4), list.Count)
		require.Len(t, list.Body, 4)
		assert.Equal(t, "one", list.Body[0].Name)
		assert.Equal(t, "a:one", list.Body[0].Canonical)
		assert.Equal(t, "A tool for tests", list.Body[0].Description)
	})

	t.Run("Namespace", func(t *testing.T) {
		list, err := client.ListTools(ctx, schema.ToolListRequest{Namespace: "b"})
		require.NoError(t, err)
		assert.Equal(t, uint64(1), list.Count)
		require.Len(t, list.Body, 1)
		assert.Equal(t, "b:three", list.Body[0].Canonical)
	})

	t.Run("InvalidNamespace", func(t *testing.T) {
		_, err := client.ListTools(ctx, schema.ToolListRequest{Namespace: "not valid"})
		assert.Equal(t, http.StatusBadRequest, status(t, err))
	})
}

func TestCallTool(t *testing.T) {
	client, ctx := newClient(t)

	t.Run("Call", func(t *testing.T) {
		result, err := client.CallTool(ctx, "c:echo", json.RawMessage(`{"city":"London"}`))
		require.NoError(t, err)
		assert.JSONEq(t, `{"city":"London"}`, string(result))
	})

	t.Run("Name", func(t *testing.T) {
		// A name without a namespace refers to the only tool with the name
		result, err := client.CallTool(ctx, "echo", json.RawMessage(`{"city":"London"}`))
		require.NoError(t, err)
		assert.JSONEq(t, `{"city":"London"}`, string(result))
	})

	t.Run("NoResult", func(t *testing.T) {
		// A tool without a result returns nil
		result, err := client.CallTool(ctx, "a:one", nil)
		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("InvalidInput", func(t *testing.T) {
		for _, input := range []string{`{}`, `{"city":1}`, ``} {
			_, err := client.CallTool(ctx, "c:echo", json.RawMessage(input))
			assert.Equal(t, http.StatusBadRequest, status(t, err), input)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		_, err := client.CallTool(ctx, "c:other", nil)
		assert.Equal(t, http.StatusNotFound, status(t, err))
	})
}

// status returns the HTTP status of an error from the client
func status(t *testing.T, err error) int {
	t.Helper()
	var response httpresponse.ErrResponse
	require.True(t, errors.As(err, &response), "error is %T: %v", err, err)
	return response.Code
}

func TestGetTool(t *testing.T) {
	client, ctx := newClient(t)

	t.Run("Get", func(t *testing.T) {
		tool, err := client.GetTool(ctx, "c:echo")
		require.NoError(t, err)
		assert.Equal(t, "echo", tool.Name)
		assert.Equal(t, "c:echo", tool.Canonical)
		assert.Equal(t, "A tool for tests", tool.Description)
		var input map[string]any
		require.NoError(t, json.Unmarshal(tool.Input, &input))
		assert.Contains(t, input["properties"], "city")
	})

	t.Run("Name", func(t *testing.T) {
		// A name without a namespace refers to the only tool with the name
		tool, err := client.GetTool(ctx, "three")
		require.NoError(t, err)
		assert.Equal(t, "b:three", tool.Canonical)
	})

	t.Run("NotFound", func(t *testing.T) {
		_, err := client.GetTool(ctx, "c:other")
		assert.Equal(t, http.StatusNotFound, status(t, err))
	})
}
