package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"mime"
	"path"
	"slices"
	"strings"
	"time"
	"unicode"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// INTERFACE CHECK

var _ llm.Transcriber = (*Client)(nil)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// transcribeFamily is how a model transcribes speech, by its name, and its
// server, which is the format of its response, and whether it streams
type transcribeFamily int

const (
	transcribeWhisper transcribeFamily = iota // whisper, and other models, which return verbose JSON, with segments, and don't stream
	transcribeText                            // OpenAI's gpt-4o and gpt-transcribe models, on any server, which return text, and stream it
	transcribeDiarize                         // OpenAI's diarizing models, on any server, which return segments with speakers, and stream them
	transcribeMistral                         // Mistral's models, which return segments, and speakers, which they're asked for, and stream
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// audioExtensions are the extensions of audio files, by their type, as the
// API detects an audio file's format from its name
var audioExtensions = map[string]string{
	"audio/mpeg":   ".mp3",
	"audio/mp3":    ".mp3",
	"audio/mp4":    ".m4a",
	"audio/x-m4a":  ".m4a",
	"audio/aac":    ".aac",
	"audio/wav":    ".wav",
	"audio/wave":   ".wav",
	"audio/x-wav":  ".wav",
	"audio/webm":   ".webm",
	"audio/ogg":    ".ogg",
	"audio/flac":   ".flac",
	"audio/x-flac": ".flac",
	"video/mp4":    ".mp4",
	"video/webm":   ".webm",
}

// whisperLanguages are the ISO 639-1 codes of the languages whisper
// detects, by the names it returns them as
var whisperLanguages = map[string]string{
	"afrikaans": "af", "albanian": "sq", "amharic": "am", "arabic": "ar", "armenian": "hy", "assamese": "as",
	"azerbaijani": "az", "bashkir": "ba", "basque": "eu", "belarusian": "be", "bengali": "bn", "bosnian": "bs",
	"breton": "br", "bulgarian": "bg", "catalan": "ca", "chinese": "zh", "croatian": "hr", "czech": "cs",
	"danish": "da", "dutch": "nl", "english": "en", "estonian": "et", "faroese": "fo", "finnish": "fi",
	"french": "fr", "galician": "gl", "georgian": "ka", "german": "de", "greek": "el", "gujarati": "gu",
	"haitian creole": "ht", "hausa": "ha", "hebrew": "he", "hindi": "hi", "hungarian": "hu", "icelandic": "is",
	"indonesian": "id", "italian": "it", "japanese": "ja", "javanese": "jw", "kannada": "kn", "kazakh": "kk",
	"khmer": "km", "korean": "ko", "lao": "lo", "latin": "la", "latvian": "lv", "lingala": "ln",
	"lithuanian": "lt", "luxembourgish": "lb", "macedonian": "mk", "malagasy": "mg", "malay": "ms", "malayalam": "ml",
	"maltese": "mt", "maori": "mi", "marathi": "mr", "mongolian": "mn", "myanmar": "my", "nepali": "ne",
	"norwegian": "no", "nynorsk": "nn", "occitan": "oc", "pashto": "ps", "persian": "fa", "polish": "pl",
	"portuguese": "pt", "punjabi": "pa", "romanian": "ro", "russian": "ru", "sanskrit": "sa", "serbian": "sr",
	"shona": "sn", "sindhi": "sd", "sinhala": "si", "slovak": "sk", "slovenian": "sl", "somali": "so",
	"spanish": "es", "sundanese": "su", "swahili": "sw", "swedish": "sv", "tagalog": "tl", "tajik": "tg",
	"tamil": "ta", "tatar": "tt", "telugu": "te", "thai": "th", "tibetan": "bo", "turkish": "tr",
	"turkmen": "tk", "ukrainian": "uk", "urdu": "ur", "uzbek": "uz", "vietnamese": "vi", "welsh": "cy",
	"yiddish": "yi", "yoruba": "yo",
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Transcribe returns the speech in the audio as text, and its language,
// when the model returns it, and its segments, and who spoke them, when
// they're asked for, and the model returns them, which whisper and Mistral's
// models do, and OpenAI's diarizing models do with their speakers, and the
// seconds and tokens used. When the stream function isn't nil, the
// transcript is streamed, by Mistral's models, and OpenAI's other than
// whisper, or otherwise the function is called with it once it's
// transcribed. The request isn't timed out, as audio can take a while, but
// is cancelled with the context.
func (c *Client) Transcribe(ctx context.Context, model llm.Model, audio llm.Resource, stream llm.TranscriberStreamFn, opts ...llm.Opt) (llm.Transcript, llm.Usage, error) {
	return c.transcribe(ctx, "transcriptions", model, audio, stream, opts...)
}

// Translate returns the speech in the audio, in any language, as English
// text, as Transcribe does, which only OpenAI's whisper does, and which
// isn't streamed. Returns ErrNotImplemented for a model which is known not
// to translate.
func (c *Client) Translate(ctx context.Context, model llm.Model, audio llm.Resource, stream llm.TranscriberStreamFn, opts ...llm.Opt) (llm.Transcript, llm.Usage, error) {
	if c.server() == serverMistral || c.isDeepSeek() {
		return llm.Transcript{}, llm.Usage{}, llm.ErrNotImplemented.With("only openai's whisper translates audio")
	} else if translation := model.Capabilities().Translation; translation != nil && !*translation {
		return llm.Transcript{}, llm.Usage{}, llm.ErrNotImplemented.Withf("model %q can't translate audio", model.Name())
	}
	return c.transcribe(ctx, "translations", model, audio, stream, opts...)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// transcribe returns the speech in the audio, with the API's endpoint, which
// is transcriptions or translations, and streams it to the function, when
// it isn't nil
func (c *Client) transcribe(ctx context.Context, endpoint string, model llm.Model, audio llm.Resource, stream llm.TranscriberStreamFn, opts ...llm.Opt) (llm.Transcript, llm.Usage, error) {
	if m, ok := model.(*Model); !ok || m.provider != c {
		return llm.Transcript{}, llm.Usage{}, llm.ErrBadParameter.With("not a model of this provider")
	} else if audio == nil {
		return llm.Transcript{}, llm.Usage{}, llm.ErrBadParameter.With("missing audio")
	} else if c.isDeepSeek() {
		return llm.Transcript{}, llm.Usage{}, llm.ErrNotImplemented.With("deepseek has no transcriptions")
	}

	// The audio, as a file, named with its type's extension, as the API
	// detects its format from it
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

	// The request, with the options, for the model, which returns segments
	// in its own format, if any
	request := transcribeRequest{
		File:  types.File{Path: audioName(audio.Name(), contentType), Body: io.NopCloser(bytes.NewReader(data)), ContentType: contentType},
		Model: model.Name(),
	}
	family := transcribeFamilyOf(c.server(), model.Name())
	options, err := request.apply(family, model.Name(), endpoint == "translations", opts...)
	if err != nil {
		return llm.Transcript{}, llm.Usage{}, err
	}

	// The transcript is streamed by the models which stream, which whisper
	// doesn't, nor its translations
	segments := options.segments || options.speakers
	request.Stream = stream != nil && endpoint == "transcriptions" && family != transcribeWhisper

	// Send the request, and receive the transcript, or its pieces
	req, err := client.NewMultipartRequest(request, types.ContentTypeJSON)
	if err != nil {
		return llm.Transcript{}, llm.Usage{}, err
	}
	var response transcribeResponse
	if request.Stream {
		chunks := &transcribeStream{stream: stream, segments: segments}
		if err := c.DoWithContext(ctx, req, nil, client.OptPath("audio", endpoint), client.OptTextStreamCallback(chunks.add), client.OptNoTimeout()); err != nil {
			return llm.Transcript{}, llm.Usage{}, err
		} else if !chunks.done {
			return llm.Transcript{}, llm.Usage{}, llm.ErrServiceUnavailable.With("the transcript ended before it was done")
		}
		response = chunks.response
	} else if err := c.DoWithContext(ctx, req, &response, client.OptPath("audio", endpoint), client.OptNoTimeout()); err != nil {
		return llm.Transcript{}, llm.Usage{}, err
	}

	// The transcript, in its language, which a translation is in English,
	// and its segments, when they're asked for
	transcript := llm.Transcript{Text: strings.TrimSpace(response.Text), Language: response.language()}
	switch {
	case endpoint == "translations":
		transcript.Language = "en"
	case transcript.Language == "":
		transcript.Language = options.language
	}
	if segments {
		for _, segment := range response.Segments {
			transcript.Segments = append(transcript.Segments, segment.transcript())
		}
	}

	// A transcript which wasn't streamed is streamed once it's transcribed
	if stream != nil && !request.Stream {
		stream(transcript.Text, nil)
		for i := range transcript.Segments {
			stream("", &transcript.Segments[i])
		}
	}
	return transcript, response.usage(), nil
}

///////////////////////////////////////////////////////////////////////////////
// STREAM

// transcribeStream collects the events of a streamed transcript into a
// response, and calls the stream function with each piece of its text, and
// each segment, when they're asked for. OpenAI's events have their type in
// their data, and Mistral's are named for it too. A model which streams
// segments without text, such as OpenAI's diarizing models, has the text of
// each segment streamed as text.
type transcribeStream struct {
	stream   llm.TranscriberStreamFn
	segments bool               // Whether segments are streamed
	text     bool               // Whether text has been streamed
	deltas   bool               // Whether the model streams its text
	done     bool               // Whether the transcript is done
	language string             // The language, which Mistral streams
	response transcribeResponse // The transcript
}

// add adds an event of the stream. An event with an error fails the
// transcript.
func (s *transcribeStream) add(event client.TextStreamEvent) error {
	if strings.TrimSpace(event.Data) == streamDone {
		return io.EOF
	}
	var chunk struct {
		Type     string     `json:"type"`
		Delta    string     `json:"delta"`          // OpenAI's text
		Text     string     `json:"text"`           // Mistral's text
		Language string     `json:"audio_language"` // Mistral's language
		Error    *chatError `json:"error"`
	}
	if err := json.Unmarshal([]byte(event.Data), &chunk); err != nil {
		return err
	} else if chunk.Error != nil {
		return errors.New(chunk.Error.Message)
	}
	switch chunk.Type {
	case "transcript.text.delta", "transcription.text.delta":
		s.deltas = true
		s.addText(chunk.Delta + chunk.Text)
	case "transcript.text.segment", "transcription.segment":
		var segment transcribeSegment
		if err := json.Unmarshal([]byte(event.Data), &segment); err != nil {
			return err
		}
		s.response.Segments = append(s.response.Segments, segment)
		if !s.deltas {
			// A segment's text is separated from the last's, which it
			// doesn't always start with a space for
			text := segment.Text
			if s.text && text != "" && !unicode.IsSpace(rune(text[0])) {
				text = " " + text
			}
			s.addText(text)
		}
		if s.segments {
			result := segment.transcript()
			s.stream("", &result)
		}
	case "transcription.language":
		s.language = chunk.Language
	case "transcript.text.done", "transcription.done":
		// The transcript, which Mistral's has the segments of, which are
		// those streamed, and its language, when it was streamed
		segments := s.response.Segments
		if err := json.Unmarshal([]byte(event.Data), &s.response); err != nil {
			return err
		}
		if len(s.response.Segments) == 0 {
			s.response.Segments = segments
		}
		if s.response.Language == nil && s.language != "" {
			s.response.Language = types.Ptr(s.language)
		}
		s.done = true
	}
	return nil
}

// addText streams a piece of the text, which, when it's the first, has no
// space at its start, as a segment's text has
func (s *transcribeStream) addText(text string) {
	if !s.text {
		text = strings.TrimLeft(text, " ")
	}
	if text != "" {
		s.text = true
		s.stream(text, nil)
	}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// transcribeFamilyOf returns how the model, by its name, on the server,
// transcribes speech: Mistral's models, OpenAI's diarizing models, and its
// gpt-4o and gpt-transcribe models, whose names are the same on another
// server, and its other models, as whisper's are, and other servers'
// models, which are whisper's, or are taken to be like it
func transcribeFamilyOf(server server, model string) transcribeFamily {
	switch {
	case server == serverMistral:
		return transcribeMistral
	case strings.HasPrefix(model, "whisper"):
		return transcribeWhisper
	case strings.Contains(model, "-diarize"):
		return transcribeDiarize
	case server == serverOpenAI || strings.HasPrefix(model, "gpt-") && strings.Contains(model, "transcribe"):
		return transcribeText
	default:
		return transcribeWhisper
	}
}

// audioName returns the name of the audio file, which is the resource's
// name, when it has the extension of an audio file, or otherwise "audio",
// with the extension of its type
func audioName(name, contentType string) string {
	if ext := strings.ToLower(path.Ext(name)); ext != "" && slices.Contains(slices.Collect(maps.Values(audioExtensions)), ext) {
		return name
	}
	if ext, exists := audioExtensions[contentType]; exists {
		return "audio" + ext
	} else if exts, err := mime.ExtensionsByType(contentType); err == nil && len(exts) > 0 {
		return "audio" + exts[0]
	}
	return "audio"
}

// seconds returns the seconds as a duration
func seconds(value float64) time.Duration {
	return time.Duration(value * float64(time.Second))
}
