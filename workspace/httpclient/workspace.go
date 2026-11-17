package httpclient

import (
	"context"
	"io"
	"net/http"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
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

// ReadResource writes the content of the resource of the workspace with the
// name, by the request's URI, as it is, or as a Markdown document, to the
// writer, as it's read, and returns its type. The download isn't timed out,
// as a large file can take a while.
func (c *Client) ReadResource(ctx context.Context, name string, req schema.ReadRequest, w io.Writer) (string, error) {
	if w == nil {
		return "", llm.ErrBadParameter.With("missing writer")
	}
	out := &download{w: w}
	if err := c.DoWithContext(ctx, client.NewRequestEx(http.MethodGet, types.ContentTypeAny), out, client.OptPath("llm", "workspace", name, "content"), client.OptQuery(req.Query()), client.OptNoTimeout()); err != nil {
		return "", err
	}
	return out.contentType, nil
}

///////////////////////////////////////////////////////////////////////////////
// DOWNLOAD

// download writes a response's content to its writer, and keeps its type
type download struct {
	w           io.Writer
	contentType string
}

var _ client.Unmarshaler = (*download)(nil)

// Unmarshal writes the content, and keeps its type
func (d *download) Unmarshal(header http.Header, r io.Reader) error {
	d.contentType = header.Get(types.ContentTypeHeader)
	_, err := io.Copy(d.w, r)
	return err
}
