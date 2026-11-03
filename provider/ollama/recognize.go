package ollama

import (
	"context"
	"regexp"
	"slices"
	"strings"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/provider/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// INTERFACES

var _ llm.Recognizer = (*Client)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// ocrModel is a model which recognizes text, by a regular expression of its
// name, and its prompt, as each is trained on its own, and is sensitive to it
type ocrModel struct {
	re     *regexp.Regexp
	prompt string
}

// ocrModels are the models which recognize text
var ocrModels = []ocrModel{
	{regexp.MustCompile(`(^|/)deepseek-ocr\b`), "Convert the document to markdown."},
	{regexp.MustCompile(`(^|/)glm-ocr\b`), "Text Recognition:"},
}

// recognizePrompt is the prompt of other models which see images
const recognizePrompt = "Convert the document to markdown."

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Recognize returns the text in the image, as a message with the text of
// its one page, in Markdown, and the tokens used, and the page. The model
// must be an Ollama model which sees images, such as deepseek-ocr, which is
// asked with its own prompt. Ollama's models read images, not documents, so
// a PDF isn't recognized. An image should be large enough for its text to be
// read, as a model doesn't read small text. The request isn't timed out, as
// a model can take a while to load, but is cancelled with the context.
func (c *Client) Recognize(ctx context.Context, model llm.Model, resource llm.Resource, opts ...llm.Opt) (llm.Message, llm.Usage, error) {
	if model == nil || model.Provider() == nil || model.Provider().Name() != llm.Ollama {
		return nil, llm.Usage{}, llm.ErrBadParameter.With("not an ollama model")
	} else if resource == nil {
		return nil, llm.Usage{}, llm.ErrBadParameter.With("missing resource")
	} else if !isImage(resource.Type()) {
		return nil, llm.Usage{}, llm.ErrBadParameter.Withf("ollama recognizes text in images, not %q", resource.Type())
	}

	// The image, and the prompt of the model
	data, err := llm.ReadAll(ctx, resource)
	if err != nil {
		return nil, llm.Usage{}, err
	} else if len(data) == 0 {
		return nil, llm.Usage{}, llm.ErrBadParameter.With("image is empty")
	}
	request := recognizeRequest{chatRequest{
		Model:    model.Name(),
		Messages: []chatMessage{{Role: "user", Content: ocrPrompt(model.Name()), Images: [][]byte{data}}},
		Stream:   types.Ptr(false),
		Options:  map[string]any{"temperature": 0},
	}}
	if err := request.apply(opts...); err != nil {
		return nil, llm.Usage{}, err
	}

	// Send the request, and receive the text
	req, err := client.NewJSONRequest(request.chatRequest)
	if err != nil {
		return nil, llm.Usage{}, err
	}
	var response chatResponse
	if err := c.DoWithContext(ctx, req, &response, client.OptPath("chat"), client.OptNoTimeout()); err != nil {
		return nil, llm.Usage{}, err
	}

	// Return the text, as the one page
	message := &schema.Message{MessageMeta: schema.MessageMeta{
		Sender:  "assistant",
		Content: []schema.ContentBlock{{Text: types.Ptr(strings.TrimSpace(response.Message.Content))}},
	}}
	return message, llm.Usage{
		InputTokens:  uint(max(response.PromptEvalCount, 0)),
		OutputTokens: uint(max(response.EvalCount, 0)),
		Pages:        1,
	}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// recognizeRequest is a request to recognize the text in an image, which is
// a request to chat, with the options to recognize text
type recognizeRequest struct {
	chatRequest
}

// apply sets the options on the request, or returns an error if any option
// isn't valid or isn't supported. An image is one page, zero, so the pages
// must have it, and the tables are Markdown, as the text is. The context
// window is num_ctx.
func (r *recognizeRequest) apply(opts ...llm.Opt) error {
	o, err := llm.ApplyOpts(opts...)
	if err != nil {
		return err
	}
	for _, key := range o.Keys() {
		var err error
		switch key {
		case llm.PagesKey:
			var pages []uint
			if pages, err = llm.GetPages(o); err == nil && !slices.Contains(pages, 0) {
				err = llm.ErrBadParameter.Withf("option %q: an image has one page, 0", key)
			}
		case llm.TableFormatKey:
			var format string
			if format, err = llm.GetOpt[string](o, key); err == nil && format != "markdown" {
				err = llm.ErrBadParameter.Withf("option %q: ollama's tables are markdown, not %q", key, format)
			}
		case llm.ContextWindowKey:
			var size uint
			if size, err = llm.GetOpt[uint](o, key); err == nil {
				r.Options["num_ctx"] = size
			}
		case llm.KeepAliveKey:
			var duration time.Duration
			if duration, err = llm.GetOpt[time.Duration](o, key); err == nil {
				r.KeepAlive = duration.String()
			}
		default:
			err = llm.ErrBadParameter.Withf("option %q isn't supported by ollama for recognition", key)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// ocrPrompt returns the prompt of the model, by its name, which is its own
// when it's a model which recognizes text, or otherwise the prompt of a
// model which sees images
func ocrPrompt(name string) string {
	if model := findOCRModel(name); model != nil {
		return model.prompt
	}
	return recognizePrompt
}

// findOCRModel returns the model which recognizes text, by its name, or nil
// when it isn't one
func findOCRModel(name string) *ocrModel {
	for i := range ocrModels {
		if ocrModels[i].re.MatchString(name) {
			return &ocrModels[i]
		}
	}
	return nil
}
