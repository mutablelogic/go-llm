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
	MetaType      = MetaPrefix + "type"

	// MetaCount is the key of the _meta of the result of resources/list,
	// which is how many there are, of every page, when the workspace knows
	MetaCount = MetaPrefix + "count"

	// CapabilityTree is the key of the workspace capability, whose value is
	// true when its folders are listed on their own, by their type, and
	// have their paths, so they're shown as a tree
	CapabilityTree = "tree"

	// The keys of the _meta of a resource: its metadata, and artwork
	MetaMeta    = MetaPrefix + "meta"
	MetaArtwork = MetaPrefix + "artwork"

	// The keys of the _meta of resources/read: the byte of the content to
	// read from, and the most bytes to read, so a large resource is read in
	// chunks, and, of its result, the offset of the chunk, which says it's
	// one, and the size of the whole content, when it's known, which a
	// server which isn't a workspace ignores, and leaves out
	MetaOffset = MetaPrefix + "offset"
	MetaLength = MetaPrefix + "length"
	MetaSize   = MetaPrefix + "size"
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
	if r.Type != "" {
		meta[MetaType] = r.Type
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
		resource.Annotations = &sdkmcp.Annotations{LastModified: r.Modified.UTC().Format(time.RFC3339Nano)}
	}
	meta := sdkmcp.Meta{}
	if len(r.Meta) > 0 {
		meta[MetaMeta] = r.Meta
	}
	if len(r.Artwork) > 0 {
		meta[MetaArtwork] = r.Artwork
	}
	if r.Path != "" {
		meta[MetaPath] = r.Path
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
	decode(params.Meta[MetaType], &req.Type)
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
		if modified, err := time.Parse(time.RFC3339Nano, resource.Annotations.LastModified); err == nil {
			r.Modified = modified
		}
	}
	decode(resource.Meta[MetaMeta], &r.Meta)
	decode(resource.Meta[MetaArtwork], &r.Artwork)
	decode(resource.Meta[MetaPath], &r.Path)
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

// ReadParams returns the params of resources/read of the resource with the
// URI, of the chunk of its content from the offset, of at most the length,
// in its _meta
func ReadParams(uri string, offset, length int64) *sdkmcp.ReadResourceParams {
	return &sdkmcp.ReadResourceParams{URI: uri, Meta: sdkmcp.Meta{MetaOffset: offset, MetaLength: length}}
}

// ReadRangeOf returns the offset, and length, of the chunk the params of
// resources/read ask for, and true, or false, when they ask for the whole
// content, as a client which isn't a workspace's does
func ReadRangeOf(params *sdkmcp.ReadResourceParams) (offset, length int64, ok bool) {
	if params == nil || params.Meta[MetaLength] == nil {
		return 0, 0, false
	}
	decode(params.Meta[MetaOffset], &offset)
	decode(params.Meta[MetaLength], &length)
	if offset < 0 || length <= 0 {
		return 0, 0, false
	}
	return offset, length, true
}

// ChunkOf returns true when the result is a chunk of the resource's
// content, as a workspace's server returns, which may be all of it, or
// false, when it's all of it, as a server which isn't a workspace returns
func ChunkOf(result *sdkmcp.ReadResourceResult) bool {
	return result != nil && result.Meta[MetaOffset] != nil
}

// CountOf returns how many resources there are, of every page, which the
// result of resources/list says, or nil, when it doesn't
func CountOf(result *sdkmcp.ListResourcesResult) *uint64 {
	if result == nil || result.Meta[MetaCount] == nil {
		return nil
	}
	data, err := json.Marshal(result.Meta[MetaCount])
	if err != nil {
		return nil
	}
	var count uint64
	if err := json.Unmarshal(data, &count); err != nil {
		// It isn't a count, such as -1, 1.5, or "unknown"
		return nil
	}
	return &count
}

// SizeOf returns the size of the whole content of the resource, which a
// chunk of it is, and true, or false, when the result doesn't say, as a
// server which isn't a workspace's doesn't
func SizeOf(result *sdkmcp.ReadResourceResult) (int64, bool) {
	if result == nil || result.Meta[MetaSize] == nil {
		return 0, false
	}
	var size int64 = -1
	decode(result.Meta[MetaSize], &size)
	return size, size >= 0
}
