package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	// Packages
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	tui "github.com/mutablelogic/go-llm/pkg/tui"
	assert "github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChannelPayload(t *testing.T) {
	assert := assert.New(t)

	payload, err := channelPayload("hello")
	if !assert.NoError(err) {
		return
	}

	var req schema.SessionChannelRequest
	if !assert.NoError(json.Unmarshal(payload, &req)) {
		return
	}

	assert.Equal("hello", req.Text)
}

func TestFormatChannelFrame(t *testing.T) {
	assert := assert.New(t)

	formatted, err := formatChannelFrame(json.RawMessage(`{"id":"11111111-1111-1111-1111-111111111111"}`))
	if !assert.NoError(err) {
		return
	}

	assert.Contains(formatted, "\n")
	assert.Contains(formatted, `"id": "11111111-1111-1111-1111-111111111111"`)
}

func TestFormatChannelFrameRejectsInvalidJSON(t *testing.T) {
	assert := assert.New(t)

	_, err := formatChannelFrame(json.RawMessage(`{"id":`))
	assert.Error(err)
}

func TestChannelFrameMarkdown(t *testing.T) {
	assert := assert.New(t)

	markdown, err := channelFrameMarkdown("session", json.RawMessage(`{"id":"11111111-1111-1111-1111-111111111111"}`))
	if !assert.NoError(err) {
		return
	}

	assert.Contains(markdown, "### Session")
	assert.Contains(markdown, "```json")
	assert.Contains(markdown, `"id": "11111111-1111-1111-1111-111111111111"`)
}

func TestChannelModelApplyDeltaUpdatesViewportInPlace(t *testing.T) {
	m := &channelModel{
		viewport: tui.NewViewport(tui.SetWidth(60), tui.SetHeight(6)),
		turn:     1,
		live:     make(map[string]struct{}),
	}

	require.NoError(t, m.applyDelta(schema.StreamDelta{Role: "assistant", Text: "hel"}))
	require.NoError(t, m.applyDelta(schema.StreamDelta{Role: "assistant", Text: "lo"}))

	view := m.viewport.View()
	assert.Contains(t, view, "Assistant")
	assert.Contains(t, view, "hello")
	assert.NotContains(t, view, channelCursorGlyph)
	assert.Equal(t, "streaming assistant", m.status)
}

func TestChannelResponseMarkdownRendersTextBlocks(t *testing.T) {
	text := "hello there"
	markdown, err := channelResponseMarkdown(schema.ChatResponse{
		CompletionResponse: schema.CompletionResponse{
			Role:    "assistant",
			Content: []schema.ContentBlock{{Text: &text}},
			Result:  schema.ResultStop,
		},
	})
	require.NoError(t, err)
	assert.Contains(t, markdown, "### Assistant")
	assert.Contains(t, markdown, text)
	assert.Contains(t, markdown, "_Result: stop_")
}

func TestChannelModelApplyResponseReplacesLiveAssistantSection(t *testing.T) {
	text := "hello"
	m := &channelModel{
		viewport: tui.NewViewport(tui.SetWidth(60), tui.SetHeight(10)),
		turn:     1,
		live:     make(map[string]struct{}),
	}

	require.NoError(t, m.applyDelta(schema.StreamDelta{Role: "assistant", Text: "hel"}))
	require.NoError(t, m.applyDelta(schema.StreamDelta{Role: "assistant", Text: "lo"}))
	require.NoError(t, m.applyResponse(schema.ChatResponse{
		CompletionResponse: schema.CompletionResponse{
			Role:    "assistant",
			Content: []schema.ContentBlock{{Text: &text}},
			Result:  schema.ResultStop,
		},
	}))

	view := m.viewport.View()
	assert.Equal(t, 1, strings.Count(view, "Assistant"))
	assert.Contains(t, view, "hello")
	assert.Equal(t, "complete", m.status)
}

func TestChannelModelResponseLeavesViewportContentStable(t *testing.T) {
	text := "hello"
	m := &channelModel{
		viewport: tui.NewViewport(tui.SetWidth(60), tui.SetHeight(10)),
		turn:     1,
		live:     make(map[string]struct{}),
	}

	require.NoError(t, m.applyDelta(schema.StreamDelta{Role: "assistant", Text: "hel"}))
	require.NoError(t, m.applyResponse(schema.ChatResponse{
		CompletionResponse: schema.CompletionResponse{
			Role:    "assistant",
			Content: []schema.ContentBlock{{Text: &text}},
			Result:  schema.ResultStop,
		},
	}))

	assert.NotContains(t, m.viewport.View(), channelCursorGlyph)
	assert.False(t, m.streaming)
}

func TestChannelModelViewShowsCursorInStatus(t *testing.T) {
	m := &channelModel{
		viewport:      tui.NewViewport(tui.SetWidth(60), tui.SetHeight(10)),
		streaming:     true,
		spinnerFrame:  1,
		activeRole:    schema.RoleAssistant,
		promptVisible: true,
		status:        "streaming assistant",
	}

	view := m.View()
	assert.Contains(t, view, "⠙ replying")
}

func TestChannelModelViewShowsPromptCursorWhenIdle(t *testing.T) {
	m := &channelModel{
		viewport:      tui.NewViewport(tui.SetWidth(60), tui.SetHeight(10)),
		promptVisible: true,
		status:        "connected",
	}

	view := m.View()
	assert.Contains(t, view, "> "+channelCursorGlyph)
	assert.Contains(t, view, "connected")
}

func TestChannelModelStartStreamingShowsStatusCursorBeforeDelta(t *testing.T) {
	m := &channelModel{
		viewport:      tui.NewViewport(tui.SetWidth(60), tui.SetHeight(10)),
		promptVisible: true,
		status:        "sent",
	}

	require.NoError(t, m.startStreaming())

	assert.True(t, m.streaming)
	assert.Contains(t, m.View(), "⠋ busy")
	assert.NotContains(t, m.viewport.View(), channelCursorGlyph)
}

func TestChannelModelTickCursorTogglesStatusWithoutViewportCursor(t *testing.T) {
	m := &channelModel{
		viewport:      tui.NewViewport(tui.SetWidth(60), tui.SetHeight(10)),
		promptVisible: true,
		streaming:     true,
		status:        "sent",
	}

	require.NoError(t, m.tickCursor())
	assert.False(t, m.promptVisible)
	assert.Contains(t, m.View(), "⠙ busy")

	require.NoError(t, m.tickCursor())
	assert.True(t, m.promptVisible)
	assert.Contains(t, m.View(), "⠹ busy")
}

func TestChannelModelSpinnerGlyph(t *testing.T) {
	m := &channelModel{spinnerFrame: 3}
	assert.Equal(t, "⠸", m.spinnerGlyph())
}

func TestChannelModelStreamingLabel(t *testing.T) {
	tests := []struct {
		name  string
		role  string
		label string
	}{
		{name: "busy by default", role: "", label: "busy"},
		{name: "thinking", role: schema.RoleThinking, label: "thinking"},
		{name: "tool", role: schema.RoleTool, label: "calling tool"},
		{name: "assistant", role: schema.RoleAssistant, label: "replying"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &channelModel{activeRole: tt.role}
			assert.Equal(t, tt.label, m.streamingLabel())
		})
	}
}

func TestTrimLastRune(t *testing.T) {
	assert := assert.New(t)

	assert.Equal("hell", trimLastRune("hello"))
	assert.Equal("", trimLastRune(""))
	assert.Equal("ab", trimLastRune("ab🙂"))
}
