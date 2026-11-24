package toolkit_test

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	// Packages
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// runner is a namespace which runs until it's cancelled, or fails, when it's
// told to
type runner struct {
	toolkit.Namespace
	logger  *slog.Logger
	runs    atomic.Int32
	stopped atomic.Bool
	fail    chan error
}

func newRunner(name string) *runner {
	return &runner{Namespace: toolkit.NewNamespace(name, ""), fail: make(chan error, 1)}
}

func (r *runner) Run(ctx context.Context, logger *slog.Logger) error {
	r.logger = logger
	r.runs.Add(1)
	defer r.stopped.Store(true)
	select {
	case err := <-r.fail:
		return err
	case <-ctx.Done():
		return nil
	}
}

func Test_Run(t *testing.T) {
	tk, err := toolkit.New(t.Context())
	require.NoError(t, err)
	defer tk.Close()
	a, b, removed, later := newRunner("a"), newRunner("b"), newRunner("removed"), newRunner("later")
	for _, ns := range []toolkit.Namespace{a, toolkit.NewNamespace("plain", ""), b, removed} {
		require.NoError(t, tk.Add(ns))
	}
	require.NoError(t, tk.Remove(removed))

	// The namespaces which run are run, once each, with the logger, until
	// it's cancelled, and the toolkit can be changed as they are
	run := func() (context.CancelFunc, chan error, *slog.Logger) {
		ctx, cancel := context.WithCancel(context.Background())
		done, logger := make(chan error, 1), slog.New(slog.DiscardHandler)
		go func() { done <- tk.Run(ctx, logger) }()
		return cancel, done, logger
	}
	cancel, done, logger := run()
	require.Eventually(t, func() bool { return a.runs.Load() == 1 && b.runs.Load() == 1 }, time.Second, time.Millisecond)
	require.NoError(t, tk.Add(later))
	cancel()
	require.NoError(t, <-done)
	assert.Same(t, logger, a.logger)
	assert.True(t, a.stopped.Load() && b.stopped.Load())
	assert.Zero(t, removed.runs.Load())
	assert.Zero(t, later.runs.Load())

	// One which fails stops the others, and its error is returned
	failed := errors.New("failed")
	b.stopped.Store(false)
	cancel, done, _ = run()
	defer cancel()
	require.Eventually(t, func() bool { return a.runs.Load() == 2 && b.runs.Load() == 2 && later.runs.Load() == 1 }, time.Second, time.Millisecond)
	a.fail <- failed
	assert.ErrorIs(t, <-done, failed)
	assert.True(t, b.stopped.Load())

	// Nothing to run is nothing to wait for
	empty, err := toolkit.New(t.Context())
	require.NoError(t, err)
	defer empty.Close()
	assert.NoError(t, empty.Run(t.Context(), nil))
}
