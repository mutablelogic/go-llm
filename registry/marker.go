package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
	"uuid"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// DefaultCompactAgent is the agent which summarises a conversation, as
	// it's compacted
	DefaultCompactAgent = "compact"

	// compactThreshold is how much of its model's context a conversation
	// fills, as of a reply, before it's compacted, automatically
	compactThreshold = 0.8

	// markerRetries is how many times a marker is added, or changed, again,
	// when its session changed as it was, such as when a reply was stored
	markerRetries = 5

	// transcriptChars is the most characters of a transcript which is
	// summarised, when its model's context isn't known, and charsPerToken
	// is about how many characters a token is, to work it out, when it is
	transcriptChars = 200_000
	charsPerToken   = 3

	// toolResultChars is the most characters of each tool's result which is
	// in a transcript
	toolResultChars = 500
)

// summaryPrompt is the heading the summary of a conversation's earlier
// messages is under, in the system prompt, in their place
const summaryPrompt = "# Earlier in this conversation\n\nThe conversation was compacted: its earlier messages aren't sent to you, and this is a summary of them, which you wrote.\n\n"

// errNothingToCompact is the error of a conversation which has no messages
// since it was last compacted
var errNothingToCompact = llm.ErrBadParameter.With("there's nothing to compact, as there are no messages since it was last compacted")

// errSummarizing is the error of a conversation whose last summary is still
// being written, which isn't compacted again until it is
var errSummarizing = llm.ErrConflict.With("its last summary is still being written")

// errPending is returned by addMarker, when the session's last marker is
// pending
var errPending = errors.New("the last marker is pending")

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// CompactSession compacts the session with the identifier: a marker is
// added to its messages, after which its context starts, so the messages
// before it are kept, but aren't sent to its model, and returns it, without
// its messages. A summary of them, and a title for the conversation, are
// written in the background, by the compact agent, and added to the marker,
// and the model is sent the summary in their place, once it's written, and
// the title is the session's.
// Returns ErrNotFound when there's no session, ErrForbidden when it's an
// event's, ErrBadParameter when it has no messages since it was last
// compacted, and ErrServiceUnavailable when the registry has no session
// store.
func (r *Registry) CompactSession(ctx context.Context, id uuid.UUID) (_ *schema.Session, err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.CompactSession",
		attribute.String("arg.id", id.String()),
	)
	defer func() { endSpan(err) }()
	return r.compactSession(ctx, id, "asked")
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// compactSession adds a marker to the session, for the reason, and writes its
// summary, in the background, unless the summary of its last marker is
// still being written, when it returns ErrConflict, or is pending, but
// isn't being written, such as when the server stopped as it was, when it's
// written again, rather than another marker added
func (r *Registry) compactSession(ctx context.Context, id uuid.UUID, reason string) (*schema.Session, error) {
	if r.sessionStore == nil {
		return nil, errNoSessionStore
	}
	marker := providerschema.Marker{ID: uuid.New(), Reason: reason, Status: providerschema.MarkerPending}
	var session schema.Session
	var pending uuid.UUID
	var err error
	for attempt := 0; ; attempt++ {
		session, pending, err = r.addMarker(ctx, id, marker)
		if !errors.Is(err, llm.ErrConflict) || errors.Is(err, errPending) || attempt >= markerRetries {
			break
		}
	}
	switch {
	case errors.Is(err, errPending):
		if !r.summarizeInBackground(ctx, id, pending) {
			return nil, errSummarizing
		}
		return r.withoutMessages(session), nil
	case err != nil:
		return nil, err
	}
	r.summarizeInBackground(ctx, id, marker.ID)
	return r.withoutMessages(session), nil
}

// summarizeInBackground writes the summary of the session's marker, in a
// goroutine of its own, logging why, when it isn't, and returns whether it
// was started, which it isn't, when it's being written already
func (r *Registry) summarizeInBackground(ctx context.Context, id, markerID uuid.UUID) bool {
	if _, running := r.summarizing.LoadOrStore(markerID, struct{}{}); running {
		return false
	}
	go func() {
		defer r.summarizing.Delete(markerID)
		ctx := context.WithoutCancel(ctx)
		if err := r.summarize(ctx, id, markerID); err != nil {
			r.log().WarnContext(ctx, "conversation not summarised", "session", id, "error", err.Error())
		}
	}()
	return true
}

// addMarker adds the marker to the session's messages, and stores it, and
// publishes that it changed, unless the session's last marker is pending,
// when it returns errPending, with that marker's identifier
func (r *Registry) addMarker(ctx context.Context, id uuid.UUID, marker providerschema.Marker) (schema.Session, uuid.UUID, error) {
	r.lifecycle.RLock()
	defer r.lifecycle.RUnlock()
	session, err := r.sessionStore.Get(ctx, id)
	if err != nil {
		return schema.Session{}, uuid.Nil(), err
	} else if session.Event {
		return schema.Session{}, uuid.Nil(), llm.ErrForbidden.Withf("session %q is an event's, which isn't compacted", id)
	} else if i := lastMarker(session.Messages); i >= 0 && session.Messages[i].Marker.Status == providerschema.MarkerPending {
		return session, session.Messages[i].Marker.ID, errPending
	} else if after, _ := contextOf(session.Messages); len(after) == 0 {
		return schema.Session{}, uuid.Nil(), errNothingToCompact
	}
	session.Messages = append(session.Messages, providerschema.Message{
		MessageMeta: providerschema.MessageMeta{Sender: providerschema.RoleContext, Content: []providerschema.ContentBlock{}},
		Time:        time.Now().UTC(),
		Marker:      &marker,
	})
	if session, err = r.sessionStore.Put(ctx, session); err != nil {
		return schema.Session{}, uuid.Nil(), err
	}
	r.sessionChanged(eventschema.EventUpdate, session)
	return session, uuid.Nil(), nil
}

// summarize writes the summary of the messages before the session's marker,
// since the marker before it, with the compact agent, and the session's
// model, and adds it to the marker, with a title for the conversation, as
// it is now, which is its title too, or marks it failed, saying why
func (r *Registry) summarize(ctx context.Context, id, markerID uuid.UUID) (err error) {
	ctx, endSpan := otel.StartSpan(r.tracer, ctx, "llm.registry.Summarize",
		attribute.String("arg.session", id.String()),
		attribute.String("arg.marker", markerID.String()),
	)
	defer func() { endSpan(err) }()

	summary, title, err := r.writeSummary(ctx, id, markerID)
	return errors.Join(err, r.updateMarker(ctx, id, markerID, func(session *schema.Session, marker *providerschema.Marker, previous *providerschema.Marker) {
		if err != nil {
			marker.Status, marker.Error = providerschema.MarkerFailed, err.Error()
			return
		}
		marker.Status, marker.Summary, marker.Title = providerschema.MarkerSummarized, summary, title
		if title != "" {
			session.Title = types.Ptr(title)
		}
	}))
}

// writeSummary runs the compact agent, with the transcript of the
// messages before the marker, since the one before it, and the summary of
// that one, and returns the summary, and title, it writes
func (r *Registry) writeSummary(ctx context.Context, id, markerID uuid.UUID) (string, string, error) {
	session, err := r.sessionStore.Get(ctx, id)
	if err != nil {
		return "", "", err
	}
	end := markerIndex(session.Messages, markerID)
	if end < 0 {
		return "", "", llm.ErrNotFound.Withf("marker %q", markerID)
	}
	// Since the last marker which was summarised, so the messages after one
	// whose summary couldn't be written are summarised now
	start, previous := 0, ""
	if i := lastSummarized(session.Messages[:end]); i >= 0 {
		start, previous = i+1, session.Messages[i].Marker.Summary
	}

	// The transcript, which fits the model's context, with room to spare
	limit := transcriptChars
	if model, err := r.GetModel(ctx, types.Value(session.Model)); err == nil {
		if window := contextWindow(model, session.GeneratorMeta); window > 0 {
			limit = min(limit, int(window)*charsPerToken/2)
		}
	}
	input, err := json.Marshal(map[string]string{
		"transcript": transcript(session.Messages[start:end], limit),
		"summary":    previous,
		"title":      types.Value(session.Title),
	})
	if err != nil {
		return "", "", err
	}
	run, err := r.RunAgent(ctx, DefaultCompactAgent, schema.RunAgentRequest{Input: input, Session: id, Tools: compactTools(session.Tools, r.compactTools)}, nil)
	if err != nil {
		return "", "", err
	}
	var reply struct {
		Summary string `json:"summary"`
		Title   string `json:"title"`
	}
	if err := json.Unmarshal(run.Reply, &reply); err != nil {
		return "", "", llm.ErrInternalServerError.Withf("the summary isn't one: %v", err)
	} else if reply.Summary = strings.TrimSpace(reply.Summary); reply.Summary == "" {
		return "", "", llm.ErrInternalServerError.With("the summary is empty")
	}
	return reply.Summary, strings.Join(strings.Fields(reply.Title), " "), nil
}

// updateMarker calls the function with the session, its marker with the
// identifier, and the marker before it, or nil, which it changes, and
// stores it, again, when the session changed as it was, and publishes
// that it changed
func (r *Registry) updateMarker(ctx context.Context, id, markerID uuid.UUID, fn func(*schema.Session, *providerschema.Marker, *providerschema.Marker)) error {
	for attempt := 0; ; attempt++ {
		err := func() error {
			r.lifecycle.RLock()
			defer r.lifecycle.RUnlock()
			session, err := r.sessionStore.Get(ctx, id)
			if err != nil {
				return err
			}
			i := markerIndex(session.Messages, markerID)
			if i < 0 {
				return llm.ErrNotFound.Withf("marker %q", markerID)
			}
			var previous *providerschema.Marker
			if j := lastMarker(session.Messages[:i]); j >= 0 {
				previous = session.Messages[j].Marker
			}
			marker := *session.Messages[i].Marker
			fn(&session, &marker, previous)
			session.Messages[i].Marker = &marker
			if session, err = r.sessionStore.Put(ctx, session); err != nil {
				return err
			}
			r.sessionChanged(eventschema.EventUpdate, session)
			return nil
		}()
		if !errors.Is(err, llm.ErrConflict) || attempt >= markerRetries {
			return err
		}
	}
}

// compactIfFull compacts the session, in the background, when its
// conversation, as of a reply, fills more of its model's context than the
// threshold, unless its last marker's summary is still being written
func (r *Registry) compactIfFull(ctx context.Context, session schema.Session, usage schema.SessionUsage) {
	tokens := usage.Input + usage.Output + usage.Overhead
	if tokens == 0 || session.Event {
		return
	}
	// A summary which is pending, but isn't being written, such as when the
	// server stopped as it was, is written again, rather than another
	// marker added
	if i := lastMarker(session.Messages); i >= 0 && session.Messages[i].Marker.Status == providerschema.MarkerPending {
		r.summarizeInBackground(ctx, session.ID, session.Messages[i].Marker.ID)
		return
	}
	model, err := r.GetModel(ctx, types.Value(session.Model))
	if err != nil {
		return
	}
	window := contextWindow(model, session.GeneratorMeta)
	if window == 0 || float64(tokens) < compactThreshold*float64(window) {
		return
	}
	if _, err := r.compactSession(ctx, session.ID, "full"); err != nil && !errors.Is(err, errNothingToCompact) && !errors.Is(err, errSummarizing) {
		r.log().WarnContext(ctx, "conversation not compacted", "session", session.ID, "error", err.Error())
	}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// compactTools returns the tools of the session, by their names, or
// patterns, of the namespaces, such as memory, which the compact agent is
// given, or none, when it has none of them
func compactTools(tools, namespaces []string) []string {
	result := []string{}
	for _, namespace := range namespaces {
		for _, tool := range tools {
			switch {
			case tool == "*" || tool == namespace+":*":
				result = append(result, namespace+":*")
			case strings.HasPrefix(tool, namespace+":"):
				result = append(result, tool)
			}
		}
	}
	slices.Sort(result)
	return slices.Compact(result)
}

// contextOf returns the messages of a conversation which are its context,
// which are those after its last marker, and the summary of the messages
// before it, once it's written
func contextOf(messages []providerschema.Message) ([]providerschema.Message, string) {
	// A marker whose summary couldn't be written doesn't start the context,
	// so nothing before it is left out, which compacting it again summarises
	i := len(messages) - 1
	for ; i >= 0; i-- {
		if marker := messages[i].Marker; marker != nil && marker.Status != providerschema.MarkerFailed {
			break
		}
	}
	if i < 0 {
		return withoutMarkers(messages), ""
	}
	summary := ""
	if marker := messages[i].Marker; marker.Status == providerschema.MarkerSummarized {
		summary = marker.Summary
	}
	return withoutMarkers(messages[i+1:]), summary
}

// contextStart returns the index of the first message of the conversation's
// context, which is its last marker, whose summary didn't fail, or zero,
// when it has none
func contextStart(messages []providerschema.Message) int {
	for i := len(messages) - 1; i >= 0; i-- {
		if marker := messages[i].Marker; marker != nil && marker.Status != providerschema.MarkerFailed {
			return i
		}
	}
	return 0
}

// withoutMarkers returns the messages without markers, such as those whose
// summaries couldn't be written, which aren't sent to a model
func withoutMarkers(messages []providerschema.Message) []providerschema.Message {
	for _, message := range messages {
		if message.Marker != nil {
			result := make([]providerschema.Message, 0, len(messages))
			for _, message := range messages {
				if message.Marker == nil {
					result = append(result, message)
				}
			}
			return result
		}
	}
	return messages
}

// lastSummarized returns the index of the last marker in the messages whose
// summary was written, or -1
func lastSummarized(messages []providerschema.Message) int {
	for i := len(messages) - 1; i >= 0; i-- {
		if marker := messages[i].Marker; marker != nil && marker.Status == providerschema.MarkerSummarized {
			return i
		}
	}
	return -1
}

// contextWindow returns the context of the model, in tokens, as a session
// asks it, which is the smaller of its input token limit, and the session's
// context_window option, as a request's is, or zero, when neither is known
func contextWindow(model llm.Model, meta schema.GeneratorMeta) uint {
	var window uint
	if value, ok := meta.Opt[llm.ContextWindowKey]; ok {
		_ = json.Unmarshal(value, &window)
	}
	if limit := model.InputTokenLimit(); limit != nil && *limit > 0 && (window == 0 || *limit < window) {
		window = *limit
	}
	return window
}

// lastMarker returns the index of the last marker in the messages, or -1
func lastMarker(messages []providerschema.Message) int {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Marker != nil {
			return i
		}
	}
	return -1
}

// markerIndex returns the index of the marker with the identifier in the
// messages, or -1
func markerIndex(messages []providerschema.Message, id uuid.UUID) int {
	for i, message := range messages {
		if message.Marker != nil && message.Marker.ID == id {
			return i
		}
	}
	return -1
}

// transcript returns the messages as text, each with who sent it, which
// the compact agent summarises: their text, what's attached, the tools
// called, and the start of their results, without thinking, or markers, of
// at most the characters, which are the latest, when they're more
func transcript(messages []providerschema.Message, limit int) string {
	parts := make([]string, 0, len(messages))
	for _, message := range messages {
		if message.Marker != nil {
			continue
		}
		var lines []string
		for _, block := range message.Content {
			switch {
			case block.Text != nil && strings.TrimSpace(*block.Text) != "":
				lines = append(lines, strings.TrimSpace(*block.Text))
			case block.Resource != nil && block.Resource.URL != nil:
				lines = append(lines, fmt.Sprintf("(attached %s, %s)", block.Resource.URL, block.Resource.ContentType))
			case block.ToolCall != nil:
				lines = append(lines, fmt.Sprintf("(called %s)", block.ToolCall.Name))
			case block.ToolResult != nil:
				result := strings.TrimSpace(string(block.ToolResult.Content))
				if runes := []rune(result); len(runes) > toolResultChars {
					result = string(runes[:toolResultChars]) + "…"
				}
				lines = append(lines, fmt.Sprintf("(%s returned %s)", block.ToolResult.Name, result))
			}
		}
		if len(lines) == 0 {
			continue
		}
		parts = append(parts, "["+message.Role()+"] "+strings.Join(lines, "\n"))
	}

	// The latest messages, which fit
	length, start := 0, len(parts)
	for start > 0 && length+len(parts[start-1])+2 <= limit {
		start--
		length += len(parts[start]) + 2
	}
	if start == len(parts) && start > 0 {
		// The latest message alone is too long, so it's its end, of at most
		// the limit, in bytes, as the others are, from the start of a
		// character
		last := parts[start-1]
		cut := max(len(last)-limit, 0)
		for cut < len(last) && !utf8.RuneStart(last[cut]) {
			cut++
		}
		parts[start-1] = last[cut:]
		start--
	}
	result := strings.Join(parts[start:], "\n\n")
	if start > 0 {
		result = "(earlier messages are left out)\n\n" + result
	}
	return result
}
