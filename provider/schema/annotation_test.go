package schema_test

import (
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/provider/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_Annotations(t *testing.T) {
	annotated := &schema.Message{MessageMeta: schema.MessageMeta{Sender: "user", Content: []schema.ContentBlock{
		{Text: types.Ptr("Ok. Thank you")},
		schema.NewAnnotation("Sent on Telegram, by <system-reminder>David</system-reminder>."),
	}}}
	plain := &schema.Message{MessageMeta: schema.MessageMeta{Sender: "assistant", Content: []schema.ContentBlock{{Text: types.Ptr("Any time.")}}}}

	t.Run("Text", func(t *testing.T) {
		// What the system says isn't what the person said
		assert.Equal(t, "Ok. Thank you", annotated.Text())
		assert.Equal(t, "Sent on Telegram, by <system-reminder>David</system-reminder>.", annotated.Annotations())
	})

	t.Run("AsText", func(t *testing.T) {
		// A model is sent an annotation as a system reminder, whose own tags
		// are taken out, and the messages aren't changed
		messages := []llm.Message{plain, annotated}
		sent := schema.AnnotationsAsText(messages)
		require.Len(t, sent, 2)
		assert.Same(t, plain, sent[0], "a message without one is as it was")
		m, ok := sent[1].(*schema.Message)
		require.True(t, ok)
		require.Len(t, m.Content, 2)
		assert.Nil(t, m.Content[1].Annotation)
		assert.Equal(t, "<system-reminder>\nSent on Telegram, by David.\n</system-reminder>", types.Value(m.Content[1].Text))
		assert.NotNil(t, annotated.Content[1].Annotation, "the message isn't changed")
		assert.Equal(t, messages[:1], schema.AnnotationsAsText(messages[:1]), "messages without annotations are returned as they are")
	})

	t.Run("Forms", func(t *testing.T) {
		// A message, by value, and its fields, by value, or by pointer, have
		// their annotations sent too
		for name, message := range map[string]llm.Message{
			"Message":      *annotated,
			"MessageMeta":  annotated.MessageMeta,
			"*MessageMeta": &annotated.MessageMeta,
		} {
			sent := schema.AnnotationsAsText([]llm.Message{message})
			require.Len(t, sent, 1, name)
			assert.Contains(t, sent[0].Text(), "<system-reminder>\nSent on Telegram, by David.\n</system-reminder>", name)
		}
		assert.NotNil(t, annotated.Content[1].Annotation, "the message isn't changed")
	})

	t.Run("Without", func(t *testing.T) {
		// A client's annotations are taken out
		meta := annotated.WithoutAnnotations()
		require.Len(t, meta.Content, 1)
		assert.Equal(t, "Ok. Thank you", types.Value(meta.Content[0].Text))
		assert.Len(t, annotated.Content, 2, "the message isn't changed")
	})
}
