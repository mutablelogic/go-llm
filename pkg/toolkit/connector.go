package toolkit

import llm "github.com/mutablelogic/go-llm"

// AddConnector registers a remote MCP server. The namespace is inferred from
// the server (e.g. the hostname or last path segment of the URL). Safe to call
// before or while Run is active; the connector starts immediately if Run is
// already running.
func (tk *toolkit) AddConnector(string) error {
	return llm.ErrNotImplemented
}

// AddConnectorNS registers a remote MCP server under an explicit namespace.
// Safe to call before or while Run is active; the connector starts immediately
// if Run is already running.
func (tk *toolkit) AddConnectorNS(namespace, url string) error {
	return llm.ErrNotImplemented
}

// RemoveConnector removes a connector by URL. Safe to call before or
// while Run is active; the connector is stopped immediately if running.
func (tk *toolkit) RemoveConnector(string) error {
	return llm.ErrNotImplemented
}
