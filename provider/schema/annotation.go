package schema

import (
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	reminder "github.com/mutablelogic/go-llm/internal/reminder"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC FUNCTIONS

// NewAnnotation returns a block of what the system says of a message, for
// the model, rather than the person, such as where it was sent
func NewAnnotation(text string) ContentBlock {
	return ContentBlock{Annotation: types.Ptr(text)}
}

// AnnotationsAsText returns the messages, with each annotation as text, in
// a system reminder, which is how a model is sent it, as providers have no
// annotations of their own. Messages without annotations are returned as
// they are, and others are copied, so the messages aren't changed.
func AnnotationsAsText(messages []llm.Message) []llm.Message {
	var result []llm.Message
	for i, message := range messages {
		meta, ok := metaOf(message)
		if !ok || !meta.hasAnnotation() {
			if result != nil {
				result = append(result, message)
			}
			continue
		}
		if result == nil {
			result = append(make([]llm.Message, 0, len(messages)), messages[:i]...)
		}
		copied := Message{MessageMeta: meta}
		if m, ok := message.(*Message); ok {
			copied = *m
		} else if m, ok := message.(Message); ok {
			copied = m
		}
		copied.Content = make([]ContentBlock, len(meta.Content))
		for j, block := range meta.Content {
			if block.Annotation != nil {
				block = ContentBlock{Text: types.Ptr("<" + reminder.Tag + ">\n" + reminder.Strip(*block.Annotation) + "\n</" + reminder.Tag + ">")}
			}
			copied.Content[j] = block
		}
		result = append(result, &copied)
	}
	if result == nil {
		return messages
	}
	return result
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// WithoutAnnotations returns the message without its annotations, as only
// the server adds them, so one a client sends, such as through the API,
// isn't taken as what the system says
func (m MessageMeta) WithoutAnnotations() MessageMeta {
	if !m.hasAnnotation() {
		return m
	}
	content := make([]ContentBlock, 0, len(m.Content))
	for _, block := range m.Content {
		if block.Annotation == nil {
			content = append(content, block)
		}
	}
	m.Content = content
	return m
}

// Annotations returns the message's annotations, joined with newlines
func (m MessageMeta) Annotations() string {
	var result []string
	for _, block := range m.Content {
		if block.Annotation != nil {
			result = append(result, strings.TrimSpace(*block.Annotation))
		}
	}
	return strings.Join(result, "\n")
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// metaOf returns the message's fields, when it's one of the schema's
// messages, as a message, or its fields, by value, or by pointer
func metaOf(message llm.Message) (MessageMeta, bool) {
	switch m := message.(type) {
	case *Message:
		if m != nil {
			return m.MessageMeta, true
		}
	case Message:
		return m.MessageMeta, true
	case *MessageMeta:
		if m != nil {
			return *m, true
		}
	case MessageMeta:
		return m, true
	}
	return MessageMeta{}, false
}

// hasAnnotation returns true when the message has an annotation
func (m MessageMeta) hasAnnotation() bool {
	for _, block := range m.Content {
		if block.Annotation != nil {
			return true
		}
	}
	return false
}
