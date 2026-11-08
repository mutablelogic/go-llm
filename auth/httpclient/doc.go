// Package httpclient is an HTTP client, with go-client, of a resource which
// may need to be authorized, such as an MCP server's: a request which isn't
// authorized returns an AuthError, with the server's challenge, and the
// resource's metadata, and that of its authorization servers, is
// discovered, which says how a token for it is got.
package httpclient
