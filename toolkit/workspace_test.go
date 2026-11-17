package toolkit_test

import (
	"context"
	"encoding/json"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	schema "github.com/mutablelogic/go-llm/toolkit/schema"
	workspace "github.com/mutablelogic/go-llm/workspace"
	workspaceschema "github.com/mutablelogic/go-llm/workspace/schema"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// notes is a namespace, with no tools, which is a workspace of one note
type notes struct{}

func (notes) Name() string         { return "notes" }
func (notes) Description() string  { return "My notes" }
func (notes) SystemPrompt() string { return "" }
func (notes) Type() string         { return schema.NamespaceBuiltin }
func (notes) Tools() []llm.Tool    { return nil }
func (notes) Watch(ctx context.Context, ch chan<- eventschema.Event) error {
	return toolkit.WatchTools(ctx, ch, nil)
}
func (notes) List(context.Context, workspaceschema.ListRequest) (*workspaceschema.ListResponse, error) {
	return &workspaceschema.ListResponse{Body: []workspaceschema.ResourceMeta{{URI: "file:///note.md", Name: "note.md", Type: "text/markdown"}}}, nil
}
func (notes) Read(_ context.Context, uri string) (llm.Resource, error) {
	if uri != "file:///note.md" {
		return nil, llm.ErrNotFound.With(uri)
	}
	return &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: "text/markdown", Data: []byte("# Note")}}, nil
}

// reader is a tool which reads the resource it's given
type reader struct{ toolkit.Base }

func (reader) Name() string                    { return "reader" }
func (reader) Description() string             { return "Read a resource" }
func (reader) InputSchema() *jsonschema.Schema { return nil }
func (reader) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var in struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, err
	}
	resource, err := toolkit.Resource(ctx, in.URI)
	if err != nil {
		return nil, err
	}
	data, err := llm.ReadAll(ctx, resource)
	if err != nil {
		return nil, err
	}
	return string(data), nil
}

func TestWorkspaces(t *testing.T) {
	ctx := context.Background()
	registry, err := workspace.New()
	require.NoError(t, err)
	tk, err := toolkit.New(ctx, toolkit.WithWorkspaces(registry))
	require.NoError(t, err)
	assert.Same(t, registry, tk.Workspaces())
	require.True(t, tk.HasNamespace(toolkit.WorkspaceNamespace))

	// A namespace which is a workspace is one, by its name, until removed
	ns := notes{}
	require.NoError(t, tk.Add(ns))
	require.NoError(t, registry.With("notes", func(workspace.Workspace) error { return nil }))

	t.Run("List", func(t *testing.T) {
		result, err := tk.CallTool(ctx, "workspace:list", nil)
		require.NoError(t, err)
		assert.JSONEq(t, `[{"name":"notes","description":"My notes"}]`, string(result))
		result, err = tk.CallTool(ctx, "workspace:list", json.RawMessage(`{"workspace":"notes"}`))
		require.NoError(t, err)
		assert.JSONEq(t, `{"body":[{"uri":"resource://notes/file:///note.md","name":"note.md","type":"text/markdown"}]}`, string(result), "the workspace's URI")
	})

	t.Run("Read", func(t *testing.T) {
		result, err := tk.CallTool(ctx, "workspace:read", json.RawMessage(`{"uri":"file:///note.md"}`))
		require.NoError(t, err)
		assert.Contains(t, string(result), "text/markdown")
		_, err = tk.CallTool(ctx, "workspace:read", json.RawMessage(`{"uri":"file:///missing.md"}`))
		assert.ErrorIs(t, err, llm.ErrNotFound)

		// By the workspace's URI, as it's listed, without the workspace
		result, err = tk.CallTool(ctx, "workspace:read", json.RawMessage(`{"uri":"resource://notes/file:///note.md"}`))
		require.NoError(t, err)
		assert.Contains(t, string(result), "text/markdown")
		_, err = tk.CallTool(ctx, "workspace:read", json.RawMessage(`{"uri":"resource://other/file:///note.md"}`))
		assert.ErrorIs(t, err, llm.ErrNotFound, "another workspace")
	})

	t.Run("Resolve", func(t *testing.T) {
		// A tool reads a resource it's given from a workspace
		require.NoError(t, tk.Add(toolkit.NewNamespace("reading", "", reader{})))
		result, err := tk.CallTool(ctx, "reading:reader", json.RawMessage(`{"uri":"file:///note.md"}`))
		require.NoError(t, err)
		assert.JSONEq(t, `"# Note"`, string(result))
	})

	t.Run("ResolveError", func(t *testing.T) {
		// A resolver which fails, rather than not having a resource, says so
		failing := toolkit.WithResolver(ctx, func(_ context.Context, uris ...string) ([]llm.Resource, error) {
			if len(uris) == 1 && uris[0] == "session:///a" {
				return nil, llm.ErrServiceUnavailable.With("the store is down")
			}
			return nil, llm.ErrNotFound.With("not all of them")
		})
		resolve := toolkit.NewNamespace("both", "", both{})
		require.NoError(t, tk.Add(resolve))
		_, err := tk.CallTool(failing, "both:both", nil)
		assert.ErrorIs(t, err, llm.ErrServiceUnavailable)
	})

	// A namespace which is removed doesn't remove a workspace which replaced
	// it, by its name
	_, err = registry.Remove("notes")
	require.NoError(t, err)
	replacement := notes{}
	require.NoError(t, registry.Add("notes", &replacement))
	require.NoError(t, tk.Remove(ns))
	assert.NoError(t, registry.With("notes", func(workspace.Workspace) error { return nil }), "the replacement")
}

// both is a tool which reads a session's resource, and a workspace's
type both struct{ toolkit.Base }

func (both) Name() string                    { return "both" }
func (both) Description() string             { return "Read two resources" }
func (both) InputSchema() *jsonschema.Schema { return nil }
func (both) Run(ctx context.Context, _ json.RawMessage) (any, error) {
	_, err := toolkit.Resources(ctx, "session:///a", "file:///note.md")
	return nil, err
}
