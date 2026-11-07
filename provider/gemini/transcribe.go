package gemini

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// INTERFACES

var _ llm.Transcriber = (*Client)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// inlineLimit is the most audio sent with a request, as it's encoded as
// base64, in a request of at most 20MB, and more is uploaded, with the
// Files API
var inlineLimit = 14 << 20

// segmentPause is the longest pause within a segment, so a speaker who
// pauses, without ending a sentence, starts another
const segmentPause = 1500 * time.Millisecond

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Transcribe returns the speech in the audio as text, with a transcription
// model, such as gemini-3.5-transcribe, in the language, when it's set,
// which it's otherwise heard in, with the transcript's segments, from when
// each word was spoken, and who spoke them, when they're asked for, which
// are each a sentence, or what was said before a pause, of a speaker. The
// pieces of the transcript are streamed, as they're transcribed. Audio of
// more than 14MB is uploaded, with the Files API, and deleted, once it's
// transcribed. Returns ErrNotImplemented for a model which doesn't
// transcribe, and ErrBadParameter for an option which isn't supported.
func (c *Client) Transcribe(ctx context.Context, model llm.Model, audio llm.Resource, stream llm.TranscriberStreamFn, opts ...llm.Opt) (llm.Transcript, llm.Usage, error) {
	m, ok := model.(*Model)
	if !ok || m.provider != c {
		return llm.Transcript{}, llm.Usage{}, llm.ErrBadParameter.With("not a model of this provider")
	} else if !types.Value(m.Capabilities().Transcription) {
		return llm.Transcript{}, llm.Usage{}, llm.ErrNotImplemented.Withf("%s doesn't transcribe", m.Name())
	} else if audio == nil {
		return llm.Transcript{}, llm.Usage{}, llm.ErrBadParameter.With("missing audio")
	}
	config, language, segments, err := transcribeOptions(opts...)
	if err != nil {
		return llm.Transcript{}, llm.Usage{}, err
	}
	data, err := llm.ReadAll(ctx, audio)
	if err != nil {
		return llm.Transcript{}, llm.Usage{}, err
	} else if len(data) == 0 {
		return llm.Transcript{}, llm.Usage{}, llm.ErrBadParameter.With("missing audio")
	}
	contentType := audio.Type()

	// The audio, with the request, or uploaded, when it's large, and deleted
	// once it's transcribed
	audioPart := part{InlineData: &blob{MimeType: contentType, Data: data}}
	if len(data) > inlineLimit {
		f, err := c.uploadFile(ctx, data, contentType)
		if err != nil {
			return llm.Transcript{}, llm.Usage{}, err
		}
		defer c.deleteFile(f)
		audioPart = part{FileData: &fileData{MimeType: contentType, FileURI: f.URI}}
	}
	request := generateRequest{Contents: []content{{Role: "user", Parts: []part{audioPart}}}}
	if config != nil {
		request.GenerationConfig = &generationConfig{TranscriptionConfig: config}
	}

	// Send the request, and collect the transcript, as it's streamed
	payload, err := client.NewJSONRequestEx(http.MethodPost, request, client.ContentTypeTextStream)
	if err != nil {
		return llm.Transcript{}, llm.Usage{}, err
	}
	transcript := &transcriptStream{stream: stream, segments: segments}
	if err := c.DoWithContext(ctx, payload, nil,
		client.OptPath("models", m.Name()+":streamGenerateContent"),
		client.OptQuery(url.Values{"alt": {"sse"}}),
		client.OptNoTimeout(),
		client.OptTextStreamCallback(transcript.add),
	); err != nil {
		return llm.Transcript{}, llm.Usage{}, err
	} else if transcript.err != nil {
		return llm.Transcript{}, llm.Usage{}, transcript.err
	} else if transcript.blocked != "" {
		return llm.Transcript{}, llm.Usage{}, llm.ErrBadParameter.Withf("the audio was blocked: %s", transcript.blocked)
	} else if !transcript.done {
		return llm.Transcript{}, llm.Usage{}, llm.ErrServiceUnavailable.With("the transcript ended before it was done")
	} else if transcript.finish == "MAX_TOKENS" {
		// More than the model returns, which is many hours of speech, so it's
		// repeating itself
		return llm.Transcript{}, replyUsage(transcript.usage), llm.ErrServiceUnavailable.With("the transcript is longer than the model returns, as it repeated itself")
	} else if transcript.finish != "STOP" {
		return llm.Transcript{}, replyUsage(transcript.usage), llm.ErrServiceUnavailable.Withf("the transcript stopped: %s", transcript.finish)
	}
	result := transcript.result
	result.Language = language
	return result, replyUsage(transcript.usage), nil
}

// Translate isn't supported, as Gemini's transcription models only
// transcribe
func (c *Client) Translate(ctx context.Context, model llm.Model, _ llm.Resource, _ llm.TranscriberStreamFn, _ ...llm.Opt) (llm.Transcript, llm.Usage, error) {
	if m, ok := model.(*Model); !ok || m.provider != c {
		return llm.Transcript{}, llm.Usage{}, llm.ErrBadParameter.With("not a model of this provider")
	}
	return llm.Transcript{}, llm.Usage{}, llm.ErrNotImplemented.Withf("%s doesn't translate", model.Name())
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// transcribeOptions returns how the audio is transcribed, as the options
// say: in the language, with when each word is spoken, for the segments,
// and who spoke it, which needs them. Returns the language, and whether
// segments are asked for, and ErrBadParameter for any other option.
func transcribeOptions(opts ...llm.Opt) (*audioTranscriptionConfig, string, bool, error) {
	o, err := llm.ApplyOpts(opts...)
	if err != nil {
		return nil, "", false, err
	}
	var language string
	var segments, speakers bool
	for _, key := range o.Keys() {
		var err error
		switch key {
		case llm.LanguageKey:
			language, err = llm.GetOpt[string](o, key)
			language = strings.TrimSpace(language)
		case llm.SegmentsKey:
			segments, err = llm.GetOpt[bool](o, key)
		case llm.SpeakersKey:
			speakers, err = llm.GetOpt[bool](o, key)
		default:
			err = llm.ErrBadParameter.Withf("option %q isn't supported for transcription by gemini", key)
		}
		if err != nil {
			return nil, "", false, err
		}
	}
	segments = segments || speakers
	config := &audioTranscriptionConfig{WordTimestamp: segments, Diarization: speakers}
	if language != "" {
		config.LanguageCodes = []string{language}
	}
	if !segments && language == "" {
		config = nil
	}
	return config, language, segments, nil
}

///////////////////////////////////////////////////////////////////////////////
// STREAM

// transcriptStream collects the pieces of a streamed transcript, each of
// which is a response with the next parts of its text, each of which is a
// speaker's turn, when who spoke is asked for, with when each word was
// spoken, when that's asked for, and calls the stream function, when there
// is one, with each piece of its text, and each of its segments, when
// they're asked for. The last piece has the tokens used.
type transcriptStream struct {
	stream   llm.TranscriberStreamFn
	segments bool
	result   llm.Transcript
	usage    usageMetadata
	blocked  string
	finish   string
	done     bool
	err      error
}

// add adds a piece of the transcript. An error fails it, and it's done
// once the model finishes, or the audio is blocked.
func (s *transcriptStream) add(event client.TextStreamEvent) error {
	if strings.TrimSpace(event.Data) == "" {
		return nil
	}
	var response generateResponse
	if err := json.Unmarshal([]byte(event.Data), &response); err != nil {
		return err
	}
	if response.Error != nil {
		s.err = streamError(*response.Error)
		return s.err
	}
	if response.UsageMetadata != nil {
		s.usage = *response.UsageMetadata
	}
	if response.PromptFeedback != nil && response.PromptFeedback.BlockReason != "" {
		s.blocked, s.done = response.PromptFeedback.BlockReason, true
	}
	for _, candidate := range response.Candidates {
		if candidate.Content != nil {
			for _, p := range candidate.Content.Parts {
				if p.Transcription != nil {
					s.addTranscription(*p.Transcription)
				}
			}
		}
		if candidate.FinishReason != "" {
			s.finish, s.done = candidate.FinishReason, true
		}
	}
	return nil
}

// addTranscription adds a part's text, after a space, unless it's the
// first, and its segments, when they're asked for
func (s *transcriptStream) addTranscription(t audioTranscription) {
	text := strings.TrimSpace(t.Text)
	if text == "" {
		return
	}
	if s.result.Text != "" {
		text = " " + text
	}
	s.result.Text += text
	if s.stream != nil {
		s.stream(text, nil)
	}
	if !s.segments {
		return
	}
	for _, segment := range segmentsOf(t) {
		s.result.Segments = append(s.result.Segments, segment)
		if s.stream != nil {
			s.stream("", &s.result.Segments[len(s.result.Segments)-1])
		}
	}
}

// segmentsOf returns the segments of a part's words, each a sentence, or
// what was said before a pause, with when they start and end, and who
// spoke them, when that's known
func segmentsOf(t audioTranscription) []llm.TranscriptSegment {
	var result []llm.TranscriptSegment
	var current *llm.TranscriptSegment
	var words []string
	end := func() {
		if current != nil {
			current.Text = strings.Join(words, " ")
			result = append(result, *current)
		}
		current, words = nil, nil
	}
	for _, w := range t.Words {
		word := strings.TrimSpace(w.Word)
		if word == "" {
			continue
		}
		start, finish := offset(w.StartOffset), offset(w.EndOffset)
		if current != nil && start-current.End > segmentPause {
			end()
		}
		if current == nil {
			current = &llm.TranscriptSegment{Start: start, Speaker: t.SpeakerLabel}
		}
		words = append(words, word)
		current.End = max(current.End, finish, start)
		if sentenceEnds(word) {
			end()
		}
	}
	end()
	return result
}

// offset returns an offset, such as 0.200s, as a duration, or zero
func offset(value string) time.Duration {
	d, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil {
		return 0
	}
	return d
}

// sentenceEnds returns whether the word ends a sentence, as it ends with a
// full stop, a question mark, or an exclamation mark, before any closing
// quotes or brackets
func sentenceEnds(word string) bool {
	word = strings.TrimRightFunc(word, func(r rune) bool {
		return unicode.Is(unicode.Pf, r) || unicode.Is(unicode.Pe, r) || r == '"' || r == '\''
	})
	return strings.HasSuffix(word, ".") || strings.HasSuffix(word, "?") || strings.HasSuffix(word, "!") ||
		strings.HasSuffix(word, "。") || strings.HasSuffix(word, "？") || strings.HasSuffix(word, "！")
}
