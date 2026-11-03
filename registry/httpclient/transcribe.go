package httpclient

import (
	"context"
	"fmt"
	"strings"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/registry/schema"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Transcribe returns the speech in the audio of the request, as text, in the
// language it was spoken in, and its segments, when they're asked for, from
// its model, with the model's canonical name, and the seconds and tokens
// used. A model is its name, or the provider and name separated by ":", such
// as "openai:whisper-1". When the stream function isn't nil, the transcript
// is streamed, and the function is called with each piece of its text, and
// each segment, when they're asked for. The request isn't timed out, as
// audio can take a while, but is cancelled with the context.
func (c *Client) Transcribe(ctx context.Context, req schema.TranscribeRequest, stream llm.TranscriberStreamFn) (*schema.TranscribeResponse, error) {
	return c.transcribe(ctx, "transcribe", req, stream)
}

// Translate returns the speech in the audio of the request, in any
// language, as English text, as Transcribe does
func (c *Client) Translate(ctx context.Context, req schema.TranscribeRequest, stream llm.TranscriberStreamFn) (*schema.TranscribeResponse, error) {
	return c.transcribe(ctx, "translate", req, stream)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// transcribe sends the request to the endpoint, which is transcribe or
// translate, and streams the transcript to the function, when it isn't nil
func (c *Client) transcribe(ctx context.Context, endpoint string, req schema.TranscribeRequest, stream llm.TranscriberStreamFn) (*schema.TranscribeResponse, error) {
	if strings.TrimSpace(req.Model) == "" {
		return nil, llm.ErrBadParameter.With("missing model")
	} else if _, err := req.Resource(); err != nil {
		return nil, err
	}
	body, err := client.NewJSONRequest(req)
	if err != nil {
		return nil, err
	}

	// Without streaming, the transcript
	if stream == nil {
		var response schema.TranscribeResponse
		if err := c.DoWithContext(ctx, body, &response, client.OptPath("llm", endpoint), client.OptNoTimeout()); err != nil {
			return nil, err
		}
		return &response, nil
	}

	// With streaming, the pieces of the text, and the segments, and then the
	// transcript, or the error
	return doStream[schema.TranscribeResponse](ctx, c, body, func(event client.TextStreamEvent) error {
		switch event.Event {
		case schema.EventText:
			var data schema.StreamEvent
			if err := event.Json(&data); err != nil {
				return fmt.Errorf("malformed %s event: %w", event.Event, err)
			}
			stream(data.Text, nil)
		case schema.EventSegment:
			var data llm.TranscriptSegment
			if err := event.Json(&data); err != nil {
				return fmt.Errorf("malformed %s event: %w", event.Event, err)
			}
			stream("", &data)
		}
		return nil
	}, "llm", endpoint)
}
