package openai

import (
	"context"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type ImageResponse struct {
	Created uint64   `json:"created"`
	Data    []*Image `json:"data"`
}

type Image llm.ImageMeta

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

type reqImageCompletion struct {
	Model          string `json:"model"`
	Prompt         string `json:"prompt"`
	NumCompletions uint64 `json:"n,omitempty"`
	Quality        string `json:"quality,omitempty"`
	ResponseFormat string `json:"response_format,omitempty"`
	Size           string `json:"size,omitempty"`
	Style          string `json:"style,omitempty"`
	User           string `json:"user,omitempty"`
}

// Send a completion request with a prompt to generate an image
func (model *model) imageCompletion(ctx context.Context, prompt string, opt *llm.Opts) (llm.Completion, error) {
	// Request
	req, err := client.NewJSONRequest(reqImageCompletion{
		Model:          model.Name(),
		Prompt:         prompt,
		NumCompletions: optNumCompletions(opt),
		Quality:        optQuality(opt),
		ResponseFormat: "b64_json",
		Size:           optSize(opt),
		Style:          optStyle(opt),
		User:           optUser(opt),
	})
	if err != nil {
		return nil, err
	}

	// Response
	var response ImageResponse
	if err := model.DoWithContext(ctx, req, &response, client.OptPath("images", "generations")); err != nil {
		return nil, err
	}

	// Add the caption in the prompt
	for _, completion := range response.Data {
		if completion.Prompt == "" {
			completion.Prompt = prompt
		}
	}

	// Return success
	return &response, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// Return the number of completions
func (r ImageResponse) Num() int {
	return len(r.Data)
}

// Return message for a specific completion
func (r ImageResponse) Choice(index int) llm.Completion {
	if index < 0 || index >= r.Num() {
		return nil
	}
	return r.Data[index]
}

// Return the role of the completion
func (r ImageResponse) Role() string {
	return "assistant"
}

// Return the text content for a specific completion
func (r ImageResponse) Text(index int) string {
	return r.Choice(index).Text(0)
}

// Return media content for a specific completion
func (r ImageResponse) Attachment(index int) *llm.Attachment {
	return r.Choice(index).Attachment(0)
}

// Unsupported
func (r ImageResponse) ToolCalls(index int) []llm.ToolCall {
	return nil
}

// Return the number of completions
func (r *Image) Num() int {
	return 1
}

// Return message for a specific completion
func (r *Image) Choice(index int) llm.Completion {
	if index != 0 {
		return nil
	}
	return r
}

// Return the role of the completion
func (r *Image) Role() string {
	return "assistant"
}

// Return the text prompt
func (r *Image) Text(index int) string {
	if index != 0 {
		return ""
	}
	return r.Prompt
}

// Return media content for a specific completion
func (r *Image) Attachment(index int) *llm.Attachment {
	if index != 0 {
		return nil
	}

	// Make the attachment
	return llm.NewAttachmentWithImage((*llm.ImageMeta)(r))
}

// Unsupported
func (r *Image) ToolCalls(index int) []llm.ToolCall {
	return nil
}
