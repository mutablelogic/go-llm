package llm

import "strings"

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	mimeTypeText = "text/plain"
	mimeTypeJSON = "application/json"
	mimeTypeMP3  = "audio/mpeg"
	mimeTypeOpus = "audio/opus"
	mimeTypeAAC  = "audio/aac"
	mimeTypeFLAC = "audio/flac"
	mimeTypeWAV  = "audio/wav"
	mimeTypePCM  = "audio/pcm"
)

var (
	// Acceptable formats
	formatMap = map[string]string{
		mimeTypeText:  "text",
		"text":        "text",
		mimeTypeJSON:  "json_object",
		"json":        "json_object",
		"json_object": "json_object",
		"image":       "image",
		mimeTypeMP3:   "audio",
		mimeTypeOpus:  "audio",
		mimeTypeAAC:   "audio",
		mimeTypeFLAC:  "audio",
		mimeTypeWAV:   "audio",
		mimeTypePCM:   "audio",
		"audio":       "audio",
		"mp3":         "audio",
		"opus":        "audio",
		"aac":         "audio",
		"flac":        "audio",
		"wav":         "audio",
		"pcm":         "audio",
	}
	audioValues = []string{
		"mp3", "opus", "aac", "flac", "wav", "pcm",
	}
	qualityValues = []string{
		"standard", "hd",
	}
	imageSizeValues = []string{
		"256x256", "512x512", "1024x1024", "1792x1024", "1024x1792",
	}
	styleValues = []string{
		"natural", "vivid",
	}
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Set format for output, which is dependent on the model used
func WithFormat(v any) Opt {
	return func(o *Opts) error {
		v_, ok := v.(string)
		if !ok {
			return ErrBadParameter.Withf("format %T unsupported", v)
		}
		format, exists := formatMap[strings.TrimSpace(strings.ToLower(v_))]
		if !exists {
			return ErrBadParameter.Withf("format %q unsupported", v_)
		}
		o.Set("format", format)
		return nil
	}
}

// Set quality for output (DALL-E)
func WithQuality(v string) Opt {
	return func(o *Opts) error {
		v = strings.TrimSpace(strings.ToLower(v))
		o.Set("quality", v)
		return nil
	}
}

// Set size for output (DALL-E)
func WithSize(v string) Opt {
	return func(o *Opts) error {
		v = strings.TrimSpace(strings.ToLower(v))
		o.Set("size", v)
		return nil
	}
}

// Set style for output (DALL-E)
func WithStyle(v string) Opt {
	return func(o *Opts) error {
		v = strings.TrimSpace(strings.ToLower(v))
		o.Set("style", v)
		return nil
	}
}
