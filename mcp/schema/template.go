package schema

import (
	"strings"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// ConnectorTemplate is a remote MCP server which a connector can be added
// for, without its URL being typed, such as GitHub's: its name, and URL,
// and the names of the credentials of the client it's signed in to with,
// when its sign-in doesn't register clients itself, which the server has,
// so it's offered
type ConnectorTemplate struct {
	Name                   string `json:"name" yaml:"name" help:"The server's name, such as GitHub" example:"GitHub"`
	Description            string `json:"description,omitempty" yaml:"description" help:"What the server does" optional:""`
	URL                    string `json:"url" yaml:"url" help:"The URL of the server, of its streamable HTTP, or SSE, transport" example:"https://api.githubcopilot.com/mcp/"`
	ClientIDCredential     string `json:"client_id_credential,omitempty" yaml:"client_id_credential" help:"Name of the credential holding the identifier of the client the server is signed in to with" example:"GITHUB_CLIENT_ID" optional:""`
	ClientSecretCredential string `json:"client_secret_credential,omitempty" yaml:"client_secret_credential" help:"Name of the credential holding the client's secret" example:"GITHUB_CLIENT_SECRET" optional:""`
}

// ConnectorTemplateList is the templates of the connectors which can be
// added, and how many there are
type ConnectorTemplateList struct {
	Count uint                 `json:"count" help:"The number of templates"`
	Body  []*ConnectorTemplate `json:"body,omitempty" help:"The templates, in order of their names"`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Meta returns the fields of a connector of the template
func (t ConnectorTemplate) Meta() ConnectorMeta {
	return ConnectorMeta{URL: strings.TrimSpace(t.URL), ClientIDCredential: t.ClientIDCredential, ClientSecretCredential: t.ClientSecretCredential}
}
