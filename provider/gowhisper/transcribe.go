package gowhisper

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"mime"
	"strings"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/provider/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// INTERFACES

var _ llm.Transcriber = (*Client)(nil)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Transcribe returns the speech in the audio as text, and its language,
// which the model detects, unless it's set, and its segments, when they're
// asked for, and the seconds of audio. When the stream
// function isn't nil, the transcript is streamed, each segment as it's
// transcribed. The request isn't timed out, as audio can take a while, but
// is cancelled with the context.
func (c *Client) Transcribe(ctx context.Context, model llm.Model, audio llm.Resource, stream llm.TranscriberStreamFn, opts ...llm.Opt) (llm.Transcript, llm.Usage, error) {
	return c.transcribe(ctx, "transcribe", model, audio, stream, opts...)
}

// Translate returns the speech in the audio, in any language, as English
// text, as Transcribe does. Returns ErrNotImplemented for an English-only
// model, which doesn't translate.
func (c *Client) Translate(ctx context.Context, model llm.Model, audio llm.Resource, stream llm.TranscriberStreamFn, opts ...llm.Opt) (llm.Transcript, llm.Usage, error) {
	if model != nil {
		if translation := model.Capabilities().Translation; translation != nil && !*translation {
			return llm.Transcript{}, llm.Usage{}, llm.ErrNotImplemented.Withf("model %q can't translate audio", model.Name())
		}
	}
	return c.transcribe(ctx, "translate", model, audio, stream, opts...)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// transcribe returns the speech in the audio, with the endpoint, which is
// transcribe or translate, and streams it to the function, when it isn't
// nil
func (c *Client) transcribe(ctx context.Context, endpoint string, model llm.Model, audio llm.Resource, stream llm.TranscriberStreamFn, opts ...llm.Opt) (llm.Transcript, llm.Usage, error) {
	if model == nil || model.Provider() == nil || model.Provider().Name() != llm.GoWhisper {
		return llm.Transcript{}, llm.Usage{}, llm.ErrBadParameter.With("not a gowhisper model")
	} else if audio == nil {
		return llm.Transcript{}, llm.Usage{}, llm.ErrBadParameter.With("missing audio")
	}

	// The audio, as a file, which go-whisper decodes
	contentType, _, err := mime.ParseMediaType(audio.Type())
	if err != nil || (!strings.HasPrefix(contentType, "audio/") && !strings.HasPrefix(contentType, "video/")) {
		return llm.Transcript{}, llm.Usage{}, llm.ErrBadParameter.Withf("not audio: %q", audio.Type())
	}
	data, err := llm.ReadAll(ctx, audio)
	if err != nil {
		return llm.Transcript{}, llm.Usage{}, err
	} else if len(data) == 0 {
		return llm.Transcript{}, llm.Usage{}, llm.ErrBadParameter.With("audio is empty")
	}

	// The request, with the options
	request := transcribeRequest{
		Model: modelID(model.Name()),
		Audio: types.File{Path: schema.AudioName(audio.Name(), contentType), Body: io.NopCloser(bytes.NewReader(data)), ContentType: contentType},
	}
	options, err := request.apply(model.Name(), endpoint == "translate", opts...)
	if err != nil {
		return llm.Transcript{}, llm.Usage{}, err
	}
	request.Filename = request.Audio.Path

	// Send the request, and receive the transcript, or its segments, as
	// they're transcribed
	req, err := client.NewMultipartRequest(request, types.ContentTypeJSON)
	if err != nil {
		return llm.Transcript{}, llm.Usage{}, err
	}
	var response transcribeResponse
	if stream != nil {
		chunks := &transcribeStream{stream: stream, segments: options.segments}
		if err := c.DoWithContext(ctx, req, nil, client.OptPath(endpoint), client.OptReqHeader("Accept", types.ContentTypeTextStream), client.OptTextStreamCallback(chunks.add), client.OptNoTimeout()); err != nil {
			return llm.Transcript{}, llm.Usage{}, err
		} else if !chunks.done {
			return llm.Transcript{}, llm.Usage{}, llm.ErrServiceUnavailable.With("the transcript ended before it was done")
		}
		response = chunks.response
	} else if err := c.DoWithContext(ctx, req, &response, client.OptPath(endpoint), client.OptNoTimeout()); err != nil {
		return llm.Transcript{}, llm.Usage{}, err
	}

	// The transcript, in its language, which a translation is in English,
	// or the language set, and its segments, when they're asked for
	transcript := llm.Transcript{Text: strings.TrimSpace(response.Text), Language: schema.LanguageCode(response.Language)}
	switch {
	case endpoint == "translate":
		transcript.Language = "en"
	case options.language != "":
		transcript.Language = options.language
	}
	if options.segments {
		for _, segment := range response.Segments {
			if result := segment.transcript(); result.Text != "" {
				transcript.Segments = append(transcript.Segments, result)
			}
		}
	}
	return transcript, llm.Usage{Seconds: uint(math.Ceil(response.Duration))}, nil
}

///////////////////////////////////////////////////////////////////////////////
// STREAM

// transcribeStream collects the events of a streamed transcript into a
// response, which are each segment, as it's transcribed, and the summary of
// the transcript, without its text, which is its segments', or an error. The
// text of each segment is streamed, and the segment, when they're asked for.
type transcribeStream struct {
	stream   llm.TranscriberStreamFn
	segments bool               // Whether segments are streamed
	done     bool               // Whether the transcript is done
	response transcribeResponse // The transcript
}

// add adds an event of the stream. An error event fails the transcript.
func (s *transcribeStream) add(event client.TextStreamEvent) error {
	switch event.Event {
	case eventSegment:
		var segment apiSegment
		if err := event.Json(&segment); err != nil {
			return err
		}
		s.response.Segments = append(s.response.Segments, segment)
		result := segment.transcript()
		if result.Text == "" {
			return nil
		}

		// The text is the segments', separated by a space
		text := result.Text
		if s.response.Text != "" {
			text = " " + text
		}
		s.response.Text += text
		s.stream(text, nil)
		if s.segments {
			s.stream("", &result)
		}
	case eventTranscribeDone:
		var summary transcribeResponse
		if err := event.Json(&summary); err != nil {
			return err
		}
		s.response.Task, s.response.Language, s.response.Duration = summary.Task, summary.Language, summary.Duration
		s.done = true
	case eventTranscribeError:
		var message string
		if err := json.Unmarshal([]byte(event.Data), &message); err != nil {
			message = event.Data
		}
		return errors.New(strings.TrimSpace(message))
	}
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS
