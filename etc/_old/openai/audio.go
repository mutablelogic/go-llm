package openai

import (
	"context"
	"io"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

type reqAudioCompletion struct {
	Model          string  `json:"model"`
	Input          string  `json:"input"`
	Voice          string  `json:"voice"`
	Speed          float64 `json:"speed,omitempty"`
	ResponseFormat string  `json:"response_format,omitempty"`
}

type responseAudio struct {
	audio *llm.Attachment
}

// Send a completion request with text for text-to-speech
func (model *model) audioCompletion(ctx context.Context, input string, opt *llm.Opts) (llm.Completion, error) {
	// Request
	req, err := client.NewJSONRequest(reqAudioCompletion{
		Model:          model.Name(),
		Input:          input,
		Voice:          optVoice(opt),
		Speed:          optSpeed(opt),
		ResponseFormat: optAudioFormat(opt),
	})
	if err != nil {
		return nil, err
	}

	// Response
	var response responseAudio
	if err := model.DoWithContext(ctx, req, &response, client.OptPath("audio", "speech")); err != nil {
		return nil, err
	}

	return &response, nil
}

func (resp *responseAudio) Unmarshal(mimetype string, r io.Reader) error {
	// Unmarshal the response
	attachment, err := llm.ReadAttachment(r, mimetype)
	if err != nil {
		return err
	} else {
		resp.audio = attachment
	}
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// COMPLETION

// Return the number of completions
func (r *responseAudio) Num() int {
	return 1
}

// Return message for a specific completion
func (r *responseAudio) Choice(index int) llm.Completion {
	if index != 0 {
		return nil
	}
	return r
}

// Return the role of the completion
func (r *responseAudio) Role() string {
	return "assistant"
}

// Unsupported
func (r *responseAudio) Text(index int) string {
	return ""
}

// Return media content for a specific completion
func (r *responseAudio) Attachment(index int) *llm.Attachment {
	if index != 0 {
		return nil
	} else {
		return r.audio
	}
}

// Unsupported
func (r *responseAudio) ToolCalls(index int) []llm.ToolCall {
	return nil
}
