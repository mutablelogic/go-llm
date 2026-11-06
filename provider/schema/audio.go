package schema

import (
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
