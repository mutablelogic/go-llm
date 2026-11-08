// Package client is a client of a remote MCP server, over HTTP, with
// go-client as its transport, so its options, such as a user agent,
// headers, a token, and tracing, apply to the requests of the MCP SDK.
//
// A server is reached at its URL, with the streamable HTTP transport of
// the 2025-03-26 protocol, or the SSE transport of the 2024-11-05
// protocol, as the server has. A server which runs as a command, over
// stdio, isn't one.
package client
