package tool

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	mcpclient "github.com/mutablelogic/go-llm/pkg/mcp/client"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	errgroup "golang.org/x/sync/errgroup"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// connEntry holds the state for a single active connector.
type connEntry struct {
	connector llm.Connector
	cancel    context.CancelFunc
	wg        sync.WaitGroup // tracks this connector's runConnector goroutine
	connected bool           // true while the session is active
}

// connectorInfo is an optional interface a Connector may implement to expose
// server metadata from the live session without opening a second connection.
type connectorInfo interface {
	ServerInfo() (name, version, protocol string)
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// AddConnector registers a remote connector under the given URL and starts
// its Run loop in a background goroutine. The goroutine's context is cancelled
// by RemoveConnector or Close. Returns an error if a connector with the same
// URL is already registered.
func (tk *Toolkit) AddConnector(url string, c llm.Connector) error {
	tk.mu.Lock()
	defer tk.mu.Unlock()

	// Check for duplicate URL
	if _, exists := tk.conns[url]; exists {
		return llm.ErrBadParameter.Withf("connector already added: %q", url)
	}

	// Start the connector's background goroutine
	ctx, cancel := context.WithCancel(context.Background())
	entry := &connEntry{
		connector: c,
		cancel:    cancel,
	}
	tk.conns[url] = entry

	entry.wg.Add(1)
	tk.wg.Add(1)
	go tk.runConnector(ctx, url, entry)

	// Return success
	return nil
}

// RemoveConnector cancels the named connector's goroutine, waits for it to
// finish, then removes it from the registry. No-op if the URL is not registered.
func (tk *Toolkit) RemoveConnector(url string) {
	tk.mu.Lock()
	entry, ok := tk.conns[url]
	if ok {
		delete(tk.conns, url)
		entry.cancel()
	}
	tk.mu.Unlock()

	if ok {
		entry.wg.Wait()
	}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// runConnector is the per-connector background goroutine. It runs c.Run and
// a ListTools poll concurrently using an errgroup, retrying with exponential
// backoff on non-context errors or unexpected server disconnects. When the
// session is established the poll fires onState/onTools and exits; Run
// continues until the context is cancelled or the server disconnects. On
// final exit the deferred cleanup fires onState/onTools(nil) unless an auth
// error caused the exit, in which case the connector state is left intact so
// tools remain listed and callers receive per-call errors rather than a full
// disconnect.
func (tk *Toolkit) runConnector(ctx context.Context, url string, entry *connEntry) {
	c := entry.connector
	defer tk.wg.Done()
	defer entry.wg.Done()

	// suppressDisconnect prevents the final cleanup from broadcasting a
	// disconnect notification. Set to true when the session exits due to an
	// auth error so the connector stays "visible" in the tool list.
	suppressDisconnect := false

	// Final cleanup: clear tools and notify on exit.
	// Always broadcast disconnect on deliberate shutdown (ctx cancelled) even
	// if suppressDisconnect is set, so observers never see stale state.
	defer func() {
		tk.mu.Lock()
		entry.connected = false
		tk.mu.Unlock()
		if !suppressDisconnect || ctx.Err() != nil {
			zero := time.Time{}
			tk.onState(url, schema.ConnectorState{ConnectedAt: &zero})
			tk.onTools(url, nil)
		}
	}()

	const (
		minBackoff = time.Second
		maxBackoff = 5 * time.Minute
	)
	backoff := minBackoff

	for {
		// Task 1: drive the MCP session.
		eg, egCtx := errgroup.WithContext(ctx)
		eg.Go(func() error {
			if err := c.Run(egCtx); err != nil && !isContextError(err) {
				tk.onLog(url, slog.LevelError, "connector run error", "err", err)
				return err
			}
			return nil
		})

		// Task 2: poll until ListTools succeeds (session is up), then fire callbacks.
		eg.Go(func() error {
			const pollInterval = 100 * time.Millisecond
			timer := time.NewTimer(0)
			defer timer.Stop()
			for {
				select {
				case <-egCtx.Done():
					return egCtx.Err()
				case <-timer.C:
					tools, err := c.ListTools(egCtx)
					if err != nil {
						timer.Reset(pollInterval)
						continue
					}
					// Session is up — mark connected and fire callbacks.
					tk.mu.Lock()
					entry.connected = true
					tk.mu.Unlock()

					now := time.Now()
					state := schema.ConnectorState{ConnectedAt: &now}
					if info, ok := c.(connectorInfo); ok {
						name, version, _ := info.ServerInfo()
						state.Name = ptrString(name)
						state.Version = ptrString(version)
					}
					tk.onState(url, state)
					tk.onTools(url, tools)
					return nil
				}
			}
		})

		err := eg.Wait()

		// Deliberate shutdown — let the deferred cleanup fire the final notifications.
		if ctx.Err() != nil {
			return
		}

		if err != nil && !isContextError(err) {
			tk.onLog(url, slog.LevelError, "connector exited with error", "err", err)
		}

		tk.mu.Lock()
		wasConnected := entry.connected
		tk.mu.Unlock()

		switch {
		case isAuthError(err) && !wasConnected:
			// Auth failure before the session was ever established — bad
			// credentials. Don't retry; notify disconnect and stop.
			tk.onLog(url, slog.LevelError, "connector auth failure, not retrying", "err", err)
			return

		case isAuthError(err) && wasConnected:
			// The SDK killed the session because one tools/call HTTP response
			// returned 403. The tool call error already reached the LLM; the
			// tool list has not changed. Keep entry.tools so tools remain
			// visible during the reconnect window. Reconnect silently — no
			// disconnect notification.
			tk.onLog(url, slog.LevelWarn, "connector session dropped by auth error, reconnecting", "err", err)
			suppressDisconnect = true
			backoff = minBackoff
			// fall through to the backoff sleep and retry

		default:
			// Real disconnect (server closed, network error, etc.)
			tk.mu.Lock()
			entry.connected = false
			tk.mu.Unlock()
			suppressDisconnect = false
			if wasConnected {
				zero := time.Time{}
				tk.onState(url, schema.ConnectorState{ConnectedAt: &zero})
				tk.onTools(url, nil)
				backoff = minBackoff // successful session — reset backoff
			}
			tk.onLog(url, slog.LevelInfo, "connector disconnected, retrying", "backoff", backoff)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, maxBackoff)
	}
}

func isContextError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func isAuthError(err error) bool {
	return mcpclient.IsAuthError(err)
}

func ptrString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
