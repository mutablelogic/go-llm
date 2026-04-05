package schema

import (
	"fmt"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	// Packages
	oidc "github.com/djthorpe/go-auth/pkg/oidc"
	pg "github.com/mutablelogic/go-pg"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// ConnectorCapability represents a named capability advertised by an MCP server.
type ConnectorCapability string

// ConnectorMeta holds the user-configurable settings for a registered MCP server.
type ConnectorMeta struct {
	// Enabled controls whether the server is active. Disabled servers are
	// retained in the registry but not connected on startup.
	// A nil value in a patch request means "preserve the existing value".
	Enabled *bool `json:"enabled,omitempty" negatable:"" jsonschema:"Whether the connector is enabled. Disabled connectors are retained but not connected on startup."`

	// Namespace is an optional prefix used to disambiguate tools from this
	// connector when multiple connectors expose tools with the same name.
	// A nil value in a patch request means "preserve the existing value".
	Namespace *string `json:"namespace,omitempty" jsonschema:"Unique connector namespace used to disambiguate tools when multiple connectors expose the same names."`

	// Groups holds auth group identifiers that are allowed to access this
	// connector.
	Groups []string `json:"groups,omitempty" jsonschema:"Auth groups that are allowed to access this connector. If empty, the connector is accessible to all authenticated users."`

	// Meta holds arbitrary user-managed connector metadata.
	// A nil value in a patch request means "preserve the existing value".
	// An empty object clears the metadata.
	Meta ProviderMetaMap `json:"meta,omitempty" jsonschema:"Arbitrary user-managed connector metadata as a JSON object."`
}

// ConnectorInsert contains the fields required to insert a new connector row.
type ConnectorInsert struct {
	URL string `json:"url" name:"url" help:"MCP server endpoint URL"`
	ConnectorMeta
}

// ConnectorURLSelector selects a connector by canonical URL.
type ConnectorURLSelector string

// ConnectorStateSelector selects a connector by canonical URL for runtime state updates.
type ConnectorStateSelector string

// ConnectorGroupList is a list of auth group identifiers associated with a connector.
type ConnectorGroupList []string

// ConnectorGroupRef represents a single connector-to-group link.
type ConnectorGroupRef struct {
	Connector string
	Group     string
}

// ConnectorGroupSelector selects connector_group rows for a connector,
// optionally scoped to a single group.
type ConnectorGroupSelector struct {
	Connector string
	Group     *string
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
	Capabilities []ConnectorCapability `json:"capabilities,omitempty"`
}

// Connector combines persisted metadata with runtime state for an MCP server.
type Connector struct {
	// Mutable fields
	ConnectorInsert

	// CreatedAt is the time the connector was first registered.
	CreatedAt time.Time `json:"created_at"`

	// ModifiedAt is the time the connector metadata was last updated.
	ModifiedAt *time.Time `json:"modified_at,omitempty"`

	// Current Connector State
	ConnectorState
}

// CreateConnectorUnauthorizedResponse is placed in the HTTP error detail when
// connector authorization is required before registration can complete.
type CreateConnectorUnauthorizedResponse struct {
	CodeFlow *oidc.BaseConfiguration `json:"codeflow,omitempty"`
	Scopes   []string                `json:"scopes,omitempty"`
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// ConnectorURLSchemes lists the URL schemes that are accepted for connector
// registration. Connectors must use one of these schemes.
var ConnectorURLSchemes = []string{"http", "https"}

const (
	ConnectorListMax uint64 = 100
)

// connectorDefaultPorts maps each supported scheme to its default port string.
// Ports matching the default are omitted from the canonical URL.
var connectorDefaultPorts = map[string]string{
	"http":  "80",
	"https": "443",
}

const (
	// CapabilityTools indicates the server advertises callable tools.
	CapabilityTools ConnectorCapability = "tools"

	// CapabilityResources indicates the server can serve resources.
	CapabilityResources ConnectorCapability = "resources"

	// CapabilityPrompts indicates the server provides reusable prompt templates.
	CapabilityPrompts ConnectorCapability = "prompts"

	// CapabilityLogging indicates the server emits log notifications.
	CapabilityLogging ConnectorCapability = "logging"

	// CapabilityCompletions indicates the server supports argument/prompt completion.
	CapabilityCompletions ConnectorCapability = "completions"
)

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (c ConnectorMeta) String() string {
	return types.Stringify(c)
}

func (c ConnectorInsert) String() string {
	return types.Stringify(c)
}

func (c ConnectorState) String() string {
	return types.Stringify(c)
}

func (c Connector) String() string {
	return types.Stringify(c)
}

func (c CreateConnectorUnauthorizedResponse) String() string {
	return types.Stringify(c)
}

///////////////////////////////////////////////////////////////////////////////
// QUERY

func (req ConnectorListRequest) Query() url.Values {
	values := url.Values{}
	if req.Offset > 0 {
		values.Set("offset", strconv.FormatUint(req.Offset, 10))
	}
	if req.Limit != nil {
		values.Set("limit", strconv.FormatUint(types.Value(req.Limit), 10))
	}
	if namespace := strings.TrimSpace(req.Namespace); namespace != "" {
		values.Set("namespace", namespace)
	}
	if req.Enabled != nil {
		values.Set("enabled", strconv.FormatBool(types.Value(req.Enabled)))
	}
	return values
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - READER

// Expected column order: url, namespace, enabled, name, title, description,
// meta, groups, created_at, modified_at, connected_at.
func (c *Connector) Scan(row pg.Row) error {
	if err := row.Scan(
		&c.URL, &c.Namespace, &c.Enabled, &c.Name, &c.Title, &c.Description,
		&c.Meta, &c.Groups, &c.CreatedAt, &c.ModifiedAt, &c.ConnectedAt,
	); err != nil {
		return err
	}
	if c.Meta == nil {
		c.Meta = make(ProviderMetaMap)
	}

	return nil
}

func (list *ConnectorGroupList) Scan(row pg.Row) error {
	var group string
	if err := row.Scan(&group); err != nil {
		return err
	}
	*list = append(*list, group)
	return nil
}

func (list *ConnectorList) Scan(row pg.Row) error {
	var connector Connector
	if err := connector.Scan(row); err != nil {
		return err
	}
	list.Body = append(list.Body, &connector)
	return nil
}

func (list *ConnectorList) ScanCount(row pg.Row) error {
	if err := row.Scan(&list.Count); err != nil {
		return err
	}
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// SELECTORS

func (c ConnectorURLSelector) Select(bind *pg.Bind, op pg.Op) (string, error) {
	url, err := CanonicalURL(string(c))
	if err != nil {
		return "", err
	}
	bind.Set("url", url)

	switch op {
	case pg.Get:
		if connectorHasUser(bind) {
			return bind.Query("connector.select_for_user"), nil
		}
		return bind.Query("connector.select"), nil
	case pg.Update:
		return bind.Query("connector.update"), nil
	case pg.Delete:
		return bind.Query("connector.delete"), nil
	default:
		return "", ErrInternalServerError.Withf("unsupported ConnectorURLSelector operation %q", op)
	}
}

func (c ConnectorStateSelector) Select(bind *pg.Bind, op pg.Op) (string, error) {
	url, err := CanonicalURL(string(c))
	if err != nil {
		return "", err
	}
	bind.Set("url", url)

	switch op {
	case pg.Update:
		return bind.Query("connector.update_state"), nil
	default:
		return "", ErrNotImplemented.Withf("unsupported ConnectorStateSelector operation %q", op)
	}
}

func (req ConnectorListRequest) Select(bind *pg.Bind, op pg.Op) (string, error) {
	bind.Del("where")

	if namespace := strings.TrimSpace(req.Namespace); namespace != "" {
		if !types.IsIdentifier(namespace) {
			return "", ErrBadParameter.Withf("connector namespace %q is not a valid identifier", namespace)
		}
		bind.Append("where", `connector.namespace = `+bind.Set("namespace", namespace))
	}
	if req.Enabled != nil {
		bind.Append("where", `connector.enabled = `+bind.Set("enabled", types.Value(req.Enabled)))
	}

	where := bind.Join("where", " AND ")
	bind.Set("orderby", `ORDER BY connector.created_at DESC, connector.url ASC`)
	req.OffsetLimit.Bind(bind, ConnectorListMax)

	switch op {
	case pg.List:
		if connectorHasUser(bind) {
			if where == "" {
				bind.Set("where", "")
			} else {
				bind.Set("where", "AND "+where)
			}
			return bind.Query("connector.list_for_user"), nil
		}
		if where == "" {
			bind.Set("where", "")
		} else {
			bind.Set("where", "WHERE "+where)
		}
		return bind.Query("connector.list"), nil
	default:
		return "", ErrNotImplemented.Withf("unsupported ConnectorListRequest operation %q", op)
	}
}

func connectorHasUser(bind *pg.Bind) bool {
	return bind.Get("user") != nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - WRITER

func (c ConnectorInsert) Insert(bind *pg.Bind) (string, error) {
	url, err := CanonicalURL(c.URL)
	if err != nil {
		return "", err
	}
	bind.Set("url", url)

	if namespace := strings.TrimSpace(types.Value(c.Namespace)); namespace == "" {
		return "", ErrBadParameter.With("connector namespace is required")
	} else if !types.IsIdentifier(namespace) {
		return "", ErrBadParameter.Withf("connector namespace %q is not a valid identifier", namespace)
	} else {
		bind.Set("namespace", namespace)
	}

	enabled := true
	if c.Enabled != nil {
		enabled = *c.Enabled
	}
	bind.Set("enabled", enabled)

	if c.Meta == nil {
		bind.Set("meta", make(ProviderMetaMap))
	} else {
		bind.Set("meta", c.Meta)
	}

	return bind.Query("connector.insert"), nil
}

func (c ConnectorInsert) Update(_ *pg.Bind) error {
	return fmt.Errorf("ConnectorInsert: update: not supported")
}

func (c ConnectorMeta) Update(bind *pg.Bind) error {
	bind.Del("patch")

	if c.Enabled != nil {
		bind.Append("patch", `enabled = `+bind.Set("enabled", types.Value(c.Enabled)))
	}
	if c.Namespace != nil {
		namespace := strings.TrimSpace(types.Value(c.Namespace))
		if namespace == "" {
			return ErrBadParameter.With("connector namespace is required")
		} else if !types.IsIdentifier(namespace) {
			return ErrBadParameter.Withf("connector namespace %q is not a valid identifier", namespace)
		} else {
			bind.Append("patch", `namespace = `+bind.Set("namespace", namespace))
		}
	}
	if c.Meta != nil {
		if expr, err := providerMetaPatch(bind, c.Meta); err != nil {
			return err
		} else if expr != "" {
			bind.Append("patch", `meta = `+expr)
		}
	}

	if bind.Join("patch", ", ") == "" {
		return ErrBadParameter.With("no fields to update")
	}
	bind.Set("patch", bind.Join("patch", ", "))
	return nil
}

func (c ConnectorMeta) Insert(_ *pg.Bind) (string, error) {
	return "", fmt.Errorf("ConnectorMeta: insert: not supported")
}

func (c ConnectorMeta) HasTableUpdates() bool {
	return c.Enabled != nil || c.Namespace != nil || c.Meta != nil
}

func (c ConnectorState) Update(bind *pg.Bind) error {
	bind.Del("patch")

	if c.ConnectedAt != nil {
		bind.Append("patch", `connected_at = `+bind.Set("connected_at", *c.ConnectedAt))
	}
	if c.Name != nil {
		bind.Append("patch", `name = `+bind.Set("name", strings.TrimSpace(*c.Name)))
	}
	if c.Title != nil {
		bind.Append("patch", `title = `+bind.Set("title", strings.TrimSpace(*c.Title)))
	}
	if c.Description != nil {
		bind.Append("patch", `description = `+bind.Set("description", strings.TrimSpace(*c.Description)))
	}

	if bind.Join("patch", ", ") == "" {
		return ErrBadParameter.With("no connector state fields to update")
	}
	bind.Set("patch", bind.Join("patch", ", "))
	return nil
}

func (c ConnectorState) Insert(_ *pg.Bind) (string, error) {
	return "", fmt.Errorf("ConnectorState: insert: not supported")
}

func (c ConnectorState) HasTableUpdates() bool {
	return c.ConnectedAt != nil || c.Name != nil || c.Title != nil || c.Description != nil
}

func (c ConnectorGroupRef) Insert(bind *pg.Bind) (string, error) {
	connector, err := CanonicalURL(c.Connector)
	if err != nil {
		return "", err
	}
	group, err := normalizeProviderGroup(c.Group)
	if err != nil {
		return "", err
	}
	bind.Set("connector", connector)
	bind.Set("group", group)
	return bind.Query("connector_group.insert"), nil
}

func (c ConnectorGroupRef) Update(_ *pg.Bind) error {
	return fmt.Errorf("ConnectorGroupRef: update: not supported")
}

func (c ConnectorGroupSelector) Select(bind *pg.Bind, op pg.Op) (string, error) {
	connector, err := CanonicalURL(c.Connector)
	if err != nil {
		return "", err
	}
	bind.Set("connector", connector)

	if c.Group != nil {
		group, err := normalizeProviderGroup(*c.Group)
		if err != nil {
			return "", err
		}
		bind.Set("group", group)
	}

	switch op {
	case pg.List:
		if c.Group != nil {
			return "", ErrNotImplemented.With("connector_group: list by connector and group is not supported")
		}
		return bind.Query("connector_group.list"), nil
	case pg.Delete:
		if c.Group != nil {
			return bind.Query("connector_group.delete"), nil
		}
		return bind.Query("connector_group.delete_all"), nil
	default:
		return "", ErrNotImplemented.Withf("unsupported ConnectorGroupSelector operation %q", op)
	}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - CANONICAL

// CanonicalURL validates rawURL and returns a canonical form consisting of
// scheme://host[:port]/path with a lowercased scheme, host, and path.
// The URL must be absolute, use a supported scheme (http or https), have a
// non-empty host, and — if a port is present — a valid port in 1–65535.
// Any userinfo, query string, or fragment is stripped.
func CanonicalURL(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", ErrBadParameter.Withf("connector url: %v", err)
	}
	if !u.IsAbs() {
		return "", ErrBadParameter.Withf("connector url: not an absolute URL: %q", rawURL)
	}

	u.Scheme = strings.ToLower(u.Scheme)
	if !slices.Contains(ConnectorURLSchemes, u.Scheme) {
		return "", ErrBadParameter.Withf("connector url: scheme %q not supported (want one of %v)", u.Scheme, ConnectorURLSchemes)
	}

	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "" {
		return "", ErrBadParameter.With("connector url: missing host")
	}

	if portStr := u.Port(); portStr != "" {
		port, err := strconv.ParseUint(portStr, 10, 16)
		if err != nil || port < 1 {
			return "", ErrBadParameter.Withf("connector url: invalid port %q (must be 1-65535)", portStr)
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
