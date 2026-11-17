package schema

import (
	"encoding/json"
	"time"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// Capability is the experimental capability of an MCP server which is a
	// workspace, which lists its resources by folder, as the _meta of
	// resources/list says
	Capability = "workspace"

	// MetaPrefix is the prefix of the keys of the _meta of a workspace's
	// requests and resources
	MetaPrefix = "go-llm.workspace/"

	// The keys of the _meta of resources/list: the folder which is listed,
	// whether everything under it is, and the most resources which are
	// returned, which a server which isn't a workspace ignores
	MetaPath      = MetaPrefix + "path"
	MetaRecursive = MetaPrefix + "recursive"
	MetaLimit     = MetaPrefix + "limit"

	// The keys of the _meta of a resource: its metadata, and artwork
	MetaMeta    = MetaPrefix + "meta"
	MetaArtwork = MetaPrefix + "artwork"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// MCP returns the request as the params of resources/list, with the folder,
// whether it's recursive, and the limit, in its _meta
func (r ListRequest) MCP() *sdkmcp.ListResourcesParams {
	params := &sdkmcp.ListResourcesParams{Cursor: r.Cursor}
	meta := sdkmcp.Meta{}
	if r.Path != "" {
		meta[MetaPath] = r.Path
	}
	if r.Recursive {
		meta[MetaRecursive] = true
	}
	if r.Limit > 0 {
		meta[MetaLimit] = r.Limit
	}
	if len(meta) > 0 {
		params.Meta = meta
	}
	return params
}

// MCP returns the resource as MCP's, with its modified time as its
// annotations' lastModified, and its metadata, and artwork, in its _meta
func (r ResourceMeta) MCP() *sdkmcp.Resource {
	resource := &sdkmcp.Resource{
		URI:         r.URI,
		Name:        r.Name,
		Title:       r.Title,
		Description: r.Description,
		MIMEType:    r.Type,
		Size:        r.Size,
	}
	if !r.Modified.IsZero() {
		resource.Annotations = &sdkmcp.Annotations{LastModified: r.Modified.UTC().Format(time.RFC3339)}
	}
	meta := sdkmcp.Meta{}
	if len(r.Meta) > 0 {
		meta[MetaMeta] = r.Meta
	}
	if len(r.Artwork) > 0 {
		meta[MetaArtwork] = r.Artwork
	}
	if len(meta) > 0 {
		resource.Meta = meta
	}
	return resource
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC FUNCTIONS

// ListRequestOf returns the params of resources/list as a request, with the
// folder, whether it's recursive, and the limit, from its _meta, which are
// left out when they aren't there, or aren't valid
func ListRequestOf(params *sdkmcp.ListResourcesParams) ListRequest {
	var req ListRequest
	if params == nil {
		return req
	}
	req.Cursor = params.Cursor
	decode(params.Meta[MetaPath], &req.Path)
	decode(params.Meta[MetaRecursive], &req.Recursive)
	decode(params.Meta[MetaLimit], &req.Limit)
	return req
}

// ResourceMetaOf returns MCP's resource as a workspace's, with its modified
// time, metadata, and artwork, when it has them, as a workspace's do, which
// are left out when they aren't valid
func ResourceMetaOf(resource *sdkmcp.Resource) ResourceMeta {
	if resource == nil {
		return ResourceMeta{}
	}
	r := ResourceMeta{
		URI:         resource.URI,
		Name:        resource.Name,
		Title:       resource.Title,
		Description: resource.Description,
		Type:        resource.MIMEType,
		Size:        resource.Size,
	}
	if resource.Annotations != nil && resource.Annotations.LastModified != "" {
		if modified, err := time.Parse(time.RFC3339, resource.Annotations.LastModified); err == nil {
			r.Modified = modified
		}
	}
	decode(resource.Meta[MetaMeta], &r.Meta)
	decode(resource.Meta[MetaArtwork], &r.Artwork)
	return r
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// decode decodes the value of a _meta, which is as it was decoded from
// JSON, or as it was set, into the value, or leaves it as it is, when it
// can't be
func decode(from any, to any) {
	if from == nil {
		return
	}
	data, err := json.Marshal(from)
	if err != nil {
		return
	}
	_ = json.Unmarshal(data, to)
}
