package schema

import (
	"regexp"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// iconName is the pattern of a template's icon, which is the file name of
// an SVG, or PNG, rather than a path, or URL
var iconName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*\.(svg|png)$`)

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
	Icon                   string `json:"icon,omitempty" yaml:"icon" help:"The file name of the server's logo, among the web UI's connector icons, at icon/connector/" example:"github.svg" optional:""`
}

// ConnectorTemplateList is the templates of the connectors which can be
// added, and how many there are
type ConnectorTemplateList struct {
	Count uint                 `json:"count" help:"The number of templates"`
	Body  []*ConnectorTemplate `json:"body,omitempty" help:"The templates, in order of their names"`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Validate returns ErrBadParameter when the template has no name, as it's
// listed by it, or its URL, or the names of its credentials, aren't valid
func (t ConnectorTemplate) Validate() error {
	if strings.TrimSpace(t.Name) == "" {
		return llm.ErrBadParameter.Withf("template of %q has no name", t.URL)
	} else if err := t.Meta().Validate(); err != nil {
		return llm.ErrBadParameter.Withf("template %q: %v", t.Name, err)
	} else if t.Icon != "" && !iconName.MatchString(t.Icon) {
		return llm.ErrBadParameter.Withf("template %q: icon %q isn't the file name of an SVG, or PNG", t.Name, t.Icon)
	}
	return nil
}

// Meta returns the fields of a connector of the template
func (t ConnectorTemplate) Meta() ConnectorMeta {
	return ConnectorMeta{URL: strings.TrimSpace(t.URL), ClientIDCredential: t.ClientIDCredential, ClientSecretCredential: t.ClientSecretCredential}
}
