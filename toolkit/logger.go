package toolkit

import (
	"context"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"sync"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	attribute "go.opentelemetry.io/otel/attribute"
	trace "go.opentelemetry.io/otel/trace"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// logSession is the session of a tool whose logger streams what the tool
// logs, at the info level or above, as lines, and adds what it logs to a
// span, as events
type logSession struct {
	id     uuid.UUID
	log    *toolLog
	logger *slog.Logger
	used   *toolUsage
}

// toolUsage is the tokens a tool used, which it adds to as it runs, from
// each goroutine it runs in
type toolUsage struct {
	sync.Mutex
	usage llm.Usage
}

var _ Session = logSession{}

// toolLog is where what a tool logs goes, which the handlers of its logger
// share
type toolLog struct {
	name   string                // The name the model called the tool by
	stream llm.GeneratorStreamFn // Streams a line, or progress, or nil when there's no stream
	span   trace.Span            // The span events are added to, or nil when there's no span
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
// name the model called it by, in the session with the identifier, or the
// nil UUID, when it isn't run in one. Its logger streams what the tool
// logs, at the info level or above, to the function, as log lines of the
// level, when it isn't info, the message, and its attributes, as
// key=value, which are shown to the user, so needn't say which tool it is,
// and its progress is streamed to the function too, unless it's nil. What
// it logs, at every level, and its progress, are added to the span, as
// events, with the tool's name, unless the span is nil. The function is
// called from each goroutine the tool, or tools run in parallel, stream
// from, so must be safe to call at once.
func NewSession(name string, id uuid.UUID, stream llm.GeneratorStreamFn, span trace.Span) Session {
	log := &toolLog{name: name, stream: stream, span: span}
	return logSession{id: id, log: log, logger: slog.New(&toolHandler{toolLog: log}), used: &toolUsage{}}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ID returns the identifier of the session the tool runs in, or the nil
// UUID
func (s logSession) ID() uuid.UUID {
	return s.id
}

func (s logSession) Logger() *slog.Logger {
	return s.logger
}

// Usage adds the tokens the tool used to those of its result
func (s logSession) Usage(usage llm.Usage) {
	s.used.Lock()
	defer s.used.Unlock()
	s.used.usage = s.used.usage.Add(usage)
}

// usage returns the tokens the tool used, which its result has
func (s logSession) usage() llm.Usage {
	s.used.Lock()
	defer s.used.Unlock()
	return s.used.usage
}

// Progress streams the tool's progress, which replaces its progress before,
// as an llm.Progress, and adds it to the span, as an event
func (s logSession) Progress(progress, total float64, message string) {
	if s.log.span != nil {
		s.log.span.AddEvent("progress", trace.WithAttributes(
			attribute.String("tool", s.log.name),
			attribute.Float64("progress", progress),
			attribute.Float64("total", total),
			attribute.String("message", message),
		))
	}
	if s.log.stream != nil {
		s.log.stream(llm.StreamProgress, llm.Progress{Progress: progress, Total: total, Message: message}.Text())
	}
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
		h.stream(llm.StreamLog, formatLine(record, attrs))
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
func formatLine(record slog.Record, attrs []slog.Attr) string {
	var line strings.Builder
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
