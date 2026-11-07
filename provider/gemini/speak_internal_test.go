package gemini

import (
	"context"
	"encoding/base64"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/provider/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_Speak(t *testing.T) {
	ctx := context.Background()
	api, c, chat := chatClient(t)
	model, err := c.GetModel(ctx, "gemini-new-preview-tts")
	require.NoError(t, err)
	pcm := base64.StdEncoding.EncodeToString([]byte{1, 2, 3, 4})

	// PCM, from the model, is WAV, in the voice, as the instructions say,
	// with the tokens used
	api.made = `{"candidates":[{"content":{"role":"model","parts":[{"inlineData":{"mimeType":"audio/L16;codec=pcm;rate=16000","data":"` + pcm + `"}}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":6,"candidatesTokenCount":40}}`
	audio, usage, err := c.Speak(ctx, model, "Hello", llm.WithOpt(llm.VoiceKey, "Kore"), llm.WithOpt(llm.InstructionsKey, "calmly"))
	require.NoError(t, err)
	assert.Equal(t, "audio/wav", audio.Type())
	data, err := llm.ReadAll(ctx, audio)
	require.NoError(t, err)
	samples, rate, channels, ok := schema.WAVSamples(data)
	require.True(t, ok)
	assert.Equal(t, []byte{1, 2, 3, 4}, samples)
	assert.Equal(t, 16000, rate)
	assert.Equal(t, 1, channels)
	assert.Equal(t, uint(6), usage.InputTokens)
	assert.Equal(t, uint(40), usage.OutputTokens)
	config := api.asked.GenerationConfig
	assert.Equal(t, []string{"AUDIO"}, config.ResponseModalities)
	require.NotNil(t, config.SpeechConfig)
	assert.Equal(t, "Kore", config.SpeechConfig.VoiceConfig.PrebuiltVoiceConfig.VoiceName)
	text := api.asked.Contents[0].Parts[0]
	assert.Equal(t, "Hello", text.Text)
	assert.Equal(t, &speechMetadata{Style: "calmly"}, text.SpeechMetadata)

	// PCM, when it's asked for, is audio/L16, which is big-endian, and WAV,
	// from the model, is PCM, too, or WAV
	audio, _, err = c.Speak(ctx, model, "Hello", llm.WithOpt(llm.AudioFormatKey, "pcm"))
	require.NoError(t, err)
	assert.Equal(t, "audio/l16; channels=1; rate=16000", audio.Type())
	data, _ = llm.ReadAll(ctx, audio)
	assert.Equal(t, []byte{2, 1, 4, 3}, data)
	assert.Nil(t, api.asked.GenerationConfig.SpeechConfig, "no voice, so the model's")
	wav := schema.WAV16([]byte{1, 2, 3, 4}, 24000, 1).Data
	api.made = `{"candidates":[{"content":{"parts":[{"inlineData":{"mimeType":"audio/wav","data":"` + base64.StdEncoding.EncodeToString(wav) + `"}}]}}]}`
	audio, _, err = c.Speak(ctx, model, "Hello", llm.WithOpt(llm.AudioFormatKey, "pcm"))
	require.NoError(t, err)
	assert.Equal(t, "audio/l16; channels=1; rate=24000", audio.Type())
	audio, _, err = c.Speak(ctx, model, "Hello")
	require.NoError(t, err)
	data, _ = llm.ReadAll(ctx, audio)
	assert.Equal(t, wav, data)

	// No text, a speed, a format which isn't WAV or PCM, and a model which
	// doesn't speak, aren't
	_, _, err = c.Speak(ctx, model, " ")
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	_, _, err = c.Speak(ctx, model, "Hello", llm.WithOpt(llm.SpeedKey, 1.5))
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	_, _, err = c.Speak(ctx, model, "Hello", llm.WithOpt(llm.AudioFormatKey, "mp3"))
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	_, _, err = c.Speak(ctx, chat, "Hello")
	assert.ErrorIs(t, err, llm.ErrNotImplemented)

	// No audio is an error, as is audio which is empty
	api.made = `{"candidates":[{"content":{"parts":[{"text":"I can't."}]}}]}`
	_, _, err = c.Speak(ctx, model, "Hello")
	assert.ErrorIs(t, err, llm.ErrServiceUnavailable)
	api.made = `{"candidates":[{"content":{"parts":[{"inlineData":{"mimeType":"audio/L16;codec=pcm;rate=24000","data":""}}]}}]}`
	_, _, err = c.Speak(ctx, model, "Hello")
	assert.ErrorIs(t, err, llm.ErrServiceUnavailable)
}

func Test_SpeakDirected(t *testing.T) {
	// A model which doesn't take a style is told how to speak in the text:
	// Gemini 2.5 before it, and later models as a tag
	ctx := context.Background()
	api, c, _ := chatClient(t)
	api.noStyle = true
	api.made = `{"candidates":[{"content":{"parts":[{"inlineData":{"mimeType":"audio/L16;codec=pcm;rate=24000","data":"AQI="}}]}}]}`
	for _, test := range []struct{ model, instructions, text string }{
		{"gemini-2.5-flash-preview-tts", "calmly", "Say this calmly: Hello"},
		{"gemini-2.5-flash-preview-tts", "Read it slowly.", "Read it slowly: Hello"},
		{"gemini-3.1-flash-tts-preview", "whispering", "[whispering] Hello"},
	} {
		model := &Model{provider: c, data: apiModel{Name: "models/" + test.model, SupportedGenerationMethods: []string{"generateContent"}}}
		_, _, err := c.Speak(ctx, model, "Hello", llm.WithOpt(llm.InstructionsKey, test.instructions))
		require.NoError(t, err, test.model)
		text := api.asked.Contents[0].Parts[0]
		assert.Equal(t, test.text, text.Text, test.model)
		assert.Nil(t, text.SpeechMetadata, test.model)
	}
}
