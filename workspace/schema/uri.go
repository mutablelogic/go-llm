package schema

import (
	"strings"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// URIScheme is the scheme of a workspace's resources' URIs, as a registry
// has them: resource://<workspace>/<uri>, whose host is the workspace's
// name, and whose path is the resource's own URI, as the workspace has it,
// which a resource on its own, resource:///<hash>, which has no host, isn't
const URIScheme = "resource"

///////////////////////////////////////////////////////////////////////////////
// PUBLIC FUNCTIONS

// URIOf returns the URI of the workspace's resource, with its own URI, as a
// registry has it, such as resource://github/ui://github/get-me
func URIOf(workspace, uri string) string {
	return URIScheme + "://" + workspace + "/" + uri
}

// ParseURI returns the workspace, and the resource's own URI, of a URI
// which URIOf returns, and true, or false, when it isn't one, such as a
// resource's own URI, or a resource on its own, resource:///<hash>
func ParseURI(uri string) (workspace, original string, ok bool) {
	rest, ok := strings.CutPrefix(uri, URIScheme+"://")
	if !ok {
		return "", "", false
	}
	workspace, original, ok = strings.Cut(rest, "/")
	if !ok || workspace == "" || original == "" {
		return "", "", false
	}
	return workspace, original, true
}
