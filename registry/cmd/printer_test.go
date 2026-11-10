package cmd

import (
	"bytes"
	"strings"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/registry/schema"
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

	t.Run("ServerThinking", func(t *testing.T) {
		// The server's line that the model is thinking is the printer's own,
		// said once, and replaced, with the text, rather than kept
		p, buf := newTestPrinter(80, 24)
		p.print(llm.StreamLog, schema.ThinkingLog)
		p.print(llm.StreamThinking, "Hmm")
		p.print(llm.StreamText, "Sunny")
		p.End()
		out := buf.String()
		assert.Equal(t, 1, strings.Count(out, thinkingLine))
		assert.NotContains(t, out, "\x1b[2m"+schema.ThinkingLog+"\x1b[0m", "the server's line isn't kept, as it is")
		assert.NotContains(t, out, "Hmm")
		assert.True(t, strings.HasSuffix(out, "\x1b[J[Sunny]\n"), "it's replaced with the text, from the row it started on")
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

func Test_StreamPrinterEdges(t *testing.T) {
	t.Run("TallThenLog", func(t *testing.T) {
		// A log after text which isn't replaced, as it's too tall, is on a
		// line of its own
		p, buf := newTestPrinter(10, 3)
		p.print(llm.StreamText, strings.Repeat("a", 35))
		p.print(llm.StreamLog, "Getting the weather")
		p.End()
		assert.Contains(t, buf.String(), strings.Repeat("a", 35)+"\n\x1b[2mGetting the weather")
	})

	t.Run("Tabs", func(t *testing.T) {
		// Text with tabs isn't replaced, as how far they move isn't known
		p, buf := newTestPrinter(10, 24)
		p.print(llm.StreamText, "\t12345")
		p.End()
		assert.NotContains(t, buf.String(), "\x1b[J")
		assert.Equal(t, "\t12345\n", buf.String())
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

func Test_StreamPrinterProgress(t *testing.T) {
	t.Run("Terminal", func(t *testing.T) {
		// A tool's progress is on a line of its own, after the text before
		// it, which each progress replaces, with its percent, when it's
		// known, and which is cleared before what's next
		p, buf := newTestPrinter(80, 24)
		p.print(llm.StreamText, "Let me check.")
		p.print(llm.StreamProgress, llm.Progress{Progress: 1, Message: "Agent segment is thinking"}.Text())
		p.print(llm.StreamProgress, llm.Progress{Progress: 5, Total: 10, Message: "Segmenting"}.Text())
		p.print(llm.StreamText, "Done.")
		p.End()
		out := buf.String()
		assert.Contains(t, out, "[Let me check.]\n\x1b[2mAgent segment is thinking\x1b[0m\r\x1b[K\x1b[2mSegmenting (50%)\x1b[0m\r\x1b[K")
		assert.True(t, strings.HasSuffix(out, "[Done.]\n"), out)
	})

	t.Run("Long", func(t *testing.T) {
		// Progress fits the width, so it's on one line
		p, buf := newTestPrinter(10, 24)
		p.print(llm.StreamProgress, llm.Progress{Message: "Agent segment is replying"}.Text())
		assert.Contains(t, buf.String(), "\x1b[2mAgent se…\x1b[0m")
	})

	t.Run("NotTerminal", func(t *testing.T) {
		// Progress isn't printed when the output isn't a terminal
		p, buf := newTestPrinter(0, 0)
		p.print(llm.StreamProgress, llm.Progress{Message: "Working"}.Text())
		p.print(llm.StreamText, "Done")
		p.End()
		assert.Equal(t, "Done\n", buf.String())
	})
}
