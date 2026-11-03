package registry_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	registry "github.com/mutablelogic/go-llm/registry"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
	attribute "go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	tracetest "go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// credentials are credentials by their keys
type credentials map[string]string

var _ registry.CredentialsProvider = credentials{}

// Expand replaces the placeholders with the credentials, which must exist
func (c credentials) Expand(_ context.Context, value string) (string, error) {
	var err error
	result := os.Expand(value, func(key string) string {
		v, ok := c[key]
		if !ok {
			err = errors.Join(err, llm.ErrNotFound.Withf("credential %q", key))
		}
		return v
	})
	return result, err
}

// newOllama returns a stub Ollama server, which replies to pings, and
// records the last bearer token it was sent
func newOllama(t *testing.T) (*httptest.Server, *atomic.Value) {
	t.Helper()
	var token atomic.Value
	token.Store("")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token.Store(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		if strings.HasSuffix(r.URL.Path, "/version") {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"version":"0.0.1"}`))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(server.Close)
	return server, &token
}

func Test_CredentialsProvider(t *testing.T) {
	ctx := context.Background()
	ollama, token := newOllama(t)

	t.Run("Environment", func(t *testing.T) {
		// Without a credentials provider, an API key is sent as it is
		r, err := registry.New()
		require.NoError(t, err)
		_, err = r.CreateProvider(ctx, schema.Provider{Name: "plain", Type: llm.Ollama, URL: ollama.URL, APIKey: "secret"})
		require.NoError(t, err)
		assert.Equal(t, "secret", token.Load())

		// And its placeholders are environment variables
		t.Setenv("LLM_REGISTRY_TEST_KEY", "from-env")
		_, err = r.CreateProvider(ctx, schema.Provider{Name: "env", Type: llm.Ollama, URL: ollama.URL, APIKey: "${LLM_REGISTRY_TEST_KEY}"})
		require.NoError(t, err)
		assert.Equal(t, "from-env", token.Load())

		// Which must be set, or the request isn't valid
		_, err = r.CreateProvider(ctx, schema.Provider{Name: "unset", Type: llm.Ollama, URL: ollama.URL, APIKey: "$LLM_REGISTRY_TEST_UNSET"})
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		assert.Nil(t, r.Provider("unset"))

		// And so they are with a nil provider
		r, err = registry.New(registry.WithCredentialsProvider(nil))
		require.NoError(t, err)
		_, err = r.CreateProvider(ctx, schema.Provider{Type: llm.Ollama, URL: ollama.URL, APIKey: "$LLM_REGISTRY_TEST_KEY"})
		require.NoError(t, err)
		assert.Equal(t, "from-env", token.Load())
	})

	t.Run("Expand", func(t *testing.T) {
		// The API key's placeholders are replaced with their credentials
		r, err := registry.New(registry.WithCredentialsProvider(credentials{"OLLAMA_KEY": "expanded"}))
		require.NoError(t, err)
		_, err = r.CreateProvider(ctx, schema.Provider{Name: "expanded", Type: llm.Ollama, URL: ollama.URL, APIKey: "${OLLAMA_KEY}"})
		require.NoError(t, err)
		assert.Equal(t, "expanded", token.Load())

		// And a credential which doesn't exist is a request which isn't valid
		_, err = r.CreateProvider(ctx, schema.Provider{Name: "missing", Type: llm.Ollama, URL: ollama.URL, APIKey: "${MISSING}"})
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		assert.ErrorContains(t, err, "MISSING")
		assert.Nil(t, r.Provider("missing"))

		// But a credentials provider which fails otherwise is its error
		errVault := errors.New("vault is sealed")
		r, err = registry.New(registry.WithCredentialsProvider(failing{errVault}))
		require.NoError(t, err)
		_, err = r.CreateProvider(ctx, schema.Provider{Type: llm.Ollama, URL: ollama.URL, APIKey: "${OLLAMA_KEY}"})
		assert.ErrorIs(t, err, errVault)
		assert.NotErrorIs(t, err, llm.ErrBadParameter)
	})
}

// failing is a credentials provider which fails with its error
type failing struct{ err error }

func (f failing) Expand(context.Context, string) (string, error) { return "", f.err }

func Test_ClientOpts(t *testing.T) {
	// The providers which are created have the registry's client options
	ctx := context.Background()
	var agent atomic.Value
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agent.Store(r.UserAgent())
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"version":"0.0.1"}`))
	}))
	t.Cleanup(ollama.Close)
	opts := []client.ClientOpt{client.OptUserAgent("test/1.0")}
	r, err := registry.New(registry.WithClientOpts(opts...))
	require.NoError(t, err)

	// Including with an API key, which isn't added to the registry's options
	_, err = r.CreateProvider(ctx, schema.Provider{Type: llm.Ollama, URL: ollama.URL, APIKey: "secret"})
	require.NoError(t, err)
	assert.Equal(t, "test/1.0", agent.Load())
	assert.Len(t, opts, 1)
}

func Test_Tracer(t *testing.T) {
	// The registry's operations are traced, with their arguments
	ctx := context.Background()
	recorder := tracetest.NewSpanRecorder()
	tracer := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)).Tracer("test")
	r, err := registry.New(registry.WithTracer(tracer))
	require.NoError(t, err)
	require.NoError(t, r.Add("openai", &provider{name: "openai", models: []string{"gpt-4o"}}))

	_, err = r.GetModel(ctx, "gpt-4o")
	require.NoError(t, err)
	_, err = r.GetModel(ctx, "missing")
	require.Error(t, err)
	require.NoError(t, r.Ping(ctx))

	spans := recorder.Ended()
	require.Len(t, spans, 3)
	assert.Equal(t, "llm.registry.GetModel", spans[0].Name())
	assert.Contains(t, spans[0].Attributes(), attribute.String("arg.name", "gpt-4o"))
	assert.Equal(t, "Unset", spans[0].Status().Code.String())
	assert.Equal(t, "Error", spans[1].Status().Code.String())
	assert.Equal(t, "llm.registry.Ping", spans[2].Name())
}

func Test_Toolkit(t *testing.T) {
	// A registry has no toolkit, by default
	r, err := registry.New()
	require.NoError(t, err)
	assert.Nil(t, r.Toolkit())

	// Or the toolkit it's given
	tools, err := toolkit.New(context.Background())
	require.NoError(t, err)
	r, err = registry.New(registry.WithToolkit(tools))
	require.NoError(t, err)
	assert.Same(t, tools, r.Toolkit())

	// Which it must be given
	_, err = registry.New(registry.WithToolkit(nil))
	assert.ErrorIs(t, err, llm.ErrBadParameter)
}
