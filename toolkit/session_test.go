package toolkit_test

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// logSession is a session which logs to a buffer
type logSession struct{ logger *slog.Logger }

func (s logSession) ID() uuid.UUID                     { return uuid.Nil() }
func (s logSession) Logger() *slog.Logger              { return s.logger }
func (s logSession) Progress(float64, float64, string) {}

func Test_Session(t *testing.T) {
	t.Run("None", func(t *testing.T) {
		// A tool which isn't run in a session has one, whose logger discards
		// what's logged
		session := toolkit.SessionFromContext(context.Background())
		require.NotNil(t, session)
		require.NotNil(t, session.Logger())
		session.Logger().Info("discarded")
	})

	t.Run("Session", func(t *testing.T) {
		// A tool run in a session logs to the session's logger
		var buf bytes.Buffer
		ctx := toolkit.WithSession(context.Background(), logSession{slog.New(slog.NewTextHandler(&buf, nil))})
		toolkit.SessionFromContext(ctx).Logger().Info("hello", "city", "Berlin")
		assert.Contains(t, buf.String(), "msg=hello city=Berlin")
	})

	t.Run("Nil", func(t *testing.T) {
		// A nil session is no session
		ctx := toolkit.WithSession(context.Background(), nil)
		require.NotNil(t, toolkit.SessionFromContext(ctx).Logger())
	})
}

func Test_Logf(t *testing.T) {
	// A tool logs what it's doing as a sentence, to its session, and to
	// nothing without one
	var lines []string
	ctx := toolkit.WithSession(context.Background(), toolkit.NewSession("current", uuid.Nil(), func(_ llm.StreamType, line string) { lines = append(lines, line) }, nil))
	toolkit.Logf(ctx, "Getting the current weather for %s", "Berlin")
	toolkit.Logf(context.Background(), "Not shown")
	assert.Equal(t, []string{"Getting the current weather for Berlin"}, lines)
}
