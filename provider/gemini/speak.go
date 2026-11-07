package gemini

import (
	"context"
	"mime"
	"slices"
	"strconv"
	"strings"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/provider/schema"
)

///////////////////////////////////////////////////////////////////////////////
// INTERFACES

var _ llm.Speaker = (*Client)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// styleUnsupported is what the API says when a model doesn't take a style,
// such as Gemini 2.5, and 3.1's preview
const styleUnsupported = "Speech metadata is not supported"

// The rate, and channels, of the PCM which Gemini's speech models return,
// unless its type says otherwise, which is 16-bit little-endian
const (
	pcmRate     = 24000
	pcmChannels = 1
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Speak returns the text, spoken by the model, such as gemini-3.8-flash-tts,
// as audio, with generateContent, which is asked for audio, in the voice,
// such as Kore, and as the instructions say, such as "calmly", and the
// tokens used. The audio is WAV, unless PCM is asked for, which is
// audio/L16. A model is told how to speak with the text's style, or, when
// it doesn't take one, in the text, as directed says. Returns ErrBadParameter when there's no
// text, or an option isn't supported, such as the speed, which the
// instructions can say instead, and ErrNotImplemented for a model which
// doesn't speak.
func (c *Client) Speak(ctx context.Context, model llm.Model, text string, opts ...llm.Opt) (llm.Resource, llm.Usage, error) {
	m, ok := model.(*Model)
	if !ok || m.provider != c {
		return nil, llm.Usage{}, llm.ErrBadParameter.With("not a model of this provider")
	} else if !m.speaks() {
		return nil, llm.Usage{}, llm.ErrNotImplemented.Withf("%s doesn't speak", m.Name())
	} else if strings.TrimSpace(text) == "" {
		return nil, llm.Usage{}, llm.ErrBadParameter.With("missing text")
	}
	o, err := llm.ApplyOpts(opts...)
	if err != nil {
		return nil, llm.Usage{}, err
	}
	var voice, format, instructions string
	for _, key := range o.Keys() {
		var err error
		switch key {
		case llm.VoiceKey:
			voice, err = llm.GetOpt[string](o, key)
		case llm.AudioFormatKey:
			if format, err = llm.GetOpt[string](o, key); err == nil && format != "wav" && format != "pcm" {
				err = llm.ErrBadParameter.Withf("audio format %q isn't wav or pcm", format)
			}
		case llm.InstructionsKey:
			instructions, err = llm.GetOpt[string](o, key)
		default:
			err = llm.ErrBadParameter.Withf("option %q isn't supported for speech by gemini", key)
		}
		if err != nil {
			return nil, llm.Usage{}, err
		}
	}

	// The text, and how it's spoken, in the voice: as its style, which a
	// model which doesn't take it rejects, when it's told how in the text
	config := &generationConfig{ResponseModalities: []string{"AUDIO"}}
	if voice = strings.TrimSpace(voice); voice != "" {
		config.SpeechConfig = &speechConfig{VoiceConfig: voiceConfig{PrebuiltVoiceConfig: prebuiltVoiceConfig{VoiceName: voice}}}
	}
	instructions = strings.TrimSpace(instructions)
	textPart := part{Text: text}
	if instructions != "" {
		if strings.HasPrefix(m.Name(), "gemini-2") {
			textPart.Text = directed(m, instructions, text)
		} else {
			textPart.SpeechMetadata = &speechMetadata{Style: instructions}
		}
	}
	response, err := c.generate(ctx, m, textPart, config)
	if err != nil && textPart.SpeechMetadata != nil && strings.Contains(err.Error(), styleUnsupported) {
		response, err = c.generate(ctx, m, part{Text: directed(m, instructions, text)}, config)
	}
	if err != nil {
		return nil, llm.Usage{}, err
	}

	// The audio, and the tokens used
	var usage llm.Usage
	if response.UsageMetadata != nil {
		usage = replyUsage(*response.UsageMetadata)
	}
	for _, candidate := range response.Candidates {
		if candidate.Content == nil {
			continue
		}
		for _, p := range candidate.Content.Parts {
			if p.InlineData != nil && strings.HasPrefix(strings.ToLower(p.InlineData.MimeType), "audio/") {
				audio, err := speech(p.InlineData, format)
				return audio, usage, err
			}
		}
	}
	if response.PromptFeedback != nil && response.PromptFeedback.BlockReason != "" {
		return nil, usage, llm.ErrBadParameter.Withf("the text was blocked: %s", response.PromptFeedback.BlockReason)
	}
	return nil, usage, llm.ErrServiceUnavailable.With("no audio")
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// generate asks the model for the part, spoken, with the configuration
func (c *Client) generate(ctx context.Context, m *Model, text part, config *generationConfig) (*generateResponse, error) {
	request := generateRequest{Contents: []content{{Role: "user", Parts: []part{text}}}, GenerationConfig: config}
	payload, err := client.NewJSONRequest(request)
	if err != nil {
		return nil, err
	}
	var response generateResponse
	if err := c.DoWithContext(ctx, payload, &response, client.OptPath("models", m.Name()+":generateContent"), client.OptNoTimeout()); err != nil {
		return nil, err
	}
	return &response, nil
}

// speaks returns whether the model speaks, which it does when it's known to
func (m *Model) speaks() bool {
	speaks := m.Capabilities().SpeechOutput
	return speaks != nil && *speaks
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// directed returns the text, with how it's spoken, for a model which
// doesn't take a style: Gemini 2.5 is told before it, such as "Say this
// calmly: Hello", and later models have it as a tag, such as "[calmly]
// Hello", as they read what's before the text aloud, or don't speak
func directed(m *Model, instructions, text string) string {
	instructions = strings.TrimRight(instructions, ".:; ")
	if strings.HasPrefix(m.Name(), "gemini-2") {
		// Such as "Say this calmly", unless it says how to say it, such as
		// "Read it slowly"
		verb, _, _ := strings.Cut(strings.ToLower(instructions), " ")
		if !slices.Contains([]string{"say", "read", "speak", "whisper", "shout", "sing"}, verb) {
			instructions = "Say this " + instructions
		}
		return instructions + ": " + text
	}
	return "[" + instructions + "] " + text
}

// speech returns the audio, in the format: WAV, unless it's pcm, which is
// audio/L16. The model's audio is WAV, or raw PCM, of 16-bit little-endian
// samples, at the rate its type says, or 24kHz, mono.
func speech(audio *blob, format string) (llm.Resource, error) {
	mediaType, params, err := mime.ParseMediaType(audio.MimeType)
	if err != nil {
		return nil, llm.ErrServiceUnavailable.Withf("audio of type %q: %v", audio.MimeType, err)
	}
	switch strings.ToLower(mediaType) {
	case "audio/wav", "audio/x-wav", "audio/wave":
		if format != "pcm" {
			return &schema.Resource{ResourceMeta: schema.ResourceMeta{ContentType: "audio/wav", Data: audio.Data}}, nil
		}
		samples, rate, channels, ok := schema.WAVSamples(audio.Data)
		if !ok {
			return nil, llm.ErrServiceUnavailable.With("audio isn't 16-bit PCM")
		}
		return schema.PCM16(samples, rate, channels), nil
	case "audio/l16", "audio/pcm":
		rate, channels := pcmRate, pcmChannels
		if value, err := strconv.Atoi(params["rate"]); err == nil && value > 0 {
			rate = value
		}
		if value, err := strconv.Atoi(params["channels"]); err == nil && value > 0 {
			channels = value
		}
		if format == "pcm" {
			return schema.PCM16(audio.Data, rate, channels), nil
		}
		return schema.WAV16(audio.Data, rate, channels), nil
	default:
		return nil, llm.ErrServiceUnavailable.Withf("audio of type %q isn't WAV or PCM", audio.MimeType)
	}
}
