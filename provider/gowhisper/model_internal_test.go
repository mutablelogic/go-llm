package gowhisper

import (
	"testing"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	assert "github.com/stretchr/testify/assert"
)

func Test_ModelID(t *testing.T) {
	// An id is itself, and a file is its base name, as go-whisper makes its
	// id, in lower case, without its extension, with other characters as
	// underscores
	for name, want := range map[string]string{
		"ggml-tiny.en":             "ggml-tiny.en",
		"ggml-tiny.en.bin":         "ggml-tiny.en",
		"english/ggml-tiny.en.bin": "ggml-tiny.en",
		"Models/GGML Tiny+1.BIN":   "ggml_tiny_1",
		" ggml-medium-q5_0.bin ":   "ggml-medium-q5_0",
	} {
		assert.Equal(t, want, modelID(name), name)
	}
}

func Test_Stream(t *testing.T) {
	// The segments' text, as it is, without the space at the start, so a
	// language without spaces has none between its segments
	var texts []string
	s := transcribeStream{stream: func(text string, _ *llm.TranscriptSegment) { texts = append(texts, text) }}
	for _, data := range []string{`{"id":0,"start":0,"end":1,"text":" 你好"}`, `{"id":1,"start":1,"end":2,"text":"世界。"}`} {
		assert.NoError(t, s.add(client.TextStreamEvent{Event: eventSegment, Data: data}))
	}
	assert.Equal(t, []string{"你好", "世界。"}, texts)
	assert.Equal(t, "你好世界。", s.response.Text)
}

func Test_EventErr(t *testing.T) {
	// An error's status is its text, which is a request which isn't valid,
	// or isn't found, or otherwise its message
	assert.ErrorIs(t, eventErr(`"Bad Request: temperature must be between 0 and 1"`), llm.ErrBadParameter)
	assert.ErrorIs(t, eventErr(`"Not Found: 404 Not Found"`), llm.ErrNotFound)
	assert.ErrorIs(t, downloadErr("ggml-tiny.bin", `"Not Found: 404 Not Found"`), llm.ErrNotFound)
	assert.EqualError(t, eventErr(`"cannot decode audio"`), "cannot decode audio")
	assert.EqualError(t, eventErr(`not json`), "not json")
}
