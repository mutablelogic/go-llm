package schema

import (
	"bytes"
	"encoding/binary"
	"maps"
	"mime"
	"path"
	"slices"
	"strconv"
	"strings"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// audioExtensions are the extensions of audio files, by their type, as an
// API may detect an audio file's format from its name, and Go doesn't know
// the extensions of all of them
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

///////////////////////////////////////////////////////////////////////////////
// PUBLIC FUNCTIONS

// AudioName returns the name of an audio file, which is its name, when it
// has the extension of an audio file, or otherwise "audio", with the
// extension of its type, such as "audio.wav" for audio/wave, as an API may
// detect an audio file's format from its name
func AudioName(name, contentType string) string {
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

// PCM16 returns raw audio, of signed 16-bit little-endian samples, at the
// rate, with the channels, as speech models return PCM, as audio/L16,
// whose samples are big-endian, and which has its rate and channels, so
// it's the audio it says it is. A trailing odd byte is left out.
func PCM16(data []byte, rate, channels int) *Resource {
	samples := make([]byte, len(data)&^1)
	for i := 0; i+1 < len(data); i += 2 {
		samples[i], samples[i+1] = data[i+1], data[i]
	}
	contentType := mime.FormatMediaType("audio/L16", map[string]string{"rate": strconv.Itoa(rate), "channels": strconv.Itoa(channels)})
	return &Resource{ResourceMeta: ResourceMeta{ContentType: contentType, Data: samples}}
}

// WAV16 returns raw audio, of signed 16-bit little-endian samples, at the
// rate, with the channels, as a WAV file, audio/wav, which can be played,
// as audio/L16 often can't be. A trailing odd byte is left out.
func WAV16(data []byte, rate, channels int) *Resource {
	samples := data[:len(data)&^1]
	var wav bytes.Buffer
	wav.Grow(44 + len(samples))
	header := []any{
		[4]byte{'R', 'I', 'F', 'F'}, uint32(36 + len(samples)), [4]byte{'W', 'A', 'V', 'E'},
		[4]byte{'f', 'm', 't', ' '}, uint32(16), uint16(1), uint16(channels), uint32(rate),
		uint32(rate * channels * 2), uint16(channels * 2), uint16(16),
		[4]byte{'d', 'a', 't', 'a'}, uint32(len(samples)),
	}
	for _, field := range header {
		_ = binary.Write(&wav, binary.LittleEndian, field)
	}
	wav.Write(samples)
	return &Resource{ResourceMeta: ResourceMeta{ContentType: "audio/wav", Data: wav.Bytes()}}
}

// WAVSamples returns the samples of a WAV file, from its data chunk, its
// rate, and channels, of 16-bit PCM, or false when it isn't one
func WAVSamples(data []byte) ([]byte, int, int, bool) {
	if len(data) < 12 || string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return nil, 0, 0, false
	}
	var rate, channels int
	for chunk := data[12:]; len(chunk) >= 8; {
		id, size := string(chunk[0:4]), int(binary.LittleEndian.Uint32(chunk[4:8]))
		body := chunk[8:]
		if size > len(body) {
			size = len(body)
		}
		switch id {
		case "fmt ":
			if size < 16 || binary.LittleEndian.Uint16(body[0:2]) != 1 || binary.LittleEndian.Uint16(body[14:16]) != 16 {
				return nil, 0, 0, false
			}
			channels, rate = int(binary.LittleEndian.Uint16(body[2:4])), int(binary.LittleEndian.Uint32(body[4:8]))
		case "data":
			if rate == 0 {
				return nil, 0, 0, false
			}
			return body[:size], rate, channels, true
		}
		// Chunks are padded to an even size
		next := size + size&1
		if next > len(body) {
			break
		}
		chunk = body[next:]
	}
	return nil, 0, 0, false
}
