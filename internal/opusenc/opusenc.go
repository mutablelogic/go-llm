// Package opusenc converts audio, such as speech, to voice notes, which are
// Ogg Opus, with opusenc, of opus-tools, which it runs, such as for the llm
// tools' voice notes
package opusenc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"os/exec"
	"strconv"
	"strings"

	// Packages
	gollm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Converter converts audio to voice notes, with opusenc
type Converter struct {
	path string
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// voiceNoteType is the type of a voice note
	voiceNoteType = "audio/ogg"

	// voiceNoteBitrate is the bitrate of a voice note, in kbit/s, which is
	// plenty for speech
	voiceNoteBitrate = "32"

	// maxVoiceNote is the largest voice note which is kept, which is an
	// hour of speech, or more, at its bitrate
	maxVoiceNote = 64 << 20
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns the converter which runs opusenc, of opus-tools, at the path,
// or the one on the PATH, when it's empty. Returns ErrNotFound when there's
// none.
func New(path string) (*Converter, error) {
	if path == "" {
		found, err := exec.LookPath("opusenc")
		if err != nil {
			return nil, gollm.ErrNotFound.With("opusenc, of opus-tools, isn't installed")
		}
		path = found
	}
	return &Converter{path: path}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Path returns the path of opusenc
func (c *Converter) Path() string {
	return c.path
}

// Convert returns the audio as a voice note, which is Ogg Opus, for speech,
// from WAV, AIFF, FLAC, or raw PCM, as audio/L16, with its rate, and
// channels, or ErrNotImplemented when it's audio of another type
func (c *Converter) Convert(ctx context.Context, audio gollm.Resource) (gollm.Resource, error) {
	args, err := opusencArgs(audio.Type())
	if err != nil {
		return nil, err
	}
	read, err := audio.Read(ctx)
	if err != nil {
		return nil, err
	}
	var encoded bytes.Buffer
	if err := read(ctx, func(r io.Reader) error {
		var stderr bytes.Buffer
		cmd := exec.CommandContext(ctx, c.path, args...)
		cmd.Stdin = r
		cmd.Stdout = &limitWriter{w: &encoded, n: maxVoiceNote}
		cmd.Stderr = &limitWriter{w: &stderr, n: 4096, drop: true}
		if err := cmd.Run(); err != nil {
			if message := strings.TrimSpace(stderr.String()); message != "" {
				return fmt.Errorf("opusenc: %w: %s", err, message)
			}
			return fmt.Errorf("opusenc: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: voiceNoteType, Data: encoded.Bytes()}}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// opusencArgs returns the arguments of opusenc, which reads audio of the
// type from its input, and writes Ogg Opus, for speech, to its output, or
// ErrNotImplemented when it isn't audio opusenc reads
func opusencArgs(contentType string) ([]string, error) {
	args := []string{"--quiet", "--bitrate", voiceNoteBitrate}
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return nil, gollm.ErrNotImplemented.Withf("audio of type %q isn't encoded as a voice note", contentType)
	}
	switch strings.ToLower(mediaType) {
	case "audio/wav", "audio/x-wav", "audio/wave", "audio/vnd.wave", "audio/aiff", "audio/x-aiff", "audio/flac", "audio/x-flac":
	case "audio/l16":
		// Raw PCM, which is 16-bit, and big-endian, with its rate, and
		// channels, which are one, unless they're given
		rate, err := strconv.Atoi(params["rate"])
		if err != nil || rate <= 0 {
			return nil, gollm.ErrNotImplemented.Withf("audio of type %q has no rate", contentType)
		}
		channels := 1
		if c, err := strconv.Atoi(params["channels"]); err == nil && c > 0 {
			channels = c
		}
		args = append(args, "--raw", "--raw-bits", "16", "--raw-rate", strconv.Itoa(rate), "--raw-chan", strconv.Itoa(channels), "--raw-endianness", "1")
	default:
		return nil, gollm.ErrNotImplemented.Withf("audio of type %q isn't encoded as a voice note", contentType)
	}
	return append(args, "-", "-"), nil
}

///////////////////////////////////////////////////////////////////////////////
// LIMIT WRITER

// limitWriter writes at most n bytes, and fails after them, or, when it
// drops them, discards them, so a command's output isn't kept without end
type limitWriter struct {
	w    io.Writer
	n    int64
	drop bool
}

// errTooLarge is the error of a voice note which is larger than is kept
var errTooLarge = errors.New("the voice note is too large")

func (l *limitWriter) Write(p []byte) (int, error) {
	if int64(len(p)) > l.n {
		if !l.drop {
			return 0, errTooLarge
		}
		kept, _ := l.w.Write(p[:l.n])
		l.n -= int64(kept)
		return len(p), nil
	}
	n, err := l.w.Write(p)
	l.n -= int64(n)
	return n, err
}
