package toolkit

import (
	"context"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// CONSTANTS

var connectorURLSchemes = []string{"http", "https"}

var connectorDefaultPorts = map[string]string{
	"http":  "80",
	"https": "443",
}

const (
	// connectorRetryInitial is the delay before the first reconnect attempt.
	connectorRetryInitial = 2 * time.Second

	// connectorRetryMax is the ceiling for the exponential backoff delay.
	connectorRetryMax = 20 * time.Minute

	// connectorRetryMaxCount is the maximum number of reconnect attempts before
	// the connector is permanently removed.
	connectorRetryMaxCount = 100
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type connector struct {
	// Underlying connector implementation provided by the handler.
	namespace string
	conn      llm.Connector

	// Managing the connector's goroutine and lifecycle.
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Last error encountered by the connector
	err error

	// Exponential backoff state
	retryCount int
	retryDelay time.Duration
	retryAt    time.Time
}

var _ llm.Connector = (*connector)(nil)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - TOOLKIT

// AddConnector registers a remote MCP server. The namespace is inferred from
// the server URL. Safe to call before or while Run is active.
//
// As a special case, passing [UserConnectorURI] ("connector:user") registers
// the connector under the reserved "user" namespace without a remote URL.
func (tk *toolkit) AddConnector(rawURL string) error {
	if rawURL == UserConnectorURI {
		return tk.addConnector(UserNamespace, rawURL)
	}
	return tk.addConnector("", rawURL)
}

// AddConnectorNS registers a remote MCP server under an explicit namespace.
// Safe to call before or while Run is active.
func (tk *toolkit) AddConnectorNS(namespace, url string) error {
	if !types.IsIdentifier(namespace) {
		return llm.ErrBadParameter.Withf("connector namespace %q is not a valid identifier", namespace)
	}
	if slices.Contains(ReservedNamespaces, namespace) {
		return llm.ErrBadParameter.Withf("connector namespace %q is reserved", namespace)
	}
	if url == UserConnectorURI {
		return llm.ErrBadParameter.Withf("connector url: %q is reserved; use AddConnector instead", UserConnectorURI)
	}
	return tk.addConnector(namespace, url)
}

// RemoveConnector removes a connector by URL. The connector is stopped
// immediately if it is currently running.
func (tk *toolkit) RemoveConnector(rawURL string) error {
	var key string
	if rawURL == UserConnectorURI {
		key = rawURL
	} else {
		var err error
		key, err = canonicalURL(rawURL)
		if err != nil {
			return err
		}
	}
	tk.mu.Lock()
	conn, exists := tk.connectors[key]
	if !exists {
		tk.mu.Unlock()
		return llm.ErrNotFound.Withf("connector not found: %q", rawURL)
	}
	delete(tk.connectors, key)
	// Remove the namespace entry if it still points at this connector.
	if conn.namespace != "" && tk.namespace[conn.namespace] == conn {
		delete(tk.namespace, conn.namespace)
	}
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
// PRIVATE METHODS - CONNECTOR

// reset zeroes the error, retry count, and backoff state. Must be called with tk.mu held.
func (c *connector) reset() {
	c.err = nil
	c.retryCount = 0
	c.retryDelay = 0
	c.retryAt = time.Time{}
}

// retry records err, increments the failure count, and advances the exponential
// backoff delay. The first failure reconnects immediately; backoff only kicks in
// from the second failure onwards. Returns false when the retry ceiling has been
// reached, indicating the connector should be permanently removed.
// Must be called with tk.mu held.
func (c *connector) retry(err error) bool {
	c.err = err
	c.retryCount++
	if c.retryCount >= connectorRetryMaxCount {
		return false
	}
	// First failure: reconnect on the next tick with no delay.
	if c.retryCount == 1 {
		return true
	}
	if c.retryDelay == 0 {
		c.retryDelay = connectorRetryInitial
	} else {
		c.retryDelay = min(c.retryDelay*2, connectorRetryMax)
	}
	c.retryAt = time.Now().Add(c.retryDelay)
	return true
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

// onConnectorEvent is invoked by a connector's onEvent callback. State-change
// events are handled internally (namespace registration, backoff reset, logging)
// and are not forwarded to the delegate; all other event kinds are forwarded
// to the delegate.
func (tk *toolkit) onConnectorEvent(c *connector, evt ConnectorEvent) {
	switch evt.Kind {
	case ConnectorEventStateChange:
		state := evt.State
		// If no namespace is pre-set and the connector didn't report a name,
		// we have nothing to register.
		if state.Name == nil && c.namespace == "" {
			return
		}
		tk.mu.Lock()
		if c.namespace == "" {
			// Reject invalid identifiers and reserved namespaces.
			// TODO: mutate the namespace to make it valid (e.g. by replacing invalid characters with "_") rather than rejecting it outright.
			ns := types.Value(state.Name)
			if !types.IsIdentifier(ns) || slices.Contains(ReservedNamespaces, ns) {
				c.err = llm.ErrConflict.Withf("connector reported reserved or invalid namespace %q", ns)
				tk.mu.Unlock()
				return
			}
			// Reject collision with a namespace already owned by a different connector.
			if existing, collision := tk.namespace[ns]; collision && existing != c {
				c.err = llm.ErrConflict.Withf("connector namespace %q already in use", ns)
				tk.mu.Unlock()
				return
			}
			c.namespace = ns
		}
		tk.namespace[c.namespace] = c

		// Successful handshake — reset backoff so the next reconnect is immediate.
		tk.logger.InfoContext(context.Background(), "connector connected", "namespace", c.namespace, "name", types.Value(state.Name), "version", types.Value(state.Version))
		c.reset()
		tk.mu.Unlock()
	default:
		if handler := tk.delegate; handler != nil {
			evt.Connector = c
			handler.OnEvent(evt)
		}
	}
}

// addConnector is the shared implementation for AddConnector and AddConnectorNS.
// Must NOT be called while tk.mu is held.
func (tk *toolkit) addConnector(namespace, url string) error {
	// UserConnectorURI is used as its own canonical key; skip URL normalisation.
	var key string
	if url == UserConnectorURI {
		key = url
	} else {
		var err error
		key, err = canonicalURL(url)
		if err != nil {
			return err
		}
	}
	if tk.delegate == nil {
		return llm.ErrNotImplemented.With("toolkit delegate is not set")
	}

	// Validate and reserve the slot under the lock, but do not hold the lock
	// while calling CreateConnector — the handler or its internal callbacks
	// may call back into the toolkit and deadlock.
	tk.mu.Lock()
	if tk.connectors == nil {
		tk.connectors = make(map[string]*connector)
	}
	if _, exists := tk.connectors[key]; exists {
		tk.mu.Unlock()
		return llm.ErrConflict.Withf("connector already added: %q", key)
	}
	// Reserve the slot so a concurrent call for the same key is rejected.
	c := &connector{
		namespace: namespace,
	}
	tk.connectors[key] = c
	tk.mu.Unlock()

	// Create the connector outside the lock.
	conn, err := tk.delegate.CreateConnector(key, func(evt ConnectorEvent) {
		tk.onConnectorEvent(c, evt)
	})
	if err != nil {
		tk.mu.Lock()
		delete(tk.connectors, key)
		tk.mu.Unlock()
		return err
	}
	if conn == nil {
		tk.mu.Lock()
		delete(tk.connectors, key)
		tk.mu.Unlock()
		return llm.ErrInternalServerError.Withf("handler returned nil connector for %q", key)
	}

	// Finalize under the lock. If RemoveConnector ran concurrently while
	// CreateConnector was in flight the reserved slot will already be gone;
	// treat that as a conflict rather than silently re-inserting.
	tk.mu.Lock()
	if _, still := tk.connectors[key]; !still {
		tk.mu.Unlock()
		return llm.ErrConflict.Withf("connector removed while being added: %q", key)
	}
	c.conn = conn
	tk.mu.Unlock()
	return nil
}

// canonicalURL normalises a connector URL to scheme://host[:port]/path with
// lowercased scheme and host. Path case is preserved because HTTP path
// semantics are commonly case-sensitive. Redundant dot-segments are cleaned,
// but a trailing slash (if present) is retained. Userinfo, query string, and
// fragment are stripped. Returns ErrBadParameter if the URL is invalid.
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
	cleanPath := path.Clean(u.Path) // preserve case; only remove dot-segments
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
