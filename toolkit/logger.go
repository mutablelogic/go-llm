package toolkit

import (
	"context"
	"log/slog"
	"slices"
	"strconv"
	"strings"

	// Packages
	attribute "go.opentelemetry.io/otel/attribute"
	trace "go.opentelemetry.io/otel/trace"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// logSession is the session of a tool whose logger streams what the tool
// logs, at the info level or above, as lines, and adds what it logs to a
// span, as events
type logSession struct {
	logger *slog.Logger
}

var _ Session = logSession{}

// toolLog is where what a tool logs goes, which the handlers of its logger
// share
type toolLog struct {
	name   string            // The name the model called the tool by
	stream func(text string) // Streams a line, or nil when there's no stream
	span   trace.Span        // The span events are added to, or nil when there's no span
}

// toolHandler is the handler of a tool's logger, with its attributes, and
// the group its attributes are in, as the prefix of their keys
type toolHandler struct {
	*toolLog
	attrs  []slog.Attr
	prefix string
}

var _ slog.Handler = (*toolHandler)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewSession returns the session of the tool with the name, such as the
// name the model called it by, whose logger streams what the tool logs, at
// the info level or above, to the function, as lines of the name, the
// level, when it isn't info, the message, and its attributes, as
// key=value, unless the function is nil, and adds what it logs, at every
// level, to the span, as events, with the tool's name and the level, unless
// the span is nil. The function is called from each goroutine the tool, or
// tools run in parallel, log from, so must be safe to call at once.
func NewSession(name string, stream func(string), span trace.Span) Session {
	return logSession{logger: slog.New(&toolHandler{toolLog: &toolLog{name: name, stream: stream, span: span}})}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (s logSession) Logger() *slog.Logger {
	return s.logger
}

// Enabled returns true when what's logged at the level is streamed, which is
// the info level or above, or added to the span, which is every level
func (h *toolHandler) Enabled(_ context.Context, level slog.Level) bool {
	return h.span != nil || (h.stream != nil && level >= slog.LevelInfo)
}

// Handle streams the record as a line, of the tool's name, its level, when
// it isn't info, its message, and its attributes, and adds it to the span,
// as an event
func (h *toolHandler) Handle(_ context.Context, record slog.Record) error {
	attrs := slices.Clone(h.attrs)
	record.Attrs(func(attr slog.Attr) bool {
		attrs = appendAttr(attrs, h.prefix, attr)
		return true
	})

	if h.span != nil {
		event := make([]attribute.KeyValue, 0, len(attrs)+2)
		event = append(event, attribute.String("tool", h.name), attribute.String("level", record.Level.String()))
		for _, attr := range attrs {
			event = append(event, attribute.String(attr.Key, attr.Value.String()))
		}
		h.span.AddEvent(record.Message, trace.WithAttributes(event...))
	}
	if h.stream != nil && record.Level >= slog.LevelInfo {
		h.stream(formatLine(h.name, record, attrs))
	}
	return nil
}

// WithAttrs returns the handler with the attributes, in its group
func (h *toolHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clone := *h
	clone.attrs = slices.Clone(h.attrs)
	for _, attr := range attrs {
		clone.attrs = appendAttr(clone.attrs, h.prefix, attr)
	}
	return &clone
}

// WithGroup returns the handler whose attributes are in the group, which is
// the prefix of their keys
func (h *toolHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	clone := *h
	clone.prefix = h.prefix + name + "."
	return &clone
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// appendAttr appends the attribute, whose key has the prefix, resolved, with
// the attributes of a group as attributes of their own, whose keys have the
// group's, and without an empty attribute
func appendAttr(attrs []slog.Attr, prefix string, attr slog.Attr) []slog.Attr {
	attr.Value = attr.Value.Resolve()
	if attr.Equal(slog.Attr{}) {
		return attrs
	}
	if attr.Value.Kind() == slog.KindGroup {
		if attr.Key != "" {
			prefix += attr.Key + "."
		}
		for _, member := range attr.Value.Group() {
			attrs = appendAttr(attrs, prefix, member)
		}
		return attrs
	}
	attr.Key = prefix + attr.Key
	return append(attrs, attr)
}

// formatLine returns the line of what the tool logged: its name, the level,
// when it isn't info, the message, and the attributes, as key=value, whose
// value is quoted when it's empty, or has spaces, quotes or equals signs
func formatLine(name string, record slog.Record, attrs []slog.Attr) string {
	var line strings.Builder
	line.WriteString(name)
	line.WriteString(": ")
	if record.Level != slog.LevelInfo {
		line.WriteString(record.Level.String())
		line.WriteString(" ")
	}
	line.WriteString(record.Message)
	for _, attr := range attrs {
		value := attr.Value.String()
		if value == "" || strings.ContainsAny(value, " \t\n\"=") {
			value = strconv.Quote(value)
		}
		line.WriteString(" ")
		line.WriteString(attr.Key)
		line.WriteString("=")
		line.WriteString(value)
	}
	return line.String()
}
