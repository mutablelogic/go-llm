package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// history is the conversation the agent loop sends to a model: the caller's
// messages, and the messages the loop adds, with the tokens of each, and of
// what's sent with them, which is compacted to fit the model's context. The
// caller's messages, and the messages the loop returns, aren't changed by
// compacting it, except the results of tools it truncates, which are
// returned as they're sent.
type history struct {
	entries  []historyEntry
	added    []providerschema.Message // The messages the loop adds, which it returns, each with its tokens
	measured []uint                   // The tokens of each of the caller's messages, before they're compacted
	overhead uint                     // The tokens of what's sent with the messages, such as the system prompt and the tools
	measure  toolkit.MeasureFn
	text     textFunc       // The text an attachment is sent as, when it's text, or nil
	zone     *time.Location // The time zone the person's messages start with when they were sent in, or nil, when they don't
}

// textFunc returns the text which an attachment is sent to the model as,
// when its resource is text, such as a file of code, and whether it is
type textFunc func(context.Context, providerschema.Resource) (string, bool)

// historyEntry is a message of the conversation, with its tokens
type historyEntry struct {
	message llm.Message
	tokens  uint
	pinned  bool // Never dropped: a system message, a new message, or the latest user message
	latest  bool // Of the latest turn: the latest reply, and the results of the tools it called
	removed bool // Its tools' results have been removed
	added   int  // The index of the message in the messages the loop adds, or -1
}

// compacted is how the conversation was compacted
type compacted struct {
	removed   int // Messages whose tools' results were removed
	truncated int // Results of tools which were truncated
	dropped   int // Messages which were dropped
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// DefaultToolResultLimit is the most tokens of a tool's result the agent
	// loop sends to the model, when the model's context isn't known
	DefaultToolResultLimit = 8192

	// contextFill is the fraction of the model's context the agent loop fills,
	// at most, so the rest is left for the model's reply, and what follows
	contextFill = 0.66

	// minToolResultLimit is the fewest tokens of a tool's result the agent
	// loop sends to the model, when its context is full
	minToolResultLimit = 256

	// attachmentTokens is an estimate of the tokens of an attachment, such as
	// an image, which providers send as media, rather than as its data as
	// text, so its tokens aren't those of its data
	attachmentTokens = 1500

	// timeTokens is an estimate of the tokens of when a person's message was
	// sent, which it starts with, when the model is told, such as (Saturday
	// 2026-11-07T12:38+01:00), which is at most this
	timeTokens = 24
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// newHistory returns the conversation of the caller's messages, none of
// which can be nil, followed by the new messages, which are the first the
// loop adds, and returns, each with its tokens, when it has them, or measured, with the tokens of what's sent with them, which are estimated
// until a model has been sent them. The system messages, the new messages,
// such as all of an agent's prompt, and the latest user message, are never
// dropped, so a conversation they don't fit in isn't sent, rather than
// having part of them dropped, and, when the messages end with the results
// of tools, they're the latest turn, with the reply which called them.
func newHistory(ctx context.Context, messages []llm.Message, fresh []providerschema.Message, overhead uint, measure toolkit.MeasureFn, text textFunc, zone *time.Location) (*history, error) {
	h := &history{entries: make([]historyEntry, 0, len(messages)+len(fresh)), overhead: overhead, measure: measure, text: text, zone: zone}
	for i, message := range messages {
		if isNil(message) {
			return nil, llm.ErrBadParameter.Withf("message %d is nil", i)
		}
		tokens, err := h.tokens(ctx, message)
		if err != nil {
			return nil, err
		}
		h.measured = append(h.measured, tokens)
		h.entries = append(h.entries, historyEntry{message: message, tokens: tokens + h.timeTokens(message), pinned: message.Role() == "system", added: -1})
	}

	// The new messages, which are added, so they're returned as they're
	// sent, such as when their tools' results are truncated, with their
	// tokens
	for _, message := range fresh {
		stamp(&message)
		tokens, err := h.tokens(ctx, &message)
		if err != nil {
			return nil, err
		}
		message.Tokens = uint64(tokens)
		h.added = append(h.added, message)
		h.entries = append(h.entries, historyEntry{message: &message, tokens: tokens + h.timeTokens(&message), pinned: true, added: len(h.added) - 1})
	}

	// The latest user message, which isn't the results of tools
	for i := len(h.entries) - 1; i >= 0; i-- {
		if message := h.entries[i].message; message.Role() == "user" && len(message.ToolResults()) == 0 {
			h.entries[i].pinned = true
			break
		}
	}

	// The latest turn, of the results of tools the messages end with, and the
	// reply which called them
	for i := len(h.entries) - 1; i >= 0; i-- {
		h.entries[i].latest = true
		if h.entries[i].message.Role() != "tool" {
			break
		}
	}
	if n := len(h.entries); n > 0 && h.entries[n-1].message.Role() != "tool" {
		h.entries[n-1].latest = false
	}
	return h, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// messages returns the messages sent to the model
func (h *history) messages() []llm.Message {
	result := make([]llm.Message, len(h.entries))
	for i, entry := range h.entries {
		result[i] = entry.message
	}
	return result
}

// size returns the tokens of the conversation, and what's sent with it
func (h *history) size() uint {
	size := h.overhead
	for _, entry := range h.entries {
		size += entry.tokens
	}
	return size
}

// sent sets the tokens of what's sent with the messages, from the usage of
// the request they were sent in, when it's reported, as the tokens of its
// input, less those of the messages
func (h *history) sent(usage llm.Usage) {
	if usage.InputTokens == 0 {
		return
	}
	h.overhead = 0
	if messages := h.size(); usage.InputTokens > messages {
		h.overhead = usage.InputTokens - messages
	}
}

// add adds the messages of an iteration, which is the latest turn, to the
// conversation, and to the messages the loop adds, each with its tokens: the
// model's reply, whose tokens are those of its output, less those of
// reasoning which isn't returned, when they're reported, and the results of
// the tools it called, or the request to call the respond tool
func (h *history) add(ctx context.Context, step []providerschema.Message, usage llm.Usage) error {
	for i := range h.entries {
		h.entries[i].latest = false
	}
	for i := range step {
		h.added = append(h.added, step[i])
		message := &h.added[len(h.added)-1]
		stamp(message)
		tokens := usage.OutputTokens - min(usage.OutputTokens, usage.ReasoningTokens)
		if i > 0 || tokens == 0 {
			var err error
			if tokens, err = h.measureMessage(ctx, message); err != nil {
				return err
			}
		}
		message.Tokens = uint64(tokens)
		copied := *message
		h.entries = append(h.entries, historyEntry{message: &copied, tokens: tokens, latest: true, added: len(h.added) - 1})
	}
	return nil
}

// removeResults removes the results of the tools of the messages, which
// aren't of the latest turn, oldest first, as the model has seen them, with
// a note of what was removed, until the conversation fits the budget.
// Returns how many messages' results were removed.
func (h *history) removeResults(ctx context.Context, budget uint) (int, error) {
	var removed int
	for i := range h.entries {
		if h.size() <= budget {
			break
		}
		entry := &h.entries[i]
		message, ok := schemaMessage(entry.message)
		if entry.latest || entry.removed || !ok || len(message.ToolResults()) == 0 {
			continue
		}
		message.Content = slices.Clone(message.Content)
		for j, block := range message.Content {
			if block.ToolResult == nil || block.ToolResult.IsError {
				continue
			}
			result := *block.ToolResult
			result.Content, _ = json.Marshal(fmt.Sprintf("[the result, of %d bytes, was removed, to fit the model's context]", len(result.Content)))
			message.Content[j].ToolResult = &result
		}
		tokens, err := h.measureMessage(ctx, message)
		if err != nil {
			return 0, err
		}
		entry.message, entry.tokens, entry.removed = message, tokens, true
		removed++
	}
	return removed, nil
}

// truncateResults truncates the results of the tools of the latest turn,
// which aren't errors, to the most tokens each, as toolkit.TruncateResult
// does, and logs each which is, in the session the function returns for
// its tool, by name. The results the loop added are returned as they're
// truncated. Returns how many results were truncated.
func (h *history) truncateResults(ctx context.Context, limit uint, session func(name string) toolkit.Session) (int, error) {
	var truncated int
	for i := range h.entries {
		entry := &h.entries[i]
		message, ok := schemaMessage(entry.message)
		if !entry.latest || !ok || len(message.ToolResults()) == 0 {
			continue
		}
		message.Content = slices.Clone(message.Content)
		var changed bool
		for j, block := range message.Content {
			if block.ToolResult == nil || block.ToolResult.IsError {
				continue
			}
			result := *block.ToolResult
			content, err := toolkit.TruncateResult(ctx, result.Content, limit, h.measure)
			if err != nil {
				return 0, err
			} else if bytes.Equal(content, result.Content) {
				continue
			}
			// What's said in the conversation, and the numbers, for the
			// trace, which aren't
			logger := session(result.Name).Logger()
			logger.Info(fmt.Sprintf("Shortened the tool's result from %s to %s, to fit the conversation's memory window", byteSize(len(result.Content)), byteSize(len(content))))
			logger.Debug("result truncated", "bytes", len(result.Content), "to_bytes", len(content), "limit_tokens", limit)
			result.Content = content
			message.Content[j].ToolResult = &result
			changed = true
			truncated++
		}
		if !changed {
			continue
		}
		tokens, err := h.measureMessage(ctx, message)
		if err != nil {
			return 0, err
		}
		entry.message, entry.tokens = message, tokens
		if entry.added >= 0 {
			message.Tokens = uint64(tokens)
			h.added[entry.added] = *message
		}
	}
	return truncated, nil
}

// latestResults returns how many results of the tools of the latest turn
// there are, which aren't errors, and the tokens of their messages
func (h *history) latestResults() (int, uint) {
	var count int
	var tokens uint
	for _, entry := range h.entries {
		if !entry.latest || entry.message.Role() != "tool" {
			continue
		}
		for _, result := range entry.message.ToolResults() {
			if !result.IsError {
				count++
			}
		}
		tokens += entry.tokens
	}
	return count, tokens
}

// dropTurns drops the messages which aren't pinned, or of the latest turn,
// oldest first, until the conversation fits the budget, with the results of
// the tools a reply called, which follow it, so each call has its result.
// Returns how many messages were dropped.
func (h *history) dropTurns(budget uint) int {
	var dropped int
	for h.size() > budget {
		i := slices.IndexFunc(h.entries, func(entry historyEntry) bool { return !entry.pinned && !entry.latest })
		if i < 0 {
			break
		}
		end := i + 1
		if len(h.entries[i].message.ToolCalls()) > 0 {
			for end < len(h.entries) && h.entries[end].message.Role() == "tool" && !h.entries[end].latest && !h.entries[end].pinned {
				end++
			}
		}
		h.entries = slices.Delete(h.entries, i, end)
		dropped += end - i
	}
	return dropped
}

// tokens returns the tokens of the message, which are its own, when it's a
// message of the provider schema with tokens, or otherwise as it's measured
func (h *history) tokens(ctx context.Context, message llm.Message) (uint, error) {
	if message, ok := schemaMessage(message); ok && message.Tokens > 0 {
		return uint(message.Tokens), nil
	}
	return h.measureMessage(ctx, message)
}

// measureMessage returns the tokens of the message, as its content as JSON,
// when it's a message of the provider schema, without the data of its
// attachments, each of which is attachmentTokens, unless it's text, which is
// sent as text, and measured as it is, or the usage of its tools' results,
// which aren't sent to the model, or otherwise its text
func (h *history) measureMessage(ctx context.Context, message llm.Message) (uint, error) {
	schema, ok := schemaMessage(message)
	if !ok {
		return h.measure(ctx, message.Text())
	}
	content := slices.Clone(schema.Content)
	var attachments, texts uint
	for i, block := range content {
		if block.Resource != nil {
			resource := *block.Resource
			if text, ok := h.attachmentText(ctx, resource); ok {
				tokens, err := h.measure(ctx, text)
				if err != nil {
					return 0, err
				}
				texts += tokens
			} else {
				attachments++
			}
			resource.Data = nil
			content[i].Resource = &resource
		}
		if block.ToolResult != nil && block.ToolResult.Usage != nil {
			result := *block.ToolResult
			result.Usage = nil
			content[i].ToolResult = &result
		}
	}
	data, err := json.Marshal(content)
	if err != nil {
		return 0, err
	}
	tokens, err := h.measure(ctx, string(data))
	if err != nil {
		return 0, err
	}
	return tokens + attachments*attachmentTokens + texts, nil
}

// attachmentText returns the text the attachment is sent as, when it's
// text, and whether it is
func (h *history) attachmentText(ctx context.Context, resource providerschema.Resource) (string, bool) {
	if h.text == nil {
		return "", false
	}
	return h.text(ctx, resource)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// isNil returns true when the message is nil, or a nil pointer
func isNil(message llm.Message) bool {
	if message == nil {
		return true
	}
	value := reflect.ValueOf(message)
	return value.Kind() == reflect.Pointer && value.IsNil()
}

// timeTokens returns the tokens of when the message was sent, which it
// starts with, as it's sent, when it's the person's, and the model is told
// when they were, which the compaction of the conversation counts, though
// they aren't the message's own
func (h *history) timeTokens(message llm.Message) uint {
	if h.zone != nil && sentAt(message) {
		return timeTokens
	}
	return 0
}

// sentAt returns whether the message is one of the person's, rather than
// the results of tools, which has when it was sent
func sentAt(m llm.Message) bool {
	message, ok := schemaMessage(m)
	return ok && message.Role() == "user" && len(message.ToolResults()) == 0 && !message.Time.IsZero()
}

// withTimes returns the messages, with when each of the person's messages
// was sent, when it's known, in the time zone, at its start, such as
// (Saturday 2026-11-07T12:38+01:00), which doesn't change as the
// conversation goes on, so it's cached. The messages aren't changed.
func withTimes(messages []llm.Message, zone *time.Location) []llm.Message {
	result := make([]llm.Message, len(messages))
	for i, m := range messages {
		result[i] = m
		if !sentAt(m) {
			continue
		}
		message, _ := schemaMessage(m)
		note := "(" + message.Time.In(zone).Format(timeFormat) + ")"
		message.Content = append([]providerschema.ContentBlock{{Text: types.Ptr(note)}}, message.Content...)
		result[i] = message
	}
	return result
}

// stamp sets when the message was added to the conversation, now, unless
// it's set
func stamp(message *providerschema.Message) {
	if message.Time.IsZero() {
		message.Time = time.Now().UTC()
	}
}

// schemaMessage returns a copy of the message, when it's a message of the
// provider schema, or its meta, by value or pointer, which can be changed
// without changing the message, once its content is cloned
func schemaMessage(message llm.Message) (*providerschema.Message, bool) {
	switch m := message.(type) {
	case *providerschema.Message:
		if m == nil {
			return nil, false
		}
		copied := *m
		return &copied, true
	case providerschema.Message:
		return &m, true
	case *providerschema.MessageMeta:
		if m == nil {
			return nil, false
		}
		return &providerschema.Message{MessageMeta: *m}, true
	case providerschema.MessageMeta:
		return &providerschema.Message{MessageMeta: m}, true
	}
	return nil, false
}

// byteSize returns a size, for people, such as 512 bytes, 30 KB or 1.5 MB,
// in units of 1024
func byteSize(n int) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%d bytes", n)
	case n < 1024*1024:
		return fmt.Sprintf("%d KB", (n+512)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
}
