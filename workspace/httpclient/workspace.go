package httpclient

import (
	"context"

	// Packages
	client "github.com/mutablelogic/go-client"
	schema "github.com/mutablelogic/go-llm/workspace/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ListWorkspaces returns the workspaces which are available, in order of
// their names, with whether each reads its resources, and is watched
func (c *Client) ListWorkspaces(ctx context.Context) (*schema.WorkspaceList, error) {
	var response schema.WorkspaceList
	if err := c.DoWithContext(ctx, client.MethodGet, &response, client.OptPath("llm", "workspace")); err != nil {
		return nil, err
	}
	return types.Ptr(response), nil
}

// GetWorkspace returns the workspace with the name
func (c *Client) GetWorkspace(ctx context.Context, name string) (*schema.Workspace, error) {
	var response schema.Workspace
	if err := c.DoWithContext(ctx, client.MethodGet, &response, client.OptPath("llm", "workspace", name)); err != nil {
		return nil, err
	}
	return types.Ptr(response), nil
}

// ListResources returns a page of the folders, and resources, of the
// workspace with the name, in the request's folder, or under it, from its
// cursor, whose URIs are the workspace's, resource://<workspace>/<uri>
func (c *Client) ListResources(ctx context.Context, name string, req schema.ListRequest) (*schema.ListResponse, error) {
	var response schema.ListResponse
	if err := c.DoWithContext(ctx, client.MethodGet, &response, client.OptPath("llm", "workspace", name, "resource"), client.OptQuery(req.Query())); err != nil {
		return nil, err
	}
	return types.Ptr(response), nil
}
