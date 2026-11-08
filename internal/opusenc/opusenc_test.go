package opusenc_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	// Packages
	gollm "github.com/mutablelogic/go-llm"
	opusenc "github.com/mutablelogic/go-llm/internal/opusenc"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_Convert(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake opusenc is a shell script")
	}
	ctx := context.Background()

	// A fake opusenc, which keeps its arguments, and writes Ogg, and what it
	// read, or fails, when it's asked to
	dir := t.TempDir()
	args := filepath.Join(dir, "args")
	script := filepath.Join(dir, "opusenc")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\necho \"$@\" > "+args+"\nif [ \"$FAIL\" = 1 ]; then echo 'bad input' >&2; exit 1; fi\nprintf OggS\ncat\n"), 0o755))
	converter, err := opusenc.New(script)
	require.NoError(t, err)
	assert.Equal(t, script, converter.Path())
	encode := converter.Convert

	// WAV is encoded, from its input, to its output
	wav := &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: "audio/wav", Data: []byte("RIFF")}}
	note, err := encode(ctx, wav)
	require.NoError(t, err)
	assert.Equal(t, "audio/ogg", note.Type())
	data, err := gollm.ReadAll(ctx, note)
	require.NoError(t, err)
	assert.Equal(t, "OggSRIFF", string(data))
	got, err := os.ReadFile(args)
	require.NoError(t, err)
	assert.Equal(t, "--quiet --bitrate 32 - -", strings.TrimSpace(string(got)))

	// Raw PCM is, with its rate, and channels, as big-endian
	pcm := &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: "audio/L16; rate=24000; channels=1", Data: []byte{0, 1}}}
	_, err = encode(ctx, pcm)
	require.NoError(t, err)
	got, err = os.ReadFile(args)
	require.NoError(t, err)
	assert.Equal(t, "--quiet --bitrate 32 --raw --raw-bits 16 --raw-rate 24000 --raw-chan 1 --raw-endianness 1 - -", strings.TrimSpace(string(got)))

	// Audio it doesn't read isn't, and its errors say why
	_, err = encode(ctx, &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: "audio/mpeg", Data: []byte("ID3")}})
	assert.ErrorIs(t, err, gollm.ErrNotImplemented)
	_, err = encode(ctx, &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: "audio/L16", Data: []byte{0}}})
	assert.ErrorIs(t, err, gollm.ErrNotImplemented, "raw PCM without a rate")
	t.Setenv("FAIL", "1")
	_, err = encode(ctx, wav)
	assert.ErrorContains(t, err, "bad input")
}

func Test_ConvertInstalled(t *testing.T) {
	converter, err := opusenc.New("")
	if err != nil {
		t.Skip("opusenc, of opus-tools, isn't installed")
	}
	ctx := context.Background()
	encode := converter.Convert

	// A second of a tone, as 16-bit samples, at 24 kHz, in one channel
	const rate = 24000
	samples := make([]int16, rate)
	for i := range samples {
		samples[i] = int16(8000 * math.Sin(2*math.Pi*440*float64(i)/rate))
	}

	// As WAV, which is little-endian, and as raw PCM, audio/L16, which is
	// big-endian, each of which is a voice note, which is Ogg Opus
	var wav bytes.Buffer
	wav.WriteString("RIFF")
	_ = binary.Write(&wav, binary.LittleEndian, uint32(36+2*len(samples)))
	wav.WriteString("WAVEfmt ")
	for _, field := range []any{uint32(16), uint16(1), uint16(1), uint32(rate), uint32(rate * 2), uint16(2), uint16(16)} {
		require.NoError(t, binary.Write(&wav, binary.LittleEndian, field))
	}
	wav.WriteString("data")
	_ = binary.Write(&wav, binary.LittleEndian, uint32(2*len(samples)))
	_ = binary.Write(&wav, binary.LittleEndian, samples)
	var pcm bytes.Buffer
	_ = binary.Write(&pcm, binary.BigEndian, samples)
	for _, audio := range []*providerschema.Resource{
		{ResourceMeta: providerschema.ResourceMeta{ContentType: "audio/wav", Data: wav.Bytes()}},
		{ResourceMeta: providerschema.ResourceMeta{ContentType: "audio/L16; rate=24000; channels=1", Data: pcm.Bytes()}},
	} {
		note, err := encode(ctx, audio)
		require.NoError(t, err, audio.ContentType)
		data, err := gollm.ReadAll(ctx, note)
		require.NoError(t, err)
		assert.True(t, bytes.HasPrefix(data, []byte("OggS")), audio.ContentType)
		assert.Contains(t, string(data), "OpusHead", audio.ContentType)
		assert.Less(t, len(data), len(audio.Data)/4, "smaller than what it was encoded from")
	}
}
