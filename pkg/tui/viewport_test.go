package tui

import (
	"strings"
	"testing"

	assert "github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestViewportSetContentAndScroll(t *testing.T) {
	assert := assert.New(t)

	v := NewViewport(SetWidth(40), SetHeight(2))
	require.NoError(t, v.SetContent("- one\n- two\n- three\n- four"))

	top := v.View()
	assert.Contains(top, "one")
	assert.NotContains(top, "four")

	v.GotoBottom()
	bottom := v.View()
	assert.Contains(bottom, "four")
	assert.NotEqual(top, bottom)
}

func TestViewportAppendKeepsBottomWhenAlreadyFollowing(t *testing.T) {
	v := NewViewport(SetWidth(40), SetHeight(2))
	require.NoError(t, v.SetContent("- one\n- two"))
	v.GotoBottom()

	require.NoError(t, v.Append("\n- three"))

	view := v.View()
	assert.Contains(t, view, "three")
}

func TestViewportResizeRewrapsContent(t *testing.T) {
	v := NewViewport(SetWidth(40), SetHeight(4))
	require.NoError(t, v.SetContent("A paragraph with enough text to wrap when the viewport gets narrower."))

	wide := v.View()
	require.NoError(t, v.Resize(20, 4))
	narrow := v.View()

	assert.NotEqual(t, wide, narrow)
	assert.GreaterOrEqual(t, strings.Count(narrow, "\n"), strings.Count(wide, "\n"))
}

func TestViewportSetSectionUpdatesInPlace(t *testing.T) {
	v := NewViewport(SetWidth(60), SetHeight(6))
	require.NoError(t, v.Append("### User\n\nhello"))
	require.NoError(t, v.SetSection("assistant", "### Assistant\n\nhe"))
	require.NoError(t, v.Append("### User\n\nnext"))
	require.NoError(t, v.SetSection("assistant", "### Assistant\n\nhello"))

	view := v.View()
	assert.Contains(t, view, "hello")
	assert.Contains(t, view, "next")
	assert.Less(t, strings.Index(view, "Assistant"), strings.Index(view, "next"))
}

func TestViewportAppendSectionAccumulatesContent(t *testing.T) {
	v := NewViewport(SetWidth(60), SetHeight(4))
	require.NoError(t, v.SetSection("thinking", "### Thinking\n\nI should "))
	require.NoError(t, v.AppendSection("thinking", "answer briefly."))

	view := v.View()
	assert.Contains(t, view, "Thinking")
	assert.Contains(t, view, "I should answer")
	assert.Contains(t, view, "briefly.")
}

func TestViewportCursorCanBlinkOnSection(t *testing.T) {
	v := NewViewport(SetWidth(60), SetHeight(4))
	require.NoError(t, v.SetSection("assistant", "### Assistant\n\nhello"))
	require.NoError(t, v.SetCursor("assistant", "|"))

	visible := v.View()
	assert.Contains(t, visible, "hello|")

	require.NoError(t, v.SetCursorVisible(false))
	hidden := v.View()
	assert.Contains(t, hidden, "hello")
	assert.NotContains(t, hidden, "hello|")
}
