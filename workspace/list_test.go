package workspace_test

import (
	"context"
	"fmt"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	workspace "github.com/mutablelogic/go-llm/workspace"
	schema "github.com/mutablelogic/go-llm/workspace/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// unlimited is a workspace which ignores the limit, as an MCP server which
// isn't a workspace's does: its pages are of 4, whose cursor is the page
type unlimited struct{}

func (unlimited) List(_ context.Context, req schema.ListRequest) (*schema.ListResponse, error) {
	page := 0
	if req.Cursor != "" {
		fmt.Sscanf(req.Cursor, "page%d", &page)
	}
	result := &schema.ListResponse{}
	for i := range 4 {
		n := page*4 + i
		result.Body = append(result.Body, schema.ResourceMeta{URI: fmt.Sprintf("ui://server/%d", n), Name: fmt.Sprint(n)})
	}
	if page < 2 {
		result.Next = fmt.Sprintf("page%d", page+1)
	}
	return result, nil
}

func Test_ListLimit(t *testing.T) {
	// The limit is kept, whether the workspace keeps it, or not, a page at a
	// time, which continues where the last ended, with an opaque cursor,
	// and each has its path
	ctx := context.Background()
	r, err := workspace.New()
	require.NoError(t, err)
	require.NoError(t, r.Add("server", unlimited{}))
	require.NoError(t, r.Add("other", unlimited{}))

	var names []string
	req := schema.ListRequest{Limit: 3}
	for {
		list, err := r.ListResources(ctx, "server", req)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(list.Body), 3)
		for _, resource := range list.Body {
			names = append(names, resource.Name)
			assert.Equal(t, "ui://server/"+resource.Name, resource.Path, "its URI, after the workspace")
		}
		if list.Next == "" {
			break
		}
		assert.NotContains(t, list.Next, "page", "an opaque cursor")
		req.Cursor = list.Next
	}
	assert.Equal(t, []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11"}, names, "each once, in order")

	// Without a limit, a page is the workspace's
	list, err := r.ListResources(ctx, "server", schema.ListRequest{})
	require.NoError(t, err)
	assert.Len(t, list.Body, 4)

	// A cursor which isn't the registry's, or is another workspace's, isn't
	// valid
	_, err = r.ListResources(ctx, "server", schema.ListRequest{Cursor: "page1"})
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	first, err := r.ListResources(ctx, "server", schema.ListRequest{Limit: 2})
	require.NoError(t, err)
	_, err = r.ListResources(ctx, "other", schema.ListRequest{Cursor: first.Next})
	assert.ErrorIs(t, err, llm.ErrBadParameter)
}
