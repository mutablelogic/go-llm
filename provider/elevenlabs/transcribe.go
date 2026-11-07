package elevenlabs

import (
	"context"
	"io"
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
// which is detected, unless it's set, and its segments, and their speakers,
// when they're asked for, and the seconds of audio. ElevenLabs doesn't
// stream a transcript, so the stream function, when it isn't nil, is called
// once it's transcribed. The request isn't timed out, as audio can take a
// while, but is cancelled with the context.
func (c *Client) Transcribe(ctx context.Context, model llm.Model, audio llm.Resource, stream llm.TranscriberStreamFn, opts ...llm.Opt) (llm.Transcript, llm.Usage, error) {
	if model == nil || model.Provider() == nil || model.Provider().Name() != llm.ElevenLabs {
		return llm.Transcript{}, llm.Usage{}, llm.ErrBadParameter.With("not an elevenlabs model")
	} else if audio == nil {
		return llm.Transcript{}, llm.Usage{}, llm.ErrBadParameter.With("missing audio")
	}

	// The audio, as a file
	contentType, _, err := mime.ParseMediaType(audio.Type())
	if err != nil || (!strings.HasPrefix(contentType, "audio/") && !strings.HasPrefix(contentType, "video/")) {
		return llm.Transcript{}, llm.Usage{}, llm.ErrBadParameter.Withf("not audio: %q", audio.Type())
	}
	if n, ok := llm.ContentLength(audio); ok && n == 0 {
		return llm.Transcript{}, llm.Usage{}, llm.ErrBadParameter.With("audio is empty")
	}

	// The request, with the options
	request := transcribeRequest{Model: model.Name()}
	options, err := request.apply(opts...)
	if err != nil {
		return llm.Transcript{}, llm.Usage{}, err
	}

	// Send the request, with the audio, as it's read, so it isn't held in
	// memory, and receive the transcript
	read, err := audio.Read(ctx)
	if err != nil {
		return llm.Transcript{}, llm.Usage{}, err
	}
	var response transcribeResponse
	if err := read(ctx, func(r io.Reader) error {
		request.File = types.File{Path: schema.AudioName(audio.Name(), contentType), Body: io.NopCloser(r), ContentType: contentType}
		req, err := client.NewStreamingMultipartRequest(request, types.ContentTypeJSON)
		if err != nil {
			return err
		}
		return c.DoWithContext(ctx, req, &response, client.OptPath("speech-to-text"), client.OptNoTimeout())
	}); err != nil {
		return llm.Transcript{}, llm.Usage{}, err
	}

	// The transcript, in its language, or the language set, and its
	// segments, when they're asked for
	transcript := llm.Transcript{Text: strings.TrimSpace(response.Text), Language: response.language()}
	if transcript.Language == "" {
		transcript.Language = options.language
	}
	if options.segments || options.speakers {
		transcript.Segments = response.segments()
	}

	// The transcript isn't streamed, so it's streamed once it's transcribed
	if stream != nil {
		stream(transcript.Text, nil)
		for i := range transcript.Segments {
			stream("", &transcript.Segments[i])
		}
	}
	return transcript, response.usage(), nil
}

// Translate returns ErrNotImplemented, as ElevenLabs doesn't translate
// speech
func (c *Client) Translate(context.Context, llm.Model, llm.Resource, llm.TranscriberStreamFn, ...llm.Opt) (llm.Transcript, llm.Usage, error) {
	return llm.Transcript{}, llm.Usage{}, llm.ErrNotImplemented.With("elevenlabs doesn't translate audio")
}
