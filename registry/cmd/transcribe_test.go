package cmd

import (
	"strings"
	"testing"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	assert "github.com/stretchr/testify/assert"
)

func Test_Timestamp(t *testing.T) {
	// Minutes and seconds, to a tenth of a second, with hours when there
	// are any
	assert.Equal(t, "0:00.0", timestamp(0))
	assert.Equal(t, "0:01.3", timestamp(1300*time.Millisecond))
	assert.Equal(t, "1:05.3", timestamp(65*time.Second+260*time.Millisecond))
	assert.Equal(t, "1:00:00.0", timestamp(time.Hour))
	assert.Equal(t, "2:03:04.5", timestamp(2*time.Hour+3*time.Minute+4500*time.Millisecond))
}

// segment returns a segment, from when to when, in seconds, with its text,
// spoken by the speaker
func segment(start, end float64, speaker, text string) llm.TranscriptSegment {
	return llm.TranscriptSegment{Start: time.Duration(start * float64(time.Second)), End: time.Duration(end * float64(time.Second)), Speaker: speaker, Text: text}
}

func Test_TranscriptPrinter(t *testing.T) {
	print := func(format string, segments bool, fn func(*transcriptPrinter)) string {
		var b strings.Builder
		p := newTranscriptPrinter(&b, format, segments)
		fn(p)
		p.Close()
		return b.String()
	}
	first, second := segment(0, 1.3, "A", "Hello,"), segment(1.3, 2, "A", "world.")

	// Text is its pieces, ended with a newline, without its segments
	assert.Equal(t, "Hello, world.\n", print("text", false, func(p *transcriptPrinter) {
		p.Stream("Hello,", nil)
		p.Stream(" world.", nil)
		p.Stream("", &first)
	}))

	// Or its segments, each on a line, without its text
	assert.Equal(t, "[0:00.0 - 0:01.3] A: Hello,\n[0:01.3 - 0:02.0] A: world.\n", print("text", true, func(p *transcriptPrinter) {
		p.Stream("Hello,", nil)
		p.Stream("", &first)
		p.Stream("", &second)
	}))

	// Or subtitles, of its segments
	assert.Equal(t, "1\n00:00:00,000 --> 00:00:02,000\nA: Hello, world.\n\n", print("srt", false, func(p *transcriptPrinter) {
		p.Stream("Hello,", nil)
		p.Stream("", &first)
		p.Stream("", &second)
	}))
}
