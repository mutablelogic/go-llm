package schema

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	// Packages
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// CanonicalURL validates rawURL and returns a canonical form consisting of
// scheme://host[:port]/path with a lowercased scheme and host.
// The URL must be absolute, use a supported scheme (http or https), have a
// non-empty host, and — if a port is present — a valid port in 1–65535.
// Any userinfo, query string, or fragment is stripped.
func CanonicalURL(rawURL string) (string, error) {
	u, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return "", fmt.Errorf("connector url: %w", err)
	}

	u.Scheme = strings.ToLower(u.Scheme)
	validScheme := false
	for _, s := range ConnectorURLSchemes {
		if u.Scheme == s {
			validScheme = true
			break
		}
	}
	if !validScheme {
		return "", fmt.Errorf("connector url: scheme %q not supported (want one of %v)", u.Scheme, ConnectorURLSchemes)
	}

	host := strings.ToLower(u.Hostname())
	if host == "" {
		return "", fmt.Errorf("connector url: missing host")
	}

	if portStr := u.Port(); portStr != "" {
		port, err := strconv.Atoi(portStr)
		if err != nil || port < 1 || port > 65535 {
			return "", fmt.Errorf("connector url: invalid port %q (must be 1-65535)", portStr)
		}
		host = host + ":" + portStr
	}

	return u.Scheme + "://" + host + u.EscapedPath(), nil
}

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Capability represents a named capability advertised by an MCP server.
type Capability string

// ConnectorMeta holds the user-configurable settings for a registered MCP server.
type ConnectorMeta struct {
	// Enabled controls whether the server is active. Disabled servers are
	// retained in the registry but not connected on startup.
	Enabled bool `json:"enabled"`

	// Namespace is an optional prefix used to disambiguate tools from this
	// connector when multiple connectors expose tools with the same name.
	Namespace string `json:"namespace,omitempty"`
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
