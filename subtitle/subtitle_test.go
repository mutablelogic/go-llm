package subtitle

import (
	"errors"
	"strings"
	"testing"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// seconds returns the seconds as a duration
func seconds(value float64) time.Duration {
	return time.Duration(value * float64(time.Second))
}

// subtitles returns the segments as subtitles, in WebVTT format when vtt is
// true, or otherwise SRT format
func subtitles(vtt bool, segments ...llm.TranscriptSegment) string {
	var b strings.Builder
	format := SRT
	if vtt {
		format = VTT
	}
	s, err := NewWriter(&b, format)
	if err != nil {
		panic(err)
	}
	for _, segment := range segments {
		if err := s.Add(segment); err != nil {
			panic(err)
		}
	}
	if err := s.Close(); err != nil {
		panic(err)
	}
	return b.String()
}

// segment returns a segment, from when to when, in seconds, with its text,
// spoken by the speaker
func segment(start, end float64, speaker, text string) llm.TranscriptSegment {
	return llm.TranscriptSegment{Start: seconds(start), End: seconds(end), Speaker: speaker, Text: text}
}

func Test_Subtitles(t *testing.T) {
	t.Run("SRT", func(t *testing.T) {
		// Segments close together, by a speaker, are a cue, numbered, until
		// a silence
		assert.Equal(t, "1\n00:00:00,000 --> 00:00:03,500\nHello, world. This is a test.\n\n"+
			"2\n00:00:05,000 --> 00:01:05,250\nThe end.\n\n",
			subtitles(false,
				segment(0, 1.3, "", "Hello, world."),
				segment(1.6, 3.5, "", " This is a test."),
				segment(5, 65.25, "", "The end."),
			))
	})

	t.Run("VTT", func(t *testing.T) {
		// After a header, without numbers, with a full stop for the
		// milliseconds, and an hour
		assert.Equal(t, "WEBVTT\n\n00:00:00.000 --> 00:00:01.300\nHello.\n\n01:00:00.000 --> 01:00:01.000\nLater.\n\n",
			subtitles(true, segment(0, 1.3, "", "Hello."), segment(3600, 3601, "", "Later.")))

		// And a header without any cues
		assert.Equal(t, "WEBVTT\n\n", subtitles(true))
		assert.Empty(t, subtitles(false))
	})

	t.Run("Speakers", func(t *testing.T) {
		// A cue ends when the speaker changes, and is labelled with them
		assert.Equal(t, "1\n00:00:00,000 --> 00:00:01,000\nA: Hello.\n\n2\n00:00:01,000 --> 00:00:02,000\nB: Hi.\n\n",
			subtitles(false, segment(0, 1, "A", "Hello."), segment(1, 2, "B", "Hi.")))
		assert.Equal(t, "WEBVTT\n\n00:00:00.000 --> 00:00:01.000\n<v A>Hello.\n\n00:00:01.000 --> 00:00:02.000\n<v B>Hi.\n\n",
			subtitles(true, segment(0, 1, "A", "Hello."), segment(1, 2, "B", "Hi.")))

		// And the label is wrapped with the text, in SRT, where it's shown
		for _, line := range strings.Split(subtitles(false, segment(0, 3, "speaker_1", "The quick brown fox jumps over the lazy dog.")), "\n") {
			assert.LessOrEqual(t, len(line), subtitleLine, line)
		}
	})

	t.Run("Length", func(t *testing.T) {
		// A cue ends before it's longer than seven seconds, or has too much
		// text, but a segment isn't split
		long := subtitles(false, segment(0, 4, "", "One."), segment(4, 7.5, "", "Two."), segment(8, 20, "", "Three, which is long."))
		assert.Equal(t, 3, strings.Count(long, " --> "))
		text := strings.Repeat("word ", 9)
		assert.Equal(t, 2, strings.Count(subtitles(false, segment(0, 1, "", text), segment(1, 2, "", text)), " --> "))

		// And empty segments are skipped
		assert.Equal(t, 1, strings.Count(subtitles(false, segment(0, 1, "", "One."), segment(1, 2, "", " ")), " --> "))
	})

	t.Run("Wrap", func(t *testing.T) {
		// A cue's text is lines of at most 42 characters, unless a word is
		// longer
		assert.Equal(t, []string{"The quick brown fox jumps over the lazy", "dog, and then the quick brown fox sleeps."},
			wrapText("The quick brown fox jumps over the lazy dog, and then the quick brown fox sleeps.", subtitleLine))
		assert.Equal(t, []string{strings.Repeat("a", 50), "b"}, wrapText(strings.Repeat("a", 50)+" b", subtitleLine))
	})
}

func Test_Write(t *testing.T) {
	// A transcript's segments, at once, in a format, which must be SRT, or
	// VTT
	var b strings.Builder
	assert.NoError(t, Write(&b, SRT, []llm.TranscriptSegment{segment(0, 1, "", "Hello.")}))
	assert.Equal(t, "1\n00:00:00,000 --> 00:00:01,000\nHello.\n\n", b.String())
	assert.Error(t, Write(&b, "ass", nil))

	// A write which fails is returned, and nothing more is written
	failed := errors.New("write failed")
	w := &failingWriter{err: failed}
	assert.ErrorIs(t, Write(w, SRT, []llm.TranscriptSegment{segment(0, 1, "", "Hello."), segment(5, 6, "", "Goodbye.")}), failed)
	assert.Equal(t, 1, w.writes)
	w = &failingWriter{err: failed}
	assert.ErrorIs(t, Write(w, VTT, nil), failed)

	// And from Add, once a cue can't be written, and from Close
	w = &failingWriter{err: failed}
	s, err := NewWriter(w, SRT)
	require.NoError(t, err)
	assert.NoError(t, s.Add(segment(0, 1, "", "Hello.")))
	assert.ErrorIs(t, s.Add(segment(5, 6, "", "Goodbye.")), failed)
	assert.ErrorIs(t, s.Add(segment(10, 11, "", "Again.")), failed)
	assert.ErrorIs(t, s.Close(), failed)
	assert.Equal(t, 1, w.writes)
}

// failingWriter is a writer whose writes fail, counting them
type failingWriter struct {
	err    error
	writes int
}

func (w *failingWriter) Write([]byte) (int, error) {
	w.writes++
	return 0, w.err
}
