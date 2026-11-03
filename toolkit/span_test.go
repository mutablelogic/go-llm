package toolkit_test

import (
	"context"
	"encoding/json"
	"testing"

	// Packages
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	tracetest "go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestSpans(t *testing.T) {
	ctx := context.Background()
	t.Setenv("TOOLKIT_TEST_KEY", "secret")
	recorder := tracetest.NewSpanRecorder()
	tk, err := toolkit.New(ctx, toolkit.WithTracer(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)).Tracer("test")))
	require.NoError(t, err)
	require.NoError(t, tk.Add("test", echoTool{name: "echo"}))

	// last returns the name and attributes of the last span which ended
	last := func(t *testing.T) (string, map[string]string) {
		t.Helper()
		spans := recorder.Ended()
		require.NotEmpty(t, spans)
		span := spans[len(spans)-1]
		attrs := make(map[string]string)
		for _, kv := range span.Attributes() {
			attrs[string(kv.Key)] = kv.Value.Emit()
		}
		return span.Name(), attrs
	}

	t.Run("CallTool", func(t *testing.T) {
		_, err := tk.CallTool(ctx, "test:echo", json.RawMessage(`{"city":"London"}`))
		require.NoError(t, err)
		name, attrs := last(t)
		assert.Equal(t, "llm.toolkit.CallTool", name)
		assert.Equal(t, map[string]string{"arg.name": "test:echo"}, attrs)
	})

	t.Run("Run", func(t *testing.T) {
		// A tool which a model calls, rather than with CallTool, is run in a
		// span, with its canonical name, rather than the name the model
		// calls it by
		tools, err := tk.Tools(ctx, "test:echo")
		require.NoError(t, err)
		require.Len(t, tools, 1)
		_, err = tools[0].Run(ctx, json.RawMessage(`{"city":"London"}`))
		require.NoError(t, err)
		name, attrs := last(t)
		assert.Equal(t, "llm.toolkit.Run", name)
		assert.Equal(t, map[string]string{"arg.name": "test:echo"}, attrs)
	})

	t.Run("Credential", func(t *testing.T) {
		// A credential is got in a span, with its key, but not its value
		value, err := tk.Credential(ctx, "TOOLKIT_TEST_KEY")
		require.NoError(t, err)
		assert.Equal(t, "secret", value)
		name, attrs := last(t)
		assert.Equal(t, "llm.toolkit.Credential", name)
		assert.Equal(t, map[string]string{"arg.key": "TOOLKIT_TEST_KEY"}, attrs)
		for _, span := range recorder.Ended() {
			for _, kv := range span.Attributes() {
				assert.NotContains(t, kv.Value.Emit(), "secret", span.Name())
			}
		}
	})

	t.Run("Tools", func(t *testing.T) {
		_, err := tk.Tools(ctx, "test:*")
		require.NoError(t, err)
		name, attrs := last(t)
		assert.Equal(t, "llm.toolkit.Tools", name)
		assert.Equal(t, `["test:*"]`, attrs["arg.names"])
	})
}
