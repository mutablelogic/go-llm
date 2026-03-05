package tool

import (
	"context"
	"log/slog"
	"sync"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
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
	tools     []llm.Tool     // current tool list; nil when disconnected
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
// a ListTools poll concurrently using an errgroup. When the session is
// established the poll fires onState/onTools and exits; Run continues until
// the context is cancelled or the server disconnects. On exit the deferred
// cleanup fires onTools(url, nil).
func (tk *Toolkit) runConnector(ctx context.Context, url string, entry *connEntry) {
	c := entry.connector
	defer tk.wg.Done()
	defer entry.wg.Done()

	// Cleanup: clear tools and notify on exit.
	defer func() {
		tk.mu.Lock()
		entry.tools = nil
		tk.mu.Unlock()
		zero := time.Time{}
		tk.onState(url, schema.ConnectorState{ConnectedAt: &zero})
		tk.onTools(url, nil)
	}()

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
				// Session is up — store tools and fire callbacks.
				tk.mu.Lock()
				entry.tools = tools
				tk.mu.Unlock()

				if info, ok := c.(connectorInfo); ok {
					name, version, _ := info.ServerInfo()
					if name != "" || version != "" {
						now := time.Now()
						tk.onState(url, schema.ConnectorState{
							ConnectedAt: &now,
							Name:        ptrString(name),
							Version:     ptrString(version),
						})
					}
				}
				tk.onTools(url, tools)
				return nil
			}
		}
	})

	if err := eg.Wait(); err != nil && !isContextError(err) {
		tk.onLog(url, slog.LevelError, "connector exited with error", "err", err)
	}
}

func isContextError(err error) bool {
	return err == context.Canceled || err == context.DeadlineExceeded
}

func ptrString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
