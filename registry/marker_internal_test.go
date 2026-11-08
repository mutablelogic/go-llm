package registry

import (
	"strings"
	"testing"
	"unicode/utf8"

	// Packages
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
)

func Test_CompactTools(t *testing.T) {
	// The compact agent has the session's memory tools, and no others
	memory := []string{"memory"}
	assert.Equal(t, []string{"memory:*"}, compactTools([]string{"*"}, memory))
	assert.Equal(t, []string{"memory:*"}, compactTools([]string{"weatherapi:*", "memory:*"}, memory))
	assert.Equal(t, []string{"memory:forget", "memory:remember"}, compactTools([]string{"memory:remember", "memory:forget", "newsapi:headlines"}, memory))
	assert.Empty(t, compactTools([]string{"weatherapi:*"}, memory))
	assert.Empty(t, compactTools(nil, memory))
	assert.Empty(t, compactTools([]string{"*"}, nil), "without namespaces, none")
}

func Test_Transcript(t *testing.T) {
	text := func(role, value string) providerschema.Message {
		return providerschema.Message{MessageMeta: providerschema.MessageMeta{Sender: role, Content: []providerschema.ContentBlock{{Text: types.Ptr(value)}}}}
	}
	messages := []providerschema.Message{
		text("user", "What's the weather?"),
		{MessageMeta: providerschema.MessageMeta{Sender: "assistant", Content: []providerschema.ContentBlock{{Thinking: types.Ptr("Hmm")}, {ToolCall: &providerschema.ToolCall{Name: "weatherapi:current"}}}}},
		{MessageMeta: providerschema.MessageMeta{Sender: "tool", Content: []providerschema.ContentBlock{{ToolResult: &providerschema.ToolResult{Name: "weatherapi:current", Content: []byte(`{"temp_c":4,"details":"` + strings.Repeat("x", 600) + `"}`)}}}}},
		text("assistant", "It's 4°C"),
		{MessageMeta: providerschema.MessageMeta{Sender: "context"}, Marker: &providerschema.Marker{}},
	}

	// Each message, with who sent it, what's attached, the tools called, the
	// start of their results, and no thinking, or markers
	got := transcript(messages, 10_000)
	assert.Contains(t, got, "[user] What's the weather?")
	assert.Contains(t, got, "[assistant] (called weatherapi:current)")
	assert.Contains(t, got, `[tool] (weatherapi:current returned {"temp_c":4,`)
	assert.Contains(t, got, "…)")
	assert.Contains(t, got, "[assistant] It's 4°C")
	assert.NotContains(t, got, "Hmm")
	assert.NotContains(t, got, "[context]")

	// The latest messages, which fit, when there are too many, and the end of
	// the latest, when it alone is too long
	got = transcript(messages, 30)
	assert.True(t, strings.HasPrefix(got, "(earlier messages are left out)"), got)
	assert.Contains(t, got, "It's 4°C")
	assert.NotContains(t, got, "weather?")
	got = transcript([]providerschema.Message{text("user", strings.Repeat("a", 100)+"END")}, 20)
	assert.True(t, strings.HasSuffix(got, "END"), got)

	// at most the limit, in bytes, from the start of a character
	got = transcript([]providerschema.Message{text("user", strings.Repeat("文", 100))}, 20)
	assert.LessOrEqual(t, len(got), 20)
	assert.True(t, utf8.ValidString(got), got)
	assert.Equal(t, strings.Repeat("文", 6), got)
}
