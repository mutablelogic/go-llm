package gowhisper

import (
	"strings"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// apiModel is a model, as go-whisper lists it, with its id, which is its
// file's name without its extension, its path, which is its file's name,
// when it was downloaded, as seconds since the epoch, and who owns it, which
// is "whisper" for its own models, or the name of a service it forwards to,
// such as "openai" or "elevenlabs"
// Reference: https://github.com/mutablelogic/go-whisper/blob/main/pkg/schema/model.go
type apiModel struct {
	Id      string `json:"id"`
	Object  string `json:"object,omitempty"`
	Path    string `json:"path,omitempty"`
	Created int64  `json:"created,omitempty"`
	OwnedBy string `json:"owned_by,omitempty"`
}

// transcribeRequest is a request to transcribe, or translate, the speech in
// audio, which is a multipart form, with the audio as a file, its name,
// which go-whisper decodes it by, and the language
// Reference: https://github.com/mutablelogic/go-whisper/blob/main/pkg/schema/request.go
type transcribeRequest struct {
	Model       string     `json:"model"`
	Audio       types.File `json:"audio"`
	Filename    string     `json:"filename,omitempty"`
	Language    string     `json:"language,omitempty"`
	Temperature *float64   `json:"temperature,omitempty"`
}

// transcribeResponse is a transcript, with the language it was spoken in,
// as whisper names it, such as "english", how long it is, in seconds, its
// text, and its segments
type transcribeResponse struct {
	Task     string       `json:"task"`
	Language string       `json:"language"`
	Duration float64      `json:"duration"`
	Text     string       `json:"text"`
	Segments []apiSegment `json:"segments"`
}

// apiSegment is a segment of a transcript, from when to when, in seconds,
// and its speaker, when it's known
type apiSegment struct {
	Id      int32   `json:"id"`
	Start   float64 `json:"start"`
	End     float64 `json:"end"`
	Text    string  `json:"text"`
	Speaker string  `json:"speaker,omitempty"`
}

// downloadRequest is a request to download a model, by its file's name,
// such as "ggml-tiny.en.bin"
type downloadRequest struct {
	Model string `json:"model"`
}

// downloadProgress is the progress of a download, in bytes
type downloadProgress struct {
	Current uint64 `json:"current"`
	Total   uint64 `json:"total"`
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// ownedBy is the owner of go-whisper's own models, which it runs, rather
	// than forwards to another service
	ownedBy = "whisper"

	// modelExt is the extension of a model's file
	modelExt = ".bin"

	// The events of a download, which are its progress, an error, and the
	// model, once it's downloaded
	eventProgress = "download.progress"
	eventError    = "download.error"
	eventDone     = "download.done"

	// The events of a transcript, which are each segment, as it's
	// transcribed, an error, and the transcript, once it's transcribed,
	// without its text, which is its segments'
	eventSegment         = "transcript.text.delta"
	eventTranscribeError = "transcript.text.error"
	eventTranscribeDone  = "transcript.text.done"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// transcript returns the segment of a transcript, from when to when, its
// text, and who spoke it, when it's known
func (s apiSegment) transcript() llm.TranscriptSegment {
	return llm.TranscriptSegment{
		Start:   time.Duration(s.Start * float64(time.Second)),
		End:     time.Duration(s.End * float64(time.Second)),
		Text:    strings.TrimSpace(s.Text),
		Speaker: s.Speaker,
	}
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (m apiModel) String() string {
	return types.Stringify(m)
}
