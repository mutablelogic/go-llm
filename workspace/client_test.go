package workspace_test

import (
	"context"
	"testing"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	llm "github.com/mutablelogic/go-llm"
	client "github.com/mutablelogic/go-llm/mcp/client"
	test "github.com/mutablelogic/go-llm/mcp/test"
	workspace "github.com/mutablelogic/go-llm/workspace"
	schema "github.com/mutablelogic/go-llm/workspace/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_Client(t *testing.T) {
	// The resources of an MCP server which isn't a workspace are listed, as
	// it ignores the folder, and read
	ctx := t.Context()
	server := test.NewServer(t)
	server.MCP.AddResource(&sdkmcp.Resource{URI: "file:///notes/a.md", Name: "a.md", MIMEType: "text/markdown"}, func(_ context.Context, req *sdkmcp.ReadResourceRequest) (*sdkmcp.ReadResourceResult, error) {
		return &sdkmcp.ReadResourceResult{Contents: []*sdkmcp.ResourceContents{{URI: req.Params.URI, MIMEType: "text/markdown", Text: "# A"}}}, nil
	})
	c, err := client.New(server.URL)
	require.NoError(t, err)
	require.NoError(t, c.Connect(ctx))
	t.Cleanup(func() { assert.NoError(t, c.Disconnect()) })

	assert.False(t, c.HasExperimental(schema.Capability), "it isn't a workspace")
	ws, err := workspace.NewClient("notes", c)
	require.NoError(t, err)
	assert.Equal(t, "notes", ws.Name())
	list, err := ws.List(ctx, schema.ListRequest{Path: "other"})
	require.NoError(t, err)
	require.Len(t, list.Body, 1)
	assert.Equal(t, "file:///notes/a.md", list.Body[0].URI)
	assert.Empty(t, list.Next)

	resource, err := ws.Read(ctx, "file:///notes/a.md")
	require.NoError(t, err)
	assert.Equal(t, "a.md", resource.Name())
	assert.Equal(t, "text/markdown", resource.Type())
	data, err := llm.ReadAll(ctx, resource)
	require.NoError(t, err)
	assert.Equal(t, "# A", string(data))

	_, err = workspace.NewClient("not a name", c)
	assert.ErrorIs(t, err, llm.ErrBadParameter)
}
