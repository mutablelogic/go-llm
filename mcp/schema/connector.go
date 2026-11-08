package schema

import (
	"net/url"
	"regexp"
	"slices"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	tui "github.com/mutablelogic/go-server/pkg/tui"
	types "github.com/mutablelogic/go-server/pkg/types"
	oauth2 "golang.org/x/oauth2"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// credentialName is the pattern of a credential's name, such as
// GITHUB_CLIENT_ID, as an environment variable's
var credentialName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// ConnectorMeta are the fields of a connector which can be set
type ConnectorMeta struct {
	URL     string   `json:"url" help:"The URL of the connector's MCP server, of its streamable HTTP, or SSE, transport" example:"https://mcp.example.com/mcp"`
	Include []string `json:"include,omitempty" help:"Regular expressions of the names of its server's tools, prompts and resources which are included, or every one when there are none" example:"[\"^search_\"]" optional:""`
	Exclude []string `json:"exclude,omitempty" help:"Regular expressions of the names of its server's tools, prompts and resources which are excluded" example:"[\"signup\"]" optional:""`

	// The names of the credentials of the client its server is authorized
	// as, when its authorization server doesn't register clients, such as
	// GitHub's, which the credentials provider has, so the client's
	// identifier, and secret, themselves, aren't kept
	ClientIDCredential     string `json:"client_id_credential,omitempty" help:"Name of the credential holding the identifier of the client its server is authorized as, when its authorization server doesn't register clients" example:"GITHUB_CLIENT_ID" optional:""`
	ClientSecretCredential string `json:"client_secret_credential,omitempty" help:"Name of the credential holding the client's secret, when it has one" example:"GITHUB_CLIENT_SECRET" optional:""`
}

// Connector is a remote MCP server, as it's connected to: its name, its
// fields which can be set, and its serial, which changes each time its
// definition does, which are what's needed to create a client of its
// server, and what a store of connectors holds, and its status: whether
// its server is connected, and which server it is, when it is, or why it
// couldn't be connected, or its session ended, which isn't stored, as it's
// the process's
type Connector struct {
	Name string `json:"name" help:"Connector name, which is an identifier, starting with a letter, followed by letters, numbers, underscores or hyphens" example:"github"`
	ConnectorMeta
	Serial uint64 `json:"serial,omitempty" readonly:"" help:"Revision of the connector's definition, which changes each time it's changed" example:"1"`

	// The status, which isn't stored
	Connected bool             `json:"connected,omitempty" readonly:"" help:"Whether the connector's server is connected"`
	Server    *ConnectorServer `json:"server,omitempty" readonly:"" help:"The connector's server, as it said, once it's connected"`
	Error     string           `json:"error,omitempty" readonly:"" help:"Why the connector's server couldn't be connected, or its session ended"`
	Namespace string           `json:"namespace,omitempty" readonly:"" help:"The namespace of its server's tools, in the toolkit, which has none while it isn't connected" example:"github"`
	Auth      string           `json:"auth,omitempty" readonly:"" help:"Whether its server needs to be authorized, which is required, until it's signed in to, or is authorized, once it is" enum:"required,authorized" example:"required"`
}

// ConnectorServer is the name, and version, of a connector's server, as it
// said, once it's connected
type ConnectorServer struct {
	Name    string `json:"name" help:"The server's name" example:"github-mcp-server"`
	Version string `json:"version,omitempty" help:"The server's version" example:"1.0.0"`
}

// The values of a connector's Auth: its server needs to be authorized, or
// is
const (
	AuthRequired   = "required"
	AuthAuthorized = "authorized"
)

// Credential is how a connector's server is authorized, which is stored,
// but isn't returned: the URL of the server it's for, its authorization
// server's endpoints, the client it registered, or the name of the
// credential of the connector's own client, whose identifier, and secret,
// aren't kept, the resource, and scopes, and the token, once it's signed in
// to, which is refreshed, as it expires
type Credential struct {
	URL                   string        `json:"url"`
	ClientCredential      string        `json:"client_credential,omitempty"`
	Issuer                string        `json:"issuer"`
	AuthorizationEndpoint string        `json:"authorization_endpoint"`
	TokenEndpoint         string        `json:"token_endpoint"`
	ClientID              string        `json:"client_id"`
	ClientSecret          string        `json:"client_secret,omitempty"`
	RedirectURIs          []string      `json:"redirect_uris,omitempty"`
	Resource              string        `json:"resource,omitempty"`
	Scopes                []string      `json:"scopes,omitempty"`
	Token                 *oauth2.Token `json:"token,omitempty"`
}

// ConnectorEvent is the data of an event of a connector: its serial, and,
// when its server was connected, which server it is, as it said, or, when
// it was disconnected, why, when it wasn't disconnected, such as when the
// server closed the session
type ConnectorEvent struct {
	Serial uint64           `json:"serial" help:"The connector's serial"`
	Server *ConnectorServer `json:"server,omitempty" help:"The connector's server, as it said, once it's connected"`
	Error  string           `json:"error,omitempty" help:"Why the connector's server was disconnected, when it wasn't disconnected"`
	Auth   string           `json:"auth,omitempty" help:"That its server needs to be authorized, which is required, as it couldn't be connected"`
}

// ConnectorList is a list of connectors, in order of their names
type ConnectorList struct {
	Count uint         `json:"count" help:"The number of connectors"`
	Body  []*Connector `json:"body,omitempty" help:"The connectors"`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Definition returns the connector without its status, which is what's
// stored
func (c Connector) Definition() Connector {
	c.Connected, c.Server, c.Error, c.Namespace, c.Auth = false, nil, "", "", ""
	return c
}

// Validate returns ErrBadParameter when the connector's name isn't an
// identifier, or its fields aren't valid
func (c Connector) Validate() error {
	if !types.IsIdentifier(c.Name) {
		return llm.ErrBadParameter.Withf("connector name %q isn't an identifier", c.Name)
	}
	return c.ConnectorMeta.Validate()
}

// Validate returns ErrBadParameter when the URL isn't an http, or https,
// URL with a host, as an MCP server which runs as a command, over stdio,
// can't be connected to
func (m ConnectorMeta) Validate() error {
	u, err := url.Parse(strings.TrimSpace(m.URL))
	if err != nil {
		return llm.ErrBadParameter.Withf("url %q: %v", m.URL, err)
	} else if u.Scheme != "http" && u.Scheme != "https" {
		return llm.ErrBadParameter.Withf("url %q isn't http, or https", m.URL)
	} else if u.Hostname() == "" {
		return llm.ErrBadParameter.Withf("url %q has no host", m.URL)
	}
	for _, name := range []string{m.ClientIDCredential, m.ClientSecretCredential} {
		if name != "" && !credentialName.MatchString(name) {
			return llm.ErrBadParameter.Withf("credential name %q isn't a name, such as GITHUB_CLIENT_ID", name)
		}
	}
	if m.ClientSecretCredential != "" && m.ClientIDCredential == "" {
		return llm.ErrBadParameter.With("a client's secret needs its identifier")
	}
	_, err = m.Matcher()
	return err
}

// Matcher returns the function which says whether the name of one of the
// server's tools, prompts or resources is included: it matches one of the
// include patterns, or there are none, and none of the exclude patterns.
// Returns ErrBadParameter when a pattern isn't a regular expression.
func (m ConnectorMeta) Matcher() (func(string) bool, error) {
	include, err := compile("include", m.Include)
	if err != nil {
		return nil, err
	}
	exclude, err := compile("exclude", m.Exclude)
	if err != nil {
		return nil, err
	}
	return func(name string) bool {
		if len(include) > 0 && !slices.ContainsFunc(include, func(re *regexp.Regexp) bool { return re.MatchString(name) }) {
			return false
		}
		return !slices.ContainsFunc(exclude, func(re *regexp.Regexp) bool { return re.MatchString(name) })
	}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// compile returns the patterns, which are regular expressions, compiled, or
// ErrBadParameter when one isn't valid
func compile(field string, patterns []string) ([]*regexp.Regexp, error) {
	result := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, llm.ErrBadParameter.Withf("%s pattern %q: %v", field, pattern, err)
		}
		result = append(result, re)
	}
	return result, nil
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (c Connector) String() string {
	return types.Stringify(c)
}

func (m ConnectorMeta) String() string {
	return types.Stringify(m)
}

func (l ConnectorList) String() string {
	return types.Stringify(l)
}

///////////////////////////////////////////////////////////////////////////////
// TABLE

func (c *Connector) Header() []string {
	return []string{"Name", "Status", "Server", "Namespace", "URL"}
}

func (c *Connector) Width(col int) int {
	return 0
}

func (c *Connector) Cell(col int) tui.CellText {
	switch col {
	case 0:
		return c.Name
	case 1:
		// A connector whose server isn't connected shows why, when it knows
		switch {
		case c.Connected:
			return "connected"
		case c.Error != "":
			// Its first line, as an error can have one for each transport
			line, _, _ := strings.Cut(c.Error, "\n")
			return line
		default:
			return "not connected"
		}
	case 2:
		if c.Server == nil {
			return ""
		} else if c.Server.Version != "" {
			return c.Server.Name + " " + c.Server.Version
		}
		return c.Server.Name
	case 3:
		return c.Namespace
	case 4:
		return c.URL
	default:
		return ""
	}
}
