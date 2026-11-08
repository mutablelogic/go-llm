package llm_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	// Packages
	gollm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	llm "github.com/mutablelogic/go-llm/toolkit/llm"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// encoder is a fake of a converter of voice notes, which keeps the audio it
// was given, and returns it, as Ogg
type encoder struct {
	given []string
}

func (e *encoder) Convert(ctx context.Context, audio gollm.Resource) (gollm.Resource, error) {
	data, err := gollm.ReadAll(ctx, audio)
	if err != nil {
		return nil, err
	}
	e.given = append(e.given, audio.Type()+":"+string(data))
	if !strings.HasPrefix(audio.Type(), "audio/wav") {
		return nil, gollm.ErrNotImplemented.Withf("audio of type %q isn't encoded as a voice note", audio.Type())
	}
	return &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: "audio/ogg", Data: append([]byte("OggS"), data...)}}, nil
}

func Test_VoiceNote(t *testing.T) {
	ctx := context.Background()
	speak := func(t *testing.T, m *models, opts ...llm.Opt) gollm.Tool {
		t.Helper()
		ns, err := llm.New(func() llm.Models { return m }, append([]llm.Opt{llm.WithSpeak("phonon", "")}, opts...)...)
		require.NoError(t, err)
		return ns.Tools()[0]
	}
	input := json.RawMessage(`{"text":"Hello","name":"greeting","format":"voice-note"}`)

	t.Run("Native", func(t *testing.T) {
		// A model which speaks Opus speaks the voice note, which isn't
		// encoded
		m, e := &models{opus: "native"}, &encoder{}
		result, err := speak(t, m, llm.WithVoiceNoteConverter(e)).Run(ctx, input)
		require.NoError(t, err)
		audio := result.(gollm.Resource)
		assert.Equal(t, "greeting.ogg", audio.Name())
		assert.Equal(t, "audio/ogg", audio.Type())
		assert.Equal(t, 1, m.spoken)
		assert.Empty(t, e.given)
	})

	t.Run("Encoded", func(t *testing.T) {
		// A model which can't speak Opus speaks its own format, which is
		// encoded, whether it rejects Opus, or ignores it
		for _, opus := range []string{"reject", ""} {
			m, e := &models{opus: opus}, &encoder{}
			result, err := speak(t, m, llm.WithVoiceNoteConverter(e)).Run(ctx, input)
			require.NoError(t, err)
			audio := result.(gollm.Resource)
			assert.Equal(t, "greeting.ogg", audio.Name(), opus)
			assert.Equal(t, "audio/ogg", audio.Type())
			assert.Equal(t, []string{"audio/wav:Hello"}, e.given)
			data, err := gollm.ReadAll(ctx, audio)
			require.NoError(t, err)
			assert.Equal(t, "OggSHello", string(data))
			if opus == "reject" {
				assert.Equal(t, 2, m.spoken, "spoken again, in its own format")
				assert.Nil(t, m.opts.Get(gollm.AudioFormatKey))
			} else {
				assert.Equal(t, 1, m.spoken)
			}
		}
	})

	t.Run("Without an encoder", func(t *testing.T) {
		// Without an encoder, the model's own format is returned, with a
		// note which says why
		m := &models{opus: "reject"}
		result, err := speak(t, m).Run(ctx, input)
		require.NoError(t, err)
		r, ok := result.(toolkit.Result)
		require.True(t, ok)
		require.Len(t, r.Resources, 1)
		assert.Equal(t, "greeting.wav", r.Resources[0].Name())
		note, err := json.Marshal(r.Value)
		require.NoError(t, err)
		assert.Contains(t, string(note), "opusenc")
	})

	t.Run("Without a format", func(t *testing.T) {
		// Without one, it's the model's own, without Opus asked for
		m, e := &models{opus: "native"}, &encoder{}
		result, err := speak(t, m, llm.WithVoiceNoteConverter(e)).Run(ctx, json.RawMessage(`{"text":"Hello"}`))
		require.NoError(t, err)
		assert.Equal(t, "speech.wav", result.(gollm.Resource).Name())
		_, err = speak(t, m).Run(ctx, json.RawMessage(`{"text":"Hello","format":"mp4"}`))
		assert.ErrorIs(t, err, gollm.ErrBadParameter)
		_, err = llm.New(func() llm.Models { return m }, llm.WithVoiceNoteConverter(nil))
		assert.ErrorIs(t, err, gollm.ErrBadParameter)
	})
}
