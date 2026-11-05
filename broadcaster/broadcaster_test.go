package broadcaster_test

import (
	"context"
	"testing"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	broadcaster "github.com/mutablelogic/go-llm/broadcaster"
	schema "github.com/mutablelogic/go-llm/broadcaster/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// watch watches the broadcaster with a buffered channel, until the test
// ends, and returns the channel
func watch(t *testing.T, b *broadcaster.Broadcaster) chan schema.Event {
	t.Helper()
	ch := make(chan schema.Event, 16)
	ctx, cancel := context.WithCancel(context.Background())
	done, err := b.Watch(ctx, ch)
	require.NoError(t, err)
	t.Cleanup(func() {
		cancel()
		assert.NoError(t, <-done)
	})
	return ch
}

// events returns the events which have been sent to the channel
func events(ch chan schema.Event) []schema.Event {
	var result []schema.Event
	for {
		select {
		case event := <-ch:
			result = append(result, event)
		default:
			return result
		}
	}
}

func Test_Broadcaster(t *testing.T) {
	ctx := context.Background()
	created := schema.Event{Action: schema.EventCreate, Table: "provider", Row: "ollama"}
	deleted := schema.Event{Action: schema.EventDelete, Table: "provider", Row: "ollama"}

	t.Run("Publish", func(t *testing.T) {
		// Each watcher is sent the events, in the order they're published
		b := broadcaster.New()
		a, c := watch(t, b), watch(t, b)
		b.Publish(created)
		b.Publish(deleted)
		assert.Equal(t, []schema.Event{created, deleted}, events(a))
		assert.Equal(t, []schema.Event{created, deleted}, events(c))
	})

	t.Run("Errors", func(t *testing.T) {
		b := broadcaster.New()
		_, err := b.Watch(ctx, nil)
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		watchCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		ch := make(chan schema.Event, 1)
		_, err = b.Watch(watchCtx, ch)
		require.NoError(t, err)
		_, err = b.Watch(watchCtx, ch)
		assert.ErrorIs(t, err, llm.ErrConflict)
	})

	t.Run("Cancelled", func(t *testing.T) {
		// Once the watch ends, no more events are sent, so the channel can
		// be closed, and it can watch again
		b := broadcaster.New()
		ch := make(chan schema.Event, 16)
		watchCtx, cancel := context.WithCancel(ctx)
		done, err := b.Watch(watchCtx, ch)
		require.NoError(t, err)
		cancel()
		require.NoError(t, <-done)
		close(ch)
		b.Publish(created)

		ch = watch(t, b)
		b.Publish(deleted)
		assert.Equal(t, []schema.Event{deleted}, events(ch))
	})

	t.Run("Stopped", func(t *testing.T) {
		// When the broadcaster stops running, the watches end, with
		// ErrServiceUnavailable, and it isn't watched until it runs again
		b := broadcaster.New()
		runCtx, stop := context.WithCancel(ctx)
		running := make(chan error)
		go func() { running <- b.Run(runCtx) }()
		ch := make(chan schema.Event, 16)
		done, err := b.Watch(ctx, ch)
		require.NoError(t, err)
		stop()
		require.NoError(t, <-running)
		assert.ErrorIs(t, <-done, llm.ErrServiceUnavailable)
		_, err = b.Watch(ctx, ch)
		assert.ErrorIs(t, err, llm.ErrServiceUnavailable)

		// And it runs again, until it's stopped, which is waited for, so it
		// doesn't outlast the test
		runCtx, stop = context.WithCancel(ctx)
		go func() { running <- b.Run(runCtx) }()
		require.Eventually(t, func() bool {
			done, err := b.Watch(runCtx, make(chan schema.Event))
			if err == nil {
				go func() { <-done }()
			}
			return err == nil
		}, time.Second, time.Millisecond)
		stop()
		require.NoError(t, <-running)
	})

	t.Run("Full", func(t *testing.T) {
		// A channel which is full misses events, rather than the publisher
		// waiting for it
		b := broadcaster.New()
		watchCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		_, err := b.Watch(watchCtx, make(chan schema.Event))
		require.NoError(t, err)
		published := make(chan struct{})
		go func() { b.Publish(created); close(published) }()
		select {
		case <-published:
		case <-time.After(time.Second):
			t.Fatal("publishing waited for a watcher")
		}
	})
}
