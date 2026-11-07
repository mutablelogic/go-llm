package ollama

import (
	"context"
	"io"
	"mime"
	"net/http"
	"strings"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/provider/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// speechRequest is a request to POST /v1/audio/speech, Ollama's
// OpenAI-compatible API, for the text, spoken, in the voice, and in the
// format, which is wav, or pcm. It has no instructions, which Ollama
// rejects.
type speechRequest struct {
	Model          string   `json:"model"`
	Input          string   `json:"input"`
	Voice          string   `json:"voice,omitempty"`
	ResponseFormat string   `json:"response_format,omitempty"`
	Speed          *float64 `json:"speed,omitempty"`
}

// audioResponse is the audio of a response, and its type
type audioResponse struct {
	data        []byte
	contentType string
}

var _ llm.Speaker = (*Client)(nil)

// pcmRate and pcmChannels are the rate, and channels, of the PCM which
// Ollama's speech models return, which is 16-bit little-endian, as
// OpenAI's is
const (
	pcmRate     = 24000
	pcmChannels = 1
)

var _ client.Unmarshaler = (*audioResponse)(nil)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Speak returns the text, spoken by the model, as audio, with Ollama's
// OpenAI-compatible API, whose type is the format it's in, which is wav,
// unless it's pcm. The voice is the model's, by its name, or its default.
// Ollama rejects a format, or speed, it doesn't support, and a voice the
// model hasn't, saying which it has. Returns ErrBadParameter when there's no
// text, or an option isn't supported.
func (c *Client) Speak(ctx context.Context, model llm.Model, text string, opts ...llm.Opt) (llm.Resource, llm.Usage, error) {
	if model == nil || model.Provider() == nil || model.Provider().Name() != llm.Ollama {
		return nil, llm.Usage{}, llm.ErrBadParameter.With("not a model of this provider")
	} else if strings.TrimSpace(text) == "" {
		return nil, llm.Usage{}, llm.ErrBadParameter.With("missing text")
	}
	req := speechRequest{Model: model.Name(), Input: text}
	if err := req.apply(opts...); err != nil {
		return nil, llm.Usage{}, err
	}
	payload, err := client.NewJSONRequestEx(http.MethodPost, req, types.ContentTypeAny)
	if err != nil {
		return nil, llm.Usage{}, err
	}
	var audio audioResponse
	if err := c.DoWithContext(ctx, payload, &audio, client.OptAbsPath(c.openAIPath("audio", "speech")...), client.OptNoTimeout()); err != nil {
		return nil, llm.Usage{}, err
	} else if len(audio.data) == 0 {
		return nil, llm.Usage{}, llm.ErrServiceUnavailable.With("no audio")
	}

	// Raw PCM, which is little-endian, is audio/L16, which is big-endian,
	// with its rate, and channels, and otherwise the audio's type, or wav,
	// when the response's isn't audio
	if req.ResponseFormat == "pcm" {
		return schema.PCM16(audio.data, pcmRate, pcmChannels), llm.Usage{}, nil
	}
	contentType := audio.contentType
	if mediaType, _, err := mime.ParseMediaType(contentType); err != nil || !strings.HasPrefix(mediaType, "audio/") {
		contentType = "audio/wav"
	}
	return &schema.Resource{ResourceMeta: schema.ResourceMeta{ContentType: contentType, Data: audio.data}}, llm.Usage{}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// apply sets the options on the request: the voice, the format, and the
// speed. The instructions, of how the text is spoken, are ignored, as
// Ollama rejects them, and they're only a hint, so the text is still
// spoken. Returns ErrBadParameter for any other option.
func (r *speechRequest) apply(opts ...llm.Opt) error {
	o, err := llm.ApplyOpts(opts...)
	if err != nil {
		return err
	}
	for _, key := range o.Keys() {
		var err error
		switch key {
		case llm.VoiceKey:
			r.Voice, err = llm.GetOpt[string](o, key)
		case llm.AudioFormatKey:
			r.ResponseFormat, err = llm.GetOpt[string](o, key)
		case llm.SpeedKey:
			var speed float64
			if speed, err = llm.GetOpt[float64](o, key); err == nil {
				r.Speed = &speed
			}
		case llm.InstructionsKey:
			// Ignored, as Ollama rejects them
		default:
			err = llm.ErrBadParameter.Withf("option %q isn't supported for speech", key)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// openAIPath returns the segments of the path of Ollama's OpenAI-compatible
// API, which is /v1, beside its own API, which is /api, under the same
// prefix, such as /ollama/v1 for /ollama/api, followed by the segments
func (c *Client) openAIPath(segments ...any) []any {
	var result []any
	prefix := strings.TrimSuffix(strings.TrimSuffix(c.url.Path, "/"), "/api")
	for _, part := range strings.Split(prefix, "/") {
		if part != "" {
			result = append(result, part)
		}
	}
	return append(append(result, "v1"), segments...)
}

// Unmarshal reads the audio of the response, and its type
func (a *audioResponse) Unmarshal(header http.Header, r io.Reader) error {
	a.contentType = header.Get(types.ContentTypeHeader)
	data, err := io.ReadAll(r)
	a.data = data
	return err
}
