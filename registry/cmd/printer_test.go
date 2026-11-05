package cmd

import (
	"bytes"
	"strings"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	assert "github.com/stretchr/testify/assert"
)

// newTestPrinter returns a printer to a buffer, on a terminal of the width
// and height, whose markdown is the text in brackets, or not on a terminal,
// when the width is zero
func newTestPrinter(width, height int) (*streamPrinter, *bytes.Buffer) {
	var buf bytes.Buffer
	p := &streamPrinter{w: &buf, width: width, height: height}
	if width > 0 {
		p.render = func(text string) string { return "[" + strings.TrimSpace(text) + "]\n" }
	}
	return p, &buf
}

func Test_StreamPrinter(t *testing.T) {
	t.Run("NotTerminal", func(t *testing.T) {
		// Only the text is printed, as it is, and nothing is replaced
		p, buf := newTestPrinter(0, 0)
		p.print(llm.StreamThinking, "Hmm")
		p.print(llm.StreamText, "Hello ")
		p.print(llm.StreamLog, "Getting the weather")
		p.print(llm.StreamText, "world")
		p.End()
		assert.Equal(t, "Hello world\n", buf.String())
	})

	t.Run("Markdown", func(t *testing.T) {
		// The thinking is shown as "Thinking…", and the text is printed as
		// it is, and then replaced, from the row it started on, with the
		// text rendered as markdown
		p, buf := newTestPrinter(80, 24)
		p.print(llm.StreamThinking, "The user says weather")
		p.print(llm.StreamThinking, " again")
		p.print(llm.StreamText, "**Sunny**\nand warm")
		p.End()
		out := buf.String()
		assert.NotContains(t, out, "The user says")
		assert.Equal(t, 1, strings.Count(out, thinkingLine))
		replaced := strings.Index(out, "\x1b[J")
		assert.Greater(t, replaced, strings.Index(out, "**Sunny**"))
		// Thinking…, a blank line, and two lines of text, are four rows
		assert.Contains(t, out, "\r\x1b[3A\x1b[J[**Sunny**\nand warm]\n")
		assert.True(t, strings.HasSuffix(out, "]\n"))
	})

	t.Run("Logs", func(t *testing.T) {
		// What tools log is kept, after the text before it, which is
		// rendered before it's printed, and the text after it is rendered
		// on its own
		p, buf := newTestPrinter(80, 24)
		p.print(llm.StreamText, "Let me check.")
		p.print(llm.StreamLog, "Getting the weather")
		p.print(llm.StreamText, "It's sunny.")
		p.End()
		out := buf.String()
		first := strings.Index(out, "[Let me check.]")
		logged := strings.Index(out, "Getting the weather")
		second := strings.Index(out, "[It's sunny.]")
		assert.True(t, first >= 0 && logged > first && second > logged, out)
		assert.Equal(t, 2, strings.Count(out, "\x1b[J"))
	})

	t.Run("Tall", func(t *testing.T) {
		// Text which is as tall as the terminal isn't replaced, as what's
		// scrolled off it can't be
		p, buf := newTestPrinter(10, 3)
		p.print(llm.StreamText, strings.Repeat("a", 35))
		p.End()
		assert.NotContains(t, buf.String(), "\x1b[J")
		assert.Equal(t, strings.Repeat("a", 35)+"\n", buf.String())
	})
}

func Test_RowsOf(t *testing.T) {
	// Lines which are longer than the width wrap, and escapes have no width
	assert.Equal(t, 1, rowsOf("hello", 10))
	assert.Equal(t, 2, rowsOf("hello\n", 10))
	assert.Equal(t, 1, rowsOf(strings.Repeat("a", 10), 10))
	assert.Equal(t, 2, rowsOf(strings.Repeat("a", 11), 10))
	assert.Equal(t, 3, rowsOf("\x1b[2mThinking…\x1b[0m\n\nhi", 10))
}
