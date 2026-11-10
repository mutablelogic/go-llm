package elevenlabs

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/provider/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

var _ llm.Speaker = (*Client)(nil)

// speechRequest is a request to speak the text, with the model, in the
// voice's settings, such as its speed
type speechRequest struct {
	Text     string          `json:"text"`
	Model    string          `json:"model_id"`
	Settings *speechSettings `json:"voice_settings,omitempty"`
}

// speechSettings are how the voice speaks
type speechSettings struct {
	Speed *float64 `json:"speed,omitempty"`
}

// audioResponse is the audio, and the characters it cost, which ElevenLabs
// says in a header
type audioResponse struct {
	data       []byte
	characters uint
}

var _ client.Unmarshaler = (*audioResponse)(nil)

// audioFormat is a format of the audio, as ElevenLabs is asked for it, and
// its type, or, for PCM, its rate, as it's raw
type audioFormat struct {
	output      string
	contentType string
	pcmRate     int
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// defaultVoice is the voice the text is spoken in, unless one is given,
// which is George, one of ElevenLabs' own voices, which every account has
const defaultVoice = "JBFqnCBsd6RMkjVDRZzb"

// audioFormats are the formats of the audio, by their names: Opus, which is
// in Ogg, as a voice note is, so it isn't converted, MP3, which is the
// default, and PCM, which is 16-bit little-endian, at 24 kHz, as it is, or
// in WAV
var audioFormats = map[string]audioFormat{
	"mp3":  {"mp3_44100_128", "audio/mpeg", 0},
	"opus": {"opus_48000_64", "audio/ogg; codecs=opus", 0},
	"pcm":  {"pcm_24000", "", 24000},
	"wav":  {"pcm_24000", "", 24000},
}

// reVoiceID matches the shape of a voice's ID, such as
// "JBFqnCBsd6RMkjVDRZzb", which a voice which isn't the account's is passed
// on by, and voicesTTL is how long the account's voices are kept, to find
// one by its title
var reVoiceID = regexp.MustCompile(`^[A-Za-z0-9]{20}$`)

const voicesTTL = time.Minute

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Speak returns the text, spoken by the model, which must be one of
// ElevenLabs' models which speak, such as eleven_v3, as audio, whose type is
// the format it's in, and the characters it cost. It's spoken in the voice,
// by its ID, or its title, such as "George", which is looked up in the
// account's voices, or George's, unless one is given, in the format, which
// is mp3, opus, pcm or wav, or mp3, unless one is given, and at the speed,
// which ElevenLabs takes from 0.7 to 1.2. eleven_v3 and later are told how
// to speak with audio tags in the text, such as [whispers], rather than
// instructions. Returns ErrBadParameter when there's no text, or it's longer
// than the model speaks at once, or an option isn't supported, and
// ErrNotImplemented for a model which doesn't speak. The request isn't timed
// out, as long text takes a while, but is cancelled with the context.
func (c *Client) Speak(ctx context.Context, model llm.Model, text string, opts ...llm.Opt) (llm.Resource, llm.Usage, error) {
	m, ok := model.(*Model)
	if !ok || m.provider != c {
		return nil, llm.Usage{}, llm.ErrBadParameter.With("not a model of this provider")
	} else if m.speech == nil {
		return nil, llm.Usage{}, llm.ErrNotImplemented.Withf("%s doesn't speak", m.Name())
	} else if strings.TrimSpace(text) == "" {
		return nil, llm.Usage{}, llm.ErrBadParameter.With("missing text")
	} else if limit := m.speech.MaxText; limit > 0 && uint(utf8.RuneCountInString(text)) > limit {
		return nil, llm.Usage{}, llm.ErrBadParameter.Withf("%s speaks at most %d characters at once", m.Name(), limit)
	}

	// The options
	o, err := llm.ApplyOpts(opts...)
	if err != nil {
		return nil, llm.Usage{}, err
	}
	req := speechRequest{Text: text, Model: m.Name()}
	voice, formatName := defaultVoice, "mp3"
	for _, key := range o.Keys() {
		switch key {
		case llm.VoiceKey:
			var name string
			if name, err = llm.GetOpt[string](o, key); err == nil && strings.TrimSpace(name) != "" {
				voice, err = c.voiceID(ctx, strings.TrimSpace(name))
			}
		case llm.AudioFormatKey:
			var name string
			if name, err = llm.GetOpt[string](o, key); err == nil && name != "" {
				if _, ok := audioFormats[strings.ToLower(name)]; ok {
					formatName = strings.ToLower(name)
				} else {
					err = llm.ErrBadParameter.Withf("audio format %q isn't mp3, opus, pcm or wav", name)
				}
			}
		case llm.SpeedKey:
			var speed float64
			if speed, err = llm.GetOpt[float64](o, key); err == nil {
				req.Settings = &speechSettings{Speed: types.Ptr(speed)}
			}
		case llm.InstructionsKey:
			err = llm.ErrBadParameter.Withf("option %q isn't supported by elevenlabs: eleven_v3, and later, are told how to speak with audio tags in the text, such as [whispers]", key)
		default:
			err = llm.ErrBadParameter.Withf("option %q isn't supported for speech", key)
		}
		if err != nil {
			return nil, llm.Usage{}, err
		}
	}

	// Speak, and return the audio, which is raw, for PCM, which is
	// audio/L16, which is big-endian, or in WAV
	format := audioFormats[formatName]
	payload, err := client.NewJSONRequestEx(http.MethodPost, req, types.ContentTypeAny)
	if err != nil {
		return nil, llm.Usage{}, err
	}
	var audio audioResponse
	if err := c.DoWithContext(ctx, payload, &audio, client.OptPath("text-to-speech", voice), client.OptQuery(url.Values{"output_format": {format.output}}), client.OptNoTimeout()); err != nil {
		return nil, llm.Usage{}, err
	} else if len(audio.data) == 0 {
		return nil, llm.Usage{}, llm.ErrServiceUnavailable.With("no audio")
	}
	usage := llm.Usage{Characters: audio.characters}
	switch {
	case formatName == "wav":
		return schema.WAV16(audio.data, format.pcmRate, 1), usage, nil
	case format.pcmRate > 0:
		return schema.PCM16(audio.data, format.pcmRate, 1), usage, nil
	}
	return &schema.Resource{ResourceMeta: schema.ResourceMeta{ContentType: format.contentType, Data: audio.data}}, usage, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// voiceID returns the ID of the voice, which is the name, when it's the ID
// of one of the account's voices, or the voice whose title is the name, or
// starts with it, before a description, such as "George" for "George - Warm,
// Captivating Storyteller", ignoring case, or the name, when it's none of
// them, but is shaped like an ID, such as a voice of the library, which
// ElevenLabs says, when it isn't one. A title which is shaped like an ID,
// such as "ProfessionalNarrator", is a title. Returns ErrBadParameter when
// there's none, or more than one.
func (c *Client) voiceID(ctx context.Context, name string) (string, error) {
	voices, err := c.cachedVoices(ctx)
	if err != nil {
		return "", err
	}
	var found []string
	for _, voice := range voices {
		if voice.ID == name {
			return voice.ID, nil
		}
		title := strings.TrimSpace(voice.Title)
		short, _, _ := strings.Cut(title, " - ")
		if strings.EqualFold(title, name) || strings.EqualFold(strings.TrimSpace(short), name) {
			found = append(found, voice.ID)
		}
	}
	switch {
	case len(found) == 1:
		return found[0], nil
	case len(found) > 1:
		return "", llm.ErrBadParameter.Withf("voice %q is more than one of the account's: give its id", name)
	case reVoiceID.MatchString(name):
		return name, nil
	default:
		return "", llm.ErrBadParameter.Withf("voice %q isn't one of the account's: list them with the models tool, with speech_output", name)
	}
}

// cachedVoices returns the account's voices, which are kept for voicesTTL,
// so a voice is found by its title without listing them each time it speaks
func (c *Client) cachedVoices(ctx context.Context) ([]llm.Voice, error) {
	c.voicesMu.Lock()
	defer c.voicesMu.Unlock()
	if !c.voicesAt.IsZero() && time.Since(c.voicesAt) < voicesTTL {
		return c.cached, nil
	}
	voices, err := c.accountVoices(ctx)
	if err != nil {
		return nil, err
	}
	c.cached, c.voicesAt = voices, time.Now()
	return voices, nil
}

///////////////////////////////////////////////////////////////////////////////
// JSON

// Unmarshal reads the audio of the response, and the characters it cost
func (a *audioResponse) Unmarshal(header http.Header, r io.Reader) error {
	if cost, err := strconv.ParseUint(strings.TrimSpace(header.Get("character-cost")), 10, 0); err == nil {
		a.characters = uint(cost)
	}
	data, err := io.ReadAll(r)
	a.data = data
	return err
}
