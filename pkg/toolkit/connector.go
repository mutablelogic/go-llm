package toolkit

import (
	"context"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

///////////////////////////////////////////////////////////////////////////////
// CONSTANTS

var connectorURLSchemes = []string{"http", "https"}

var connectorDefaultPorts = map[string]string{
	"http":  "80",
	"https": "443",
}

///////////////////////////////////////////////////////////////////////////////
// TYPES

type connector struct {
	ctx       context.Context
	cancel    context.CancelFunc
	namespace string
	conn      llm.Connector
	wg        sync.WaitGroup
	err       error
}

var _ llm.Connector = (*connector)(nil)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - TOOLKIT

// AddConnector registers a remote MCP server. The namespace is inferred from
// the server URL. Safe to call before or while Run is active.
func (tk *toolkit) AddConnector(url string) error {
	return tk.addConnector("", url)
}

// AddConnectorNS registers a remote MCP server under an explicit namespace.
// Safe to call before or while Run is active.
func (tk *toolkit) AddConnectorNS(namespace, url string) error {
	return tk.addConnector(namespace, url)
}

// RemoveConnector removes a connector by URL. The connector is stopped
// immediately if it is currently running.
func (tk *toolkit) RemoveConnector(url string) error {
	key, err := canonicalURL(url)
	if err != nil {
		return err
	}
	tk.mu.Lock()
	conn, exists := tk.connectors[key]
	if !exists {
		tk.mu.Unlock()
		return llm.ErrNotFound.Withf("connector not found: %q", url)
	}
	delete(tk.connectors, key)
	// Capture and clear cancel under the lock so we don't race with the
	// goroutine that also writes conn.cancel under the lock.
	cancelFn := conn.cancel
	conn.cancel = nil
	tk.mu.Unlock()

	// Stop the connector's goroutine and wait for it to finish.
	if cancelFn != nil {
		cancelFn()
	}
	conn.wg.Wait()
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - CONNECTOR (delegates to inner conn)

func (c *connector) Run(ctx context.Context) error {
	return c.conn.Run(ctx)
}

func (c *connector) ListTools(ctx context.Context) ([]llm.Tool, error) {
	return c.conn.ListTools(ctx)
}

func (c *connector) ListPrompts(ctx context.Context) ([]llm.Prompt, error) {
	return c.conn.ListPrompts(ctx)
}

func (c *connector) ListResources(ctx context.Context) ([]llm.Resource, error) {
	return c.conn.ListResources(ctx)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// addConnector is the shared implementation for AddConnector and AddConnectorNS.
// Must NOT be called while tk.mu is held.
func (tk *toolkit) addConnector(namespace, url string) error {
	key, err := canonicalURL(url)
	if err != nil {
		return err
	}
	if tk.handler == nil {
		return llm.ErrNotImplemented.With("toolkit handler is not set")
	}

	tk.mu.Lock()
	defer tk.mu.Unlock()

	// Initialise the map lazily (handles the case where New didn't set it).
	if tk.connectors == nil {
		tk.connectors = make(map[string]*connector)
	}

	if _, exists := tk.connectors[key]; exists {
		return llm.ErrConflict.Withf("connector already added: %q", key)
	}

	// Build the connector entry first so the onState callback can capture it.
	c := &connector{
		namespace: namespace,
	}

	// The onState callback is invoked by the connector after a successful
	// handshake. If the reported Name (and no explicit namespace was given)
	// we use it as the connector's namespace, then register it in the map.
	onState := func(state schema.ConnectorState) {
		if state.Name == nil {
			return
		}
		tk.mu.Lock()
		if c.namespace == "" {
			c.namespace = *state.Name
		}
		tk.namespace[c.namespace] = c
		tk.mu.Unlock()
	}

	// Create the connector via the handler, passing the onState callback.
	conn, err := tk.handler.CreateConnector(key, onState)
	if err != nil {
		return err
	}
	if conn == nil {
		return llm.ErrInternalServerError.Withf("handler returned nil connector for %q", key)
	}

	c.conn = conn
	tk.connectors[key] = c
	return nil
}

// canonicalURL normalises a connector URL to scheme://host[:port]/path with
// lowercased scheme, host, and path. Userinfo, query string, and fragment are
// stripped. Returns ErrBadParameter if the URL is invalid.
func canonicalURL(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", llm.ErrBadParameter.Withf("connector url: %v", err)
	}
	if !u.IsAbs() {
		return "", llm.ErrBadParameter.Withf("connector url: not an absolute URL: %q", rawURL)
	}

	u.Scheme = strings.ToLower(u.Scheme)
	if !slices.Contains(connectorURLSchemes, u.Scheme) {
		return "", llm.ErrBadParameter.Withf("connector url: scheme %q not supported (want one of %v)", u.Scheme, connectorURLSchemes)
	}

	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "" {
		return "", llm.ErrBadParameter.Withf("connector url: missing host")
	}

	if portStr := u.Port(); portStr != "" {
		port, err := strconv.ParseUint(portStr, 10, 16)
		if err != nil || port < 1 {
			return "", llm.ErrBadParameter.Withf("connector url: invalid port %q (must be 1-65535)", portStr)
		}
		if connectorDefaultPorts[u.Scheme] != portStr {
			host = host + ":" + portStr
		}
	}

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
