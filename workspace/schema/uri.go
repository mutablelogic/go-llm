package schema

import (
	"strings"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// URIScheme is the scheme of a workspace's resources' URIs, as a registry
// has them: resource://<workspace>/<uri>, whose host is the workspace's
// name, and whose path is the resource's own URI, as the workspace has it,
// or its path under the workspace's root, which a resource on its own,
// resource:///<hash>, which has no host, isn't
const URIScheme = "resource"

///////////////////////////////////////////////////////////////////////////////
// PUBLIC FUNCTIONS

// URIOf returns the URI of the workspace's resource, with its own URI, as a
// registry has it: resource://<workspace>/<path>, without the workspace's
// root, when it's under it, such as resource://files/notes/a.md, of
// file:///notes/a.md, with the root file:///, or otherwise with all of its
// own URI, such as resource://github/ui://github/get-me
func URIOf(workspace, root, uri string) string {
	if path, ok := strings.CutPrefix(uri, root); ok && root != "" && path != "" && !strings.Contains(path, "://") {
		uri = path
	}
	return URIScheme + "://" + workspace + "/" + uri
}

// ParseURI returns the workspace, and the rest of a URI which URIOf
// returns, which is the resource's path, under the workspace's root, or its
// own URI, which OriginalURI returns the resource's own URI of, and true,
// or false, when it isn't one, such as a resource's own URI, or a resource
// on its own, resource:///<hash>
func ParseURI(uri string) (workspace, rest string, ok bool) {
	after, ok := strings.CutPrefix(uri, URIScheme+"://")
	if !ok {
		return "", "", false
	}
	workspace, rest, ok = strings.Cut(after, "/")
	if !ok || workspace == "" || rest == "" {
		return "", "", false
	}
	return workspace, rest, true
}

// OriginalURI returns the resource's own URI, of the rest of a URI which
// ParseURI returns: its root, and the rest, when it's a path, or the rest,
// when it's a URI, which has ://, such as artwork's, or the workspace has
// no root
func OriginalURI(root, rest string) string {
	if root == "" || strings.Contains(rest, "://") {
		return rest
	}
	return root + rest
}

// RootOf returns the root of the resources of an MCP server, of its resource
// templates: the prefix of its one template which is a prefix, and one
// variable, such as file:///{+path}, whose prefix is file:///, or nothing,
// when it has none, or more than one
func RootOf(templates []*sdkmcp.ResourceTemplate) string {
	var root string
	for _, t := range templates {
		if t == nil {
			continue
		}
		if prefix, ok := rootTemplate(t.URITemplate); ok {
			if root != "" {
				return ""
			}
			root = prefix
		}
	}
	return root
}

// RootTemplate returns the resource template of the root, such as
// file:///{+path}, of file:///
func RootTemplate(root string) string {
	return root + "{+path}"
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// rootTemplate returns the prefix of the template, and true, when it's a
// prefix, which ends with a slash, and one variable, at its end, such as
// {+path}, or {path}
func rootTemplate(template string) (string, bool) {
	prefix, variable, ok := strings.Cut(template, "{")
	if !ok || prefix == "" || !strings.HasSuffix(prefix, "/") || !strings.HasSuffix(variable, "}") {
		return "", false
	}
	name := strings.TrimPrefix(strings.TrimSuffix(variable, "}"), "+")
	if name == "" || strings.ContainsAny(name, "{},*:") {
		return "", false
	}
	return prefix, true
}
