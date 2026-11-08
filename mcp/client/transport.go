package client

import (
	"net/http"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// httpClient returns the HTTP client of the MCP SDK's requests, which is a
// copy of go-client's, with its transports, such as its user agent, headers,
// token, and tracing, but without its timeout, as the streams of an MCP
// session last as long as it does, and are cancelled with its context
func (c *Client) httpClient() *http.Client {
	client := *c.Client.Client
	client.Timeout = 0
	return &client
}

// transports returns the MCP SDK's transports of the server, in the order
// they're tried, as it's connected: streamable HTTP, of the 2025-03-26
// protocol, and then SSE, of the 2024-11-05 protocol, for a server which
// hasn't it, which share an HTTP client
func (c *Client) transports() []sdkmcp.Transport {
	client := c.httpClient()
	return []sdkmcp.Transport{
		&sdkmcp.StreamableClientTransport{Endpoint: c.endpoint, HTTPClient: client},
		&sdkmcp.SSEClientTransport{Endpoint: c.endpoint, HTTPClient: client},
	}
}
