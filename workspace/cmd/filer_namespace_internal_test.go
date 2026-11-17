//go:build !client

package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	// Packages
	gofiler "github.com/mutablelogic/go-filer"
	filerfs "github.com/mutablelogic/go-filer/filer/fs"
	llm "github.com/mutablelogic/go-llm"
	namespace "github.com/mutablelogic/go-llm/mcp/namespace"
	workspace "github.com/mutablelogic/go-llm/workspace"
	filer "github.com/mutablelogic/go-llm/workspace/filer"
	schema "github.com/mutablelogic/go-llm/workspace/schema"
	eventschema "github.com/mutablelogic/go-server/broadcaster/schema"
	require "github.com/stretchr/testify/require"
)

func Test_FilerNamespace(t *testing.T) {
	// A file which is edited is said to be, without subscribing to it, as
	// the namespace's changes are its backend's
	ctx := t.Context()
	dir := t.TempDir()
	file := filepath.Join(dir, "a.md")
	require.NoError(t, os.WriteFile(file, []byte("# A"), 0o600))
	backend, err := filerfs.New(FilerName, dir, nil, gofiler.Opts{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = backend.Close() })
	srv, err := filer.NewServer(FilerName, FilerTitle, backend)
	require.NoError(t, err)
	ns, err := namespace.New(FilerName, "files")
	require.NoError(t, err)
	var files workspace.Workspace = &filerNamespace{Namespace: ns, workspace: srv.Workspace()}

	watcher, ok := files.(workspace.Watcher)
	require.True(t, ok)
	changes := make(chan schema.Change, 16)
	watchCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- watcher.WatchResources(watchCtx, changes) }()
	select {
	case <-changes:
	case err := <-done:
		require.ErrorIs(t, err, llm.ErrNotImplemented)
		t.Skip("the backend doesn't watch here:", err)
	case <-time.After(10 * time.Second):
		t.Fatal("the watch didn't start")
	}

	require.NoError(t, os.WriteFile(file, []byte("# A, edited"), 0o600))
	deadline := time.After(10 * time.Second)
	for {
		select {
		case change := <-changes:
			// An edit is an update, or, as macOS's FSEvents may say, the
			// file was created
			if change.URI == "file:///a.md" && (change.Action == eventschema.ChangeUpdate || change.Action == eventschema.ChangeCreate) {
				return
			}
		case <-deadline:
			t.Fatal("not told the file was edited")
		}
	}
}
