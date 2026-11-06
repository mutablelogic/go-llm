package openai

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

// speechRequest is a request to POST /audio/speech, for the text, spoken,
// in the voice, in the format, at the speed, and as the instructions say
type speechRequest struct {
	Model          string   `json:"model"`
	Input          string   `json:"input"`
	Voice          string   `json:"voice,omitempty"`
	ResponseFormat string   `json:"response_format,omitempty"`
	Speed          *float64 `json:"speed,omitempty"`
	Instructions   string   `json:"instructions,omitempty"`
}

// audioResponse is the audio of a response, and its type
type audioResponse struct {
	data        []byte
	contentType string
}

var _ llm.Speaker = (*Client)(nil)
var _ client.Unmarshaler = (*audioResponse)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// defaultVoice is the voice of OpenAI's models, unless one is given, as
// they need one
const defaultVoice = "alloy"

// audioTypes are the types of the formats of audio, by their names
var audioTypes = map[string]string{
	"mp3":  "audio/mpeg",
	"opus": "audio/ogg",
	"aac":  "audio/aac",
	"flac": "audio/flac",
	"wav":  "audio/wav",
	"pcm":  "audio/L16",
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Speak returns the text, spoken by the model, as audio, with POST
// /audio/speech, whose type is the format it's in. OpenAI's models speak in
// a voice, which is alloy, unless one is given. Returns ErrBadParameter
// when there's no text, or an option isn't supported.
func (c *Client) Speak(ctx context.Context, model llm.Model, text string, opts ...llm.Opt) (llm.Resource, llm.Usage, error) {
	if m, ok := model.(*Model); !ok || m.provider != c {
		return nil, llm.Usage{}, llm.ErrBadParameter.With("not a model of this provider")
	} else if strings.TrimSpace(text) == "" {
		return nil, llm.Usage{}, llm.ErrBadParameter.With("missing text")
	}
	req := speechRequest{Model: model.Name(), Input: text}
	if err := req.apply(opts...); err != nil {
		return nil, llm.Usage{}, err
	}
	if req.Voice == "" && c.server() == serverOpenAI {
		req.Voice = defaultVoice
	}
	resource, err := speak(ctx, c.Client, req, client.OptPath("audio", "speech"))
	if err != nil {
		return nil, llm.Usage{}, err
	}
	return resource, llm.Usage{}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// apply sets the options on the request: the voice, the format, the speed,
// and the instructions. Returns ErrBadParameter for any other option.
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
			r.Speed, err = getOpt[float64](o, key)
		case llm.InstructionsKey:
			r.Instructions, err = llm.GetOpt[string](o, key)
		default:
			err = llm.ErrBadParameter.Withf("option %q isn't supported for speech", key)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// speak sends the request, to the path, and returns the audio, whose type is
// the response's, or the format's, when the response's says only that it's
// binary
func speak(ctx context.Context, c *client.Client, req speechRequest, path client.RequestOpt) (llm.Resource, error) {
	payload, err := client.NewJSONRequestEx(http.MethodPost, req, types.ContentTypeAny)
	if err != nil {
		return nil, err
	}
	var audio audioResponse
	if err := c.DoWithContext(ctx, payload, &audio, path, client.OptNoTimeout()); err != nil {
		return nil, err
	} else if len(audio.data) == 0 {
		return nil, llm.ErrServiceUnavailable.With("no audio")
	}
	contentType := audio.contentType
	if mediaType, _, err := mime.ParseMediaType(contentType); err != nil || mediaType == types.ContentTypeBinary || !strings.HasPrefix(mediaType, "audio/") {
		format := req.ResponseFormat
		if format == "" {
			format = "mp3"
		}
		if byFormat, ok := audioTypes[format]; ok {
			contentType = byFormat
		}
	}
	return &schema.Resource{ResourceMeta: schema.ResourceMeta{ContentType: contentType, Data: audio.data}}, nil
}

// Unmarshal reads the audio of the response, and its type
func (a *audioResponse) Unmarshal(header http.Header, r io.Reader) error {
	a.contentType = header.Get(types.ContentTypeHeader)
	data, err := io.ReadAll(r)
	a.data = data
	return err
}
