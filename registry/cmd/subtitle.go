package cmd

import (
	"fmt"
	"io"
	"strings"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// subtitleWriter writes the segments of a transcript as subtitles, in SRT,
// or WebVTT, format. A cue is the segments, in turn, until one is spoken by
// another speaker, or after a silence, or would make the cue too long, or
// have too much text, to read. A segment isn't split.
type subtitleWriter struct {
	w     io.Writer
	vtt   bool         // WebVTT, rather than SRT
	index int          // The number of cues written
	cue   *subtitleCue // The cue being made, until it's written
}

// subtitleCue is a cue of subtitles, from when to when, who spoke it, when
// the speakers are known, and its text
type subtitleCue struct {
	start, end time.Duration
	speaker    string
	text       []string
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	subtitleGap      = time.Second     // Silence which ends a cue
	subtitleDuration = 7 * time.Second // The longest a cue is, unless it's one segment
	subtitleText     = 84              // The most characters of a cue, unless it's one segment
	subtitleLine     = 42              // The most characters of a line of a cue, unless it's one word
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// newSubtitleWriter returns a writer of subtitles, in WebVTT format when vtt
// is true, or otherwise SRT format
func newSubtitleWriter(w io.Writer, vtt bool) *subtitleWriter {
	return &subtitleWriter{w: w, vtt: vtt}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Add adds the segment to the cue, or writes the cue, and starts another,
// when the segment ends it
func (s *subtitleWriter) Add(segment llm.TranscriptSegment) {
	text := strings.TrimSpace(segment.Text)
	if text == "" {
		return
	}
	if s.cue != nil && s.cue.ends(segment, text) {
		s.flush()
	}
	if s.cue == nil {
		s.cue = &subtitleCue{start: segment.Start, speaker: segment.Speaker}
	}
	s.cue.text = append(s.cue.text, text)
	s.cue.end = max(s.cue.end, segment.End)
}

// Close writes the last cue, and the WebVTT header, when no cue was written
func (s *subtitleWriter) Close() {
	s.flush()
	if s.vtt && s.index == 0 {
		fmt.Fprint(s.w, "WEBVTT\n\n")
	}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// flush writes the cue, with its number in SRT, or after the header in
// WebVTT, and its speaker, when it's known
func (s *subtitleWriter) flush() {
	if s.cue == nil {
		return
	}
	cue := s.cue
	s.cue, s.index = nil, s.index+1
	text := strings.Join(cue.text, " ")
	if s.vtt {
		// The speaker is a voice tag, which isn't shown as text
		lines := wrapText(text, subtitleLine)
		if s.index == 1 {
			fmt.Fprint(s.w, "WEBVTT\n\n")
		}
		if cue.speaker != "" {
			lines[0] = "<v " + cue.speaker + ">" + lines[0]
		}
		fmt.Fprintf(s.w, "%s --> %s\n%s\n\n", subtitleTime(cue.start, '.'), subtitleTime(cue.end, '.'), strings.Join(lines, "\n"))
	} else {
		// The speaker is shown, so it's wrapped with the text
		if cue.speaker != "" {
			text = cue.speaker + ": " + text
		}
		fmt.Fprintf(s.w, "%d\n%s --> %s\n%s\n\n", s.index, subtitleTime(cue.start, ','), subtitleTime(cue.end, ','), strings.Join(wrapText(text, subtitleLine), "\n"))
	}
}

// ends returns true when the segment, with its text, ends the cue: when it's
// spoken by another speaker, or after a silence, or would make the cue too
// long, or have too much text
func (c *subtitleCue) ends(segment llm.TranscriptSegment, text string) bool {
	length := len(text)
	for _, t := range c.text {
		length += len(t) + 1
	}
	return segment.Speaker != c.speaker ||
		segment.Start-c.end >= subtitleGap ||
		segment.End-c.start > subtitleDuration ||
		length > subtitleText
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// subtitleTime returns when in audio, as hours, minutes, seconds and
// milliseconds, with the separator of the milliseconds, which is a comma in
// SRT, and a full stop in WebVTT, such as 00:01:05,300
func subtitleTime(d time.Duration, separator byte) string {
	d = max(d, 0).Round(time.Millisecond)
	return fmt.Sprintf("%02d:%02d:%02d%c%03d", int(d/time.Hour), int(d%time.Hour/time.Minute), int(d%time.Minute/time.Second), separator, int(d%time.Second/time.Millisecond))
}

// wrapText returns the text as lines of words, each of at most the width,
// unless it's one word which is longer
func wrapText(text string, width int) []string {
	var lines []string
	var line string
	for _, word := range strings.Fields(text) {
		switch {
		case line == "":
			line = word
		case len(line)+1+len(word) > width:
			lines, line = append(lines, line), word
		default:
			line += " " + word
		}
	}
	return append(lines, line)
}
