// Package subtitle writes the segments of a transcript as subtitles, in SRT
// or WebVTT format, whose cues are the segments, in turn, until one is
// spoken by another speaker, or after a silence, or would make the cue too
// long, or have too much text, to read
package subtitle

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

// Format is the format of subtitles
type Format string

// Writer writes the segments of a transcript as subtitles, in SRT, or
// WebVTT, format, as they're added, so they can be written as they're
// transcribed. A cue is the segments, in turn, until one is spoken by
// another speaker, or after a silence, or would make the cue too long, or
// have too much text, to read. A segment isn't split. Once a write fails,
// nothing more is written, and Add and Close return its error.
type Writer struct {
	w     io.Writer
	err   error        // The first error writing, which is returned from then on
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
	SRT Format = "srt" // SubRip
	VTT Format = "vtt" // WebVTT
)

const (
	subtitleGap      = time.Second     // Silence which ends a cue
	subtitleDuration = 7 * time.Second // The longest a cue is, unless it's one segment
	subtitleText     = 84              // The most characters of a cue, unless it's one segment
	subtitleLine     = 42              // The most characters of a line of a cue, unless it's one word
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewWriter returns a writer of subtitles, in the format, which is SRT, or
// VTT, or an error for another format
func NewWriter(w io.Writer, format Format) (*Writer, error) {
	switch format {
	case SRT, VTT:
		return &Writer{w: w, vtt: format == VTT}, nil
	default:
		return nil, llm.ErrBadParameter.Withf("invalid subtitle format %q", format)
	}
}

// Write writes the segments of a transcript as subtitles, in the format,
// which is SRT, or VTT, or returns an error for another format
func Write(w io.Writer, format Format, segments []llm.TranscriptSegment) error {
	writer, err := NewWriter(w, format)
	if err != nil {
		return err
	}
	for _, segment := range segments {
		if err := writer.Add(segment); err != nil {
			return err
		}
	}
	return writer.Close()
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Add adds the segment to the cue, or writes the cue, and starts another,
// when the segment ends it, returning an error when a cue can't be written
func (s *Writer) Add(segment llm.TranscriptSegment) error {
	if s.err != nil {
		return s.err
	}
	text := strings.TrimSpace(segment.Text)
	if text == "" {
		return nil
	}
	if s.cue != nil && s.cue.ends(segment, text) {
		s.flush()
	}
	if s.cue == nil {
		s.cue = &subtitleCue{start: segment.Start, speaker: segment.Speaker}
	}
	s.cue.text = append(s.cue.text, text)
	s.cue.end = max(s.cue.end, segment.End)
	return s.err
}

// Close writes the last cue, and the WebVTT header, when no cue was written,
// returning the first error writing the subtitles
func (s *Writer) Close() error {
	s.flush()
	if s.vtt && s.index == 0 {
		s.printf("WEBVTT\n\n")
	}
	return s.err
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// flush writes the cue, with its number in SRT, or after the header in
// WebVTT, and its speaker, when it's known
func (s *Writer) flush() {
	if s.cue == nil || s.err != nil {
		return
	}
	cue := s.cue
	s.cue, s.index = nil, s.index+1
	text := strings.Join(cue.text, " ")
	if s.vtt {
		// The speaker is a voice tag, which isn't shown as text
		lines := wrapText(text, subtitleLine)
		if s.index == 1 {
			s.printf("WEBVTT\n\n")
		}
		if cue.speaker != "" {
			lines[0] = "<v " + cue.speaker + ">" + lines[0]
		}
		s.printf("%s --> %s\n%s\n\n", subtitleTime(cue.start, '.'), subtitleTime(cue.end, '.'), strings.Join(lines, "\n"))
	} else {
		// The speaker is shown, so it's wrapped with the text
		if cue.speaker != "" {
			text = cue.speaker + ": " + text
		}
		s.printf("%d\n%s --> %s\n%s\n\n", s.index, subtitleTime(cue.start, ','), subtitleTime(cue.end, ','), strings.Join(wrapText(text, subtitleLine), "\n"))
	}
}

// printf writes to the writer, unless a write has failed, keeping the first
// error
func (s *Writer) printf(format string, args ...any) {
	if s.err == nil {
		_, s.err = fmt.Fprintf(s.w, format, args...)
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
