package toolkit_test

import (
	"context"
	"log/slog"
	"testing"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
	attribute "go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	tracetest "go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// logAll logs at each level, with attributes, in groups
func logAll(logger *slog.Logger) {
	logger.Debug("starting")
	logger.Info("fetching", "city", "Berlin")
	logger.WithGroup("quota").Warn("low", "remaining", 3, "note", "two words")
	logger.With(slog.Group("req", "id", 7)).Error("failed", "empty", "")
}

func Test_NewSession(t *testing.T) {
	t.Run("Stream", func(t *testing.T) {
		// What's logged at the info level or above is streamed, as lines
		var lines []string
		logAll(toolkit.NewSession("weather", uuid.Nil(), func(_ llm.StreamType, line string) { lines = append(lines, line) }, nil).Logger())
		assert.Equal(t, []string{
			"fetching city=Berlin",
			`WARN low quota.remaining=3 quota.note="two words"`,
			`ERROR failed req.id=7 empty=""`,
		}, lines)
	})

	t.Run("Span", func(t *testing.T) {
		// What's logged at every level is added to the span, as events
		recorder := tracetest.NewSpanRecorder()
		tracer := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)).Tracer("test")
		_, span := tracer.Start(context.Background(), "test")
		logAll(toolkit.NewSession("weather", uuid.Nil(), nil, span).Logger())
		span.End()

		require.Len(t, recorder.Ended(), 1)
		events := recorder.Ended()[0].Events()
		require.Len(t, events, 4)
		assert.Equal(t, "starting", events[0].Name)
		assert.Contains(t, events[0].Attributes, attribute.String("level", "DEBUG"))
		assert.Contains(t, events[0].Attributes, attribute.String("tool", "weather"))
		assert.Contains(t, events[1].Attributes, attribute.String("city", "Berlin"))
		assert.Contains(t, events[2].Attributes, attribute.String("quota.remaining", "3"))
		assert.Contains(t, events[3].Attributes, attribute.String("req.id", "7"))
	})

	t.Run("Token", func(t *testing.T) {
		// Neither the stream, nor the span, has the token of a webhook's URL,
		// in a message, an attribute, its key, or progress
		const token = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
		const link = "https://example.com/api/hook/" + token
		recorder := tracetest.NewSpanRecorder()
		_, span := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)).Tracer("test").Start(context.Background(), "test")
		var lines []string
		session := toolkit.NewSession("fetch", uuid.Nil(), func(_ llm.StreamType, line string) { lines = append(lines, line) }, span)
		toolkit.Logf(toolkit.WithSession(context.Background(), session), "Reading %s", link)
		session.Logger().With("from", link).Info("reading", "url", link, slog.Group("group", "url", link), link, "key")
		session.Logger().WithGroup(link).Info("reading", "url", "group")
		session.Progress(1, 2, "Reading "+link)
		span.End()

		require.Len(t, lines, 4)
		assert.Equal(t, "Reading https://example.com/api/hook/{token}", lines[0])
		assert.Contains(t, lines[3], "hook/{token}")
		for _, line := range lines {
			assert.NotContains(t, line, token)
		}
		require.Len(t, recorder.Ended()[0].Events(), 4)
		for _, event := range recorder.Ended()[0].Events() {
			assert.NotContains(t, event.Name, token)
			for _, attr := range event.Attributes {
				assert.NotContains(t, string(attr.Key), token)
				assert.NotContains(t, attr.Value.Emit(), token)
			}
		}
	})

	t.Run("Neither", func(t *testing.T) {
		// With neither a stream nor a span, nothing is enabled
		logger := toolkit.NewSession("weather", uuid.Nil(), nil, nil).Logger()
		assert.False(t, logger.Enabled(context.Background(), slog.LevelError))
		logAll(logger)
	})
}
