//go:build !client

package cmd

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	// Packages
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	gofiler "github.com/mutablelogic/go-filer"
	filerfs "github.com/mutablelogic/go-filer/filer/fs"
	namespace "github.com/mutablelogic/go-llm/mcp/namespace"
	filer "github.com/mutablelogic/go-llm/workspace/filer"
	schema "github.com/mutablelogic/go-llm/workspace/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// closer is a transport whose connections are closed, as a server which
// closes its session does
type closer struct {
	sdkmcp.Transport
	mu    *sync.Mutex
	conns *[]sdkmcp.Connection
}

func (c closer) Connect(ctx context.Context) (sdkmcp.Connection, error) {
	conn, err := c.Transport.Connect(ctx)
	if err == nil {
		c.mu.Lock()
		*c.conns = append(*c.conns, conn)
		c.mu.Unlock()
	}
	return conn, err
}

func Test_ConnectFiler(t *testing.T) {
	// The namespace is connected again, once the server closes its session
	reconnectDelay = 10 * time.Millisecond
	ctx := t.Context()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.md"), []byte("# A"), 0o644))
	backend, err := filerfs.New(FilerName, dir, nil, gofiler.Opts{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = backend.Close() })
	srv, err := filer.NewServer(FilerName, FilerTitle, backend)
	require.NoError(t, err)

	var mu sync.Mutex
	var conns []sdkmcp.Connection
	transport := func(ctx context.Context) (sdkmcp.Transport, error) {
		t, err := srv.InMemoryTransport(ctx)
		return closer{Transport: t, mu: &mu, conns: &conns}, err
	}
	ns, err := namespace.New(FilerName, "files")
	require.NoError(t, err)
	disconnect, err := connectFiler(ctx, slog.New(slog.DiscardHandler), ns, "files", transport)
	require.NoError(t, err)
	t.Cleanup(disconnect)
	require.True(t, ns.Available())

	// Close the session, as the server does
	mu.Lock()
	require.Len(t, conns, 1)
	require.NoError(t, conns[0].Close())
	mu.Unlock()

	// It's connected again, with another connection
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(conns) == 2 && ns.Available()
	}, 5*time.Second, 10*time.Millisecond)
	list, err := ns.List(ctx, schema.ListRequest{})
	require.NoError(t, err)
	require.Len(t, list.Body, 1)
	assert.Equal(t, "file:///a.md", list.Body[0].URI)
}
