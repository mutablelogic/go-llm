//go:build !client

package cmd_test

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	eliza "github.com/mutablelogic/go-llm/provider/eliza"
	registry "github.com/mutablelogic/go-llm/registry"
	registrycmd "github.com/mutablelogic/go-llm/registry/cmd"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	server "github.com/mutablelogic/go-server"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	tracetest "go.opentelemetry.io/otel/sdk/trace/tracetest"
	trace "go.opentelemetry.io/otel/trace"
)

// command is the server's command, with its context, logger and tracer,
// its name and version, and its client options
type command struct {
	server.Cmd
	ctx    context.Context
	tracer trace.Tracer
}

func (c command) Context() context.Context { return c.ctx }
func (c command) Logger() *slog.Logger     { return slog.New(slog.DiscardHandler) }
func (c command) Tracer() trace.Tracer     { return c.tracer }
func (c command) Name() string             { return "llm" }
func (c command) Version() string          { return "v1.2.3" }

// ClientEndpoint returns the client options of the server, which time out
func (c command) ClientEndpoint() (string, []client.ClientOpt, error) {
	return "http://localhost:8084/api", []client.ClientOpt{client.OptTimeout(time.Minute)}, nil
}

func Test_WithRegistry(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	tracer := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)).Tracer("test")
	ctx := command{ctx: t.Context(), tracer: tracer}

	t.Run("Registry", func(t *testing.T) {
		// The function is called with a registry, which has the server's
		// tracer, and can have providers added
		called := false
		require.NoError(t, registrycmd.Flags{}.WithRegistry(ctx, nil, func(r *registry.Registry) error {
			called = true
			client, err := eliza.New()
			require.NoError(t, err)
			require.NoError(t, r.Add("", client))
			_, err = r.GetModel(t.Context(), eliza.ModelName)
			return err
		}))
		assert.True(t, called)
		if spans := recorder.Ended(); assert.NotEmpty(t, spans) {
			assert.Equal(t, "llm.registry.GetModel", spans[len(spans)-1].Name())
		}
	})

	t.Run("Options", func(t *testing.T) {
		// The options come after the server's, so they're used instead
		count := len(recorder.Ended())
		require.NoError(t, registrycmd.Flags{}.WithRegistry(ctx, nil, func(r *registry.Registry) error {
			return r.Ping(t.Context())
		}, registry.WithTracer(nil)))
		assert.Len(t, recorder.Ended(), count)
	})

	t.Run("ClientOpts", func(t *testing.T) {
		// The providers which are created have the server's client options,
		// with its name and version as the user agent
		var agent atomic.Value
		ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			agent.Store(r.UserAgent())
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"version":"0.0.1"}`))
		}))
		defer ollama.Close()
		require.NoError(t, registrycmd.Flags{}.WithRegistry(ctx, nil, func(r *registry.Registry) error {
			_, err := r.CreateProvider(t.Context(), schema.Provider{Type: llm.Ollama, URL: ollama.URL})
			return err
		}))
		assert.Equal(t, "llm/v1.2.3", agent.Load())
	})

	t.Run("Error", func(t *testing.T) {
		// The function's error is returned
		err := registrycmd.Flags{}.WithRegistry(ctx, nil, func(*registry.Registry) error {
			return llm.ErrNotFound
		})
		assert.True(t, errors.Is(err, llm.ErrNotFound))
	})
}
