package gemini

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/provider/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// audio returns MP3, of the size
func audio(size int) llm.Resource {
	return &schema.Resource{ResourceMeta: schema.ResourceMeta{ContentType: "audio/mpeg", Data: []byte(strings.Repeat("x", size))}}
}

func Test_Transcribe(t *testing.T) {
	ctx := context.Background()
	api, c, chat := chatClient(t)
	model, err := c.GetModel(ctx, "gemini-new-transcribe")
	require.NoError(t, err)

	// The text, as it's streamed, with the tokens used, of audio which is
	// sent with the request
	api.reply = []string{
		`{"candidates":[{"content":{"role":"model","parts":[{"audioTranscription":{"text":"Hello there."}}]}}],"usageMetadata":{"promptTokenCount":300,"candidatesTokenCount":3}}`,
		`{"candidates":[{"content":{"role":"model","parts":[{"audioTranscription":{"text":"How are you?"}}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":300,"candidatesTokenCount":7}}`,
	}
	var streamed []string
	transcript, usage, err := c.Transcribe(ctx, model, audio(10), func(text string, segment *llm.TranscriptSegment) {
		streamed = append(streamed, text)
	})
	require.NoError(t, err)
	assert.Equal(t, "Hello there. How are you?", transcript.Text)
	assert.Equal(t, []string{"Hello there.", " How are you?"}, streamed)
	assert.Empty(t, transcript.Segments)
	assert.Empty(t, transcript.Language)
	assert.Equal(t, uint(300), usage.InputTokens)
	assert.Equal(t, uint(7), usage.OutputTokens)
	parts := api.asked.Contents[0].Parts
	require.Len(t, parts, 1)
	require.NotNil(t, parts[0].InlineData)
	assert.Equal(t, "audio/mpeg", parts[0].InlineData.MimeType)
	assert.Nil(t, api.asked.GenerationConfig, "nothing's asked for")

	// The segments, of each speaker's sentences, and of what's said before
	// a pause, with who spoke them, in the language
	api.reply = []string{`{"candidates":[{"content":{"parts":[
		{"audioTranscription":{"text":"Hello there. How are you?","speakerLabel":"spk:0","words":[
			{"word":"Hello","startOffset":"0.200s","endOffset":"0.500s"},
			{"word":"there.","startOffset":"0.500s","endOffset":"0.900s"},
			{"word":"How","startOffset":"1.200s","endOffset":"1.400s"},
			{"word":"are","startOffset":"1.400s","endOffset":"1.500s"},
			{"word":"you?","startOffset":"1.500s","endOffset":"2s"}]}},
		{"audioTranscription":{"text":"Fine thanks","speakerLabel":"spk:1","words":[
			{"word":"Fine","startOffset":"3s","endOffset":"3.400s"},
			{"word":"thanks","startOffset":"5.500s","endOffset":"6s"}]}}
	]},"finishReason":"STOP"}]}`}
	api.reply[0] = strings.NewReplacer("\n", "", "\t", "").Replace(api.reply[0])
	var segments []llm.TranscriptSegment
	transcript, _, err = c.Transcribe(ctx, model, audio(10), func(text string, segment *llm.TranscriptSegment) {
		if segment != nil {
			segments = append(segments, *segment)
		}
	}, llm.WithOpt(llm.SpeakersKey, true), llm.WithOpt(llm.LanguageKey, "en"))
	require.NoError(t, err)
	assert.Equal(t, "Hello there. How are you? Fine thanks", transcript.Text)
	assert.Equal(t, "en", transcript.Language)
	assert.Equal(t, []llm.TranscriptSegment{
		{Start: 200 * time.Millisecond, End: 900 * time.Millisecond, Text: "Hello there.", Speaker: "spk:0"},
		{Start: 1200 * time.Millisecond, End: 2 * time.Second, Text: "How are you?", Speaker: "spk:0"},
		{Start: 3 * time.Second, End: 3400 * time.Millisecond, Text: "Fine", Speaker: "spk:1"},
		{Start: 5500 * time.Millisecond, End: 6 * time.Second, Text: "thanks", Speaker: "spk:1"},
	}, transcript.Segments)
	assert.Equal(t, transcript.Segments, segments, "each is streamed")
	assert.Equal(t, &audioTranscriptionConfig{LanguageCodes: []string{"en"}, WordTimestamp: true, Diarization: true}, api.asked.GenerationConfig.TranscriptionConfig)

	// Segments, without speakers, are asked for with when each word is
	// spoken
	_, _, err = c.Transcribe(ctx, model, audio(10), nil, llm.WithOpt(llm.SegmentsKey, true))
	require.NoError(t, err)
	assert.Equal(t, &audioTranscriptionConfig{WordTimestamp: true}, api.asked.GenerationConfig.TranscriptionConfig)

	// An option which isn't supported, no audio, a model which doesn't
	// transcribe, and translation, aren't
	_, _, err = c.Transcribe(ctx, model, audio(10), nil, llm.WithOpt(llm.TemperatureKey, 0.5))
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	_, _, err = c.Transcribe(ctx, model, audio(0), nil)
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	_, _, err = c.Transcribe(ctx, chat, audio(10), nil)
	assert.ErrorIs(t, err, llm.ErrNotImplemented)
	_, _, err = c.Translate(ctx, model, audio(10), nil)
	assert.ErrorIs(t, err, llm.ErrNotImplemented)

	// A transcript which isn't finished is an error, as is one which is
	// longer than the model returns, as it repeated itself, or stopped
	api.reply = []string{`{"candidates":[{"content":{"parts":[{"audioTranscription":{"text":"Hello"}}]}}]}`}
	_, _, err = c.Transcribe(ctx, model, audio(10), nil)
	assert.ErrorIs(t, err, llm.ErrServiceUnavailable)
	api.reply = []string{`{"candidates":[{"content":{"parts":[{"audioTranscription":{"text":"Hello Hello"}}]},"finishReason":"MAX_TOKENS"}]}`}
	_, _, err = c.Transcribe(ctx, model, audio(10), nil)
	assert.ErrorIs(t, err, llm.ErrServiceUnavailable)
	assert.ErrorContains(t, err, "repeated itself")
	api.reply = []string{`{"candidates":[{"content":{"parts":[]},"finishReason":"SAFETY"}]}`}
	_, _, err = c.Transcribe(ctx, model, audio(10), nil)
	assert.ErrorContains(t, err, "SAFETY")
}

func Test_TranscribeUpload(t *testing.T) {
	// Audio which is larger than is sent with a request is uploaded, with its
	// length, and transcribed, once it's processed, by its URI, and deleted
	ctx := context.Background()
	api, c, _ := chatClient(t)
	model, err := c.GetModel(ctx, "gemini-new-transcribe")
	require.NoError(t, err)
	limit, interval := inlineLimit, fileInterval
	inlineLimit, fileInterval = 10, time.Millisecond
	t.Cleanup(func() { inlineLimit, fileInterval = limit, interval })
	api.processing = 2
	api.reply = []string{`{"candidates":[{"content":{"parts":[{"audioTranscription":{"text":"Hello"}}]},"finishReason":"STOP"}]}`}
	transcript, _, err := c.Transcribe(ctx, model, audio(20), nil)
	require.NoError(t, err)
	assert.Equal(t, "Hello", transcript.Text)
	assert.Equal(t, strings.Repeat("x", 20), string(api.uploaded))
	assert.Equal(t, int64(20), api.length, "with its length")
	parts := api.asked.Contents[0].Parts
	require.Len(t, parts, 1)
	assert.Nil(t, parts[0].InlineData)
	assert.Equal(t, &fileData{MimeType: "audio/mpeg", FileURI: api.URL + "/v1beta/files/abc"}, parts[0].FileData)
	assert.Equal(t, []string{"files/abc"}, api.deleted)
	assert.Zero(t, api.processing, "until it's processed")
}

func Test_TranscribeSegments(t *testing.T) {
	ctx := context.Background()
	api, c, _ := chatClient(t)
	model, err := c.GetModel(ctx, "gemini-new-transcribe")
	require.NoError(t, err)
	part := func(text, words string) string {
		return `{"candidates":[{"content":{"parts":[{"audioTranscription":{"text":"` + text + `","speakerLabel":"spk:0","words":[` + words + `]}}]}}]}`
	}
	word := func(text, start, end string) string {
		return `{"word":"` + text + `","startOffset":"` + start + `","endOffset":"` + end + `"}`
	}
	finished := `{"candidates":[{"content":{"parts":[]},"finishReason":"STOP"}]}`
	for _, test := range []struct {
		name     string
		reply    []string
		text     string
		segments []string
	}{
		{"unspaced", []string{part("你好世界。", word("你好", "0s", "0.5s")+","+word("世界。", "0.5s", "1s")), finished},
			"你好世界。", []string{"你好世界。"}},
		{"across parts", []string{part("Hello", word("Hello", "0s", "0.3s")), part("there.", word("there.", "0.4s", "0.7s")), finished},
			"Hello there.", []string{"Hello there."}},
		{"unspaced, across parts", []string{part("你好", word("你好", "0s", "0.5s")), part("世界。", word("世界。", "0.5s", "1s")), finished},
			"你好世界。", []string{"你好世界。"}},
		{"the same, across parts", []string{part("Hello", word("Hello", "0s", "0.3s")), part("Hello", word("Hello", "0.4s", "0.7s")), finished},
			"Hello Hello", []string{"Hello Hello"}},
		{"a word which isn't in the text", []string{part("Hello, world.", word("Hello,", "0s", "0.3s")+","+word("wurld.", "0.4s", "0.7s")), finished},
			"Hello, world.", []string{"Hello, wurld."}},
		{"spacing, as the model spaces it", []string{part("It costs £5 , really.", word("It", "0s", "0.1s")+","+word("costs", "0.1s", "0.3s")+","+word("£5", "0.3s", "0.5s")+","+word("really.", "0.6s", "0.9s")), finished},
			"It costs £5 , really.", []string{"It costs £5 , really."}},
	} {
		api.reply = test.reply
		var streamed []string
		transcript, _, err := c.Transcribe(ctx, model, audio(10), func(_ string, segment *llm.TranscriptSegment) {
			if segment != nil {
				streamed = append(streamed, segment.Text)
			}
		}, llm.WithOpt(llm.SegmentsKey, true))
		require.NoError(t, err, test.name)
		assert.Equal(t, test.text, transcript.Text, test.name)
		var segments []string
		for _, segment := range transcript.Segments {
			segments = append(segments, segment.Text)
		}
		assert.Equal(t, test.segments, segments, test.name)
		assert.Equal(t, test.segments, streamed, test.name)
	}
}

func Test_SentenceEnds(t *testing.T) {
	for word, ends := range map[string]bool{
		"dog.": true, "you?": true, "now!": true, `said."`: true, "it.)": true, "终。": true,
		"Mr.": false, "Dr.": false, "J.": false, "U.S.": false, "e.g.": false, "Miami,": false, "word": false,
	} {
		assert.Equal(t, ends, sentenceEnds(word), word)
	}
}

// kept is audio which is read from where it's kept, as it's read, whose
// length is known, unless it's -1, and which counts how often it's read
type kept struct {
	data   string
	length int64
	reads  int
}

func (k *kept) URI() string         { return "resource://abc" }
func (k *kept) Name() string        { return "talk.mp3" }
func (k *kept) Description() string { return "" }
func (k *kept) Type() string        { return "audio/mpeg" }
func (k *kept) ContentLength() int64 {
	return k.length
}
func (k *kept) Read(context.Context) (llm.ReadFunc, error) {
	return func(_ context.Context, fn func(io.Reader) error) error {
		k.reads++
		return fn(strings.NewReader(k.data))
	}, nil
}

func Test_TranscribeKept(t *testing.T) {
	// Audio which is kept elsewhere, of a length which is known, is uploaded
	// as it's read, with its length, rather than read first, and audio of a
	// length which isn't is read, to know it, and then uploaded
	ctx := context.Background()
	api, c, _ := chatClient(t)
	model, err := c.GetModel(ctx, "gemini-new-transcribe")
	require.NoError(t, err)
	limit := inlineLimit
	inlineLimit = 10
	t.Cleanup(func() { inlineLimit = limit })
	for _, length := range []int64{20, -1} {
		api.uploaded, api.length, api.deleted = nil, 0, nil
		api.reply = []string{`{"candidates":[{"content":{"parts":[{"audioTranscription":{"text":"Hello"}}]},"finishReason":"STOP"}]}`}
		audio := &kept{data: strings.Repeat("y", 20), length: length}
		transcript, _, err := c.Transcribe(ctx, model, audio, nil)
		require.NoError(t, err, length)
		assert.Equal(t, "Hello", transcript.Text)
		assert.Equal(t, strings.Repeat("y", 20), string(api.uploaded), length)
		assert.Equal(t, int64(20), api.length, "with its length")
		assert.Equal(t, 1, audio.reads, "it's read once")
		assert.Equal(t, []string{"files/abc"}, api.deleted)
		assert.Nil(t, api.asked.Contents[0].Parts[0].InlineData)
	}
}
