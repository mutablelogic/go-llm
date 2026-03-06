package schema

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	// Packages
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// CanonicalURL validates rawURL and returns a canonical form consisting of
// scheme://host[:port]/path with a lowercased scheme, host, and path.
// The URL must be absolute, use a supported scheme (http or https), have a
// non-empty host, and — if a port is present — a valid port in 1–65535.
// Any userinfo, query string, or fragment is stripped.
func CanonicalURL(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("connector url: %w", err)
	}
	if !u.IsAbs() {
		return "", fmt.Errorf("connector url: not an absolute URL: %q", rawURL)
	}

	u.Scheme = strings.ToLower(u.Scheme)
	if !slices.Contains(ConnectorURLSchemes, u.Scheme) {
		return "", fmt.Errorf("connector url: scheme %q not supported (want one of %v)", u.Scheme, ConnectorURLSchemes)
	}

	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "" {
		return "", fmt.Errorf("connector url: missing host")
	}

	if portStr := u.Port(); portStr != "" {
		port, err := strconv.ParseUint(portStr, 10, 16)
		if err != nil || port < 1 {
			return "", fmt.Errorf("connector url: invalid port %q (must be 1-65535)", portStr)
		}
		// Only include the port if it is non-default for the scheme.
		if connectorDefaultPorts[u.Scheme] != portStr {
			host = host + ":" + portStr
		}
	}

	// Clean the decoded path; path.Clean("") returns "." so normalise that back.
	// Preserve a trailing slash if the original path had one.
	hadTrailingSlash := len(u.Path) > 1 && strings.HasSuffix(u.Path, "/")
	cleanPath := strings.ToLower(path.Clean(u.Path))
	if cleanPath == "." {
		cleanPath = ""
	}
	if hadTrailingSlash {
		cleanPath += "/"
	}
	u.Path = cleanPath
	u.RawPath = ""

	return u.Scheme + "://" + host + u.EscapedPath(), nil
}

// CanonicalNamespace normalises a namespace string by lowercasing it,
// removing spaces and replacing "-" and "." characters with underscores.
func CanonicalNamespace(ns string) string {
	return strings.ToLower(strings.NewReplacer(
		" ", "",
		"-", "_",
		".", "_",
	).Replace(ns))
}

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Capability represents a named capability advertised by an MCP server.
type Capability string

// ConnectorMeta holds the user-configurable settings for a registered MCP server.
type ConnectorMeta struct {
	// Enabled controls whether the server is active. Disabled servers are
	// retained in the registry but not connected on startup.
	// A nil value in a patch request means "preserve the existing value".
	Enabled *bool `json:"enabled,omitempty"`

	// Namespace is an optional prefix used to disambiguate tools from this
	// connector when multiple connectors expose tools with the same name.
	// A nil value in a patch request means "preserve the existing value".
	Namespace *string `json:"namespace,omitempty"`
}

// ConnectorState carries the server-reported runtime state of a connector.
// All fields are pointers so callers can update a subset without overwriting
// fields they did not observe.
type ConnectorState struct {
	// ConnectedAt is the time of the most recent successful connection.
	ConnectedAt *time.Time `json:"connected_at,omitempty"`

	// Name is the programmatic identifier reported by the server.
	Name *string `json:"name,omitempty"`

	// Title is a human-readable display name reported by the server.
	Title *string `json:"title,omitempty"`

	// Description is the server-reported description.
	Description *string `json:"description,omitempty"`

	// Version is the server-reported version string.
	Version *string `json:"version,omitempty"`

	// Capabilities is the set of capabilities declared at initialization.
	Capabilities []Capability `json:"capabilities,omitempty"`
}

// Connector combines persisted metadata with runtime state for an MCP server.
type Connector struct {
	// URL is the MCP server endpoint (used as the primary key).
	URL string `json:"url"`

	// CreatedAt is the time the connector was first registered.
	CreatedAt time.Time `json:"created_at"`

	// Embedded metadata and state fields
	ConnectorMeta
	ConnectorState
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (c ConnectorMeta) String() string  { return types.Stringify(c) }
func (c ConnectorState) String() string { return types.Stringify(c) }
func (c Connector) String() string      { return types.Stringify(c) }

///////////////////////////////////////////////////////////////////////////////
// CONSTANTS

// ConnectorURLSchemes lists the URL schemes that are accepted for connector
// registration. Connectors must use one of these schemes.
var ConnectorURLSchemes = []string{"http", "https"}

// connectorDefaultPorts maps each supported scheme to its default port string.
// Ports matching the default are omitted from the canonical URL.
var connectorDefaultPorts = map[string]string{
	"http":  "80",
	"https": "443",
}

const (
	// CapabilityTools indicates the server advertises callable tools.
	CapabilityTools Capability = "tools"

	// CapabilityResources indicates the server can serve resources.
	CapabilityResources Capability = "resources"

	// CapabilityPrompts indicates the server provides reusable prompt templates.
	CapabilityPrompts Capability = "prompts"

	// CapabilityLogging indicates the server emits log notifications.
	CapabilityLogging Capability = "logging"

	// CapabilityCompletions indicates the server supports argument/prompt completion.
	CapabilityCompletions Capability = "completions"
)

///////////////////////////////////////////////////////////////////////////////
// INTERFACES

// ConnectorStore is the interface for persisting MCP connector registrations.
type ConnectorStore interface {
	// CreateConnector registers a new MCP server. url is used as the primary
	// key. Returns an error if a connector with that URL already exists.
	CreateConnector(ctx context.Context, url string, meta ConnectorMeta) (*Connector, error)

	// GetConnector returns the connector for the given URL, or an error if not found.
	GetConnector(ctx context.Context, url string) (*Connector, error)

	// UpdateConnector applies meta to the connector identified by url.
	// Only user-editable fields (ConnectorMeta) are updated; server-reported
	// fields are ignored. Returns an error if the connector does not exist.
	UpdateConnector(ctx context.Context, url string, meta ConnectorMeta) (*Connector, error)

	// DeleteConnector removes the connector for the given URL.
	DeleteConnector(ctx context.Context, url string) error

	// ListConnectors returns connectors matching the request filters.
	// Supports pagination via Offset/Limit and optional filtering by Namespace and Enabled.
	ListConnectors(ctx context.Context, req ListConnectorsRequest) (*ListConnectorsResponse, error)

	// UpdateConnectorState merges state into the connector identified by url,
	// updating only the non-nil fields in state. Intended to be called after
	// a successful connection to record server-reported information.
	// Returns an error if the connector does not exist.
	UpdateConnectorState(ctx context.Context, url string, state ConnectorState) (*Connector, error)
}
