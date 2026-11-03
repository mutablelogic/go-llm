package ollama

import (
	"context"
	"encoding/json"
	"regexp"
	"slices"
	"strings"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// INTERFACES

var _ llm.Recognizer = (*Client)(nil)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// ocrModel is a model which recognizes text, by a regular expression of its
// name, and its prompts, as each is trained on its own, and is sensitive to
// them: for the text, and for the text with its regions, which a model
// without one can't recognize
type ocrModel struct {
	re        *regexp.Regexp
	prompt    string
	grounding string
}

// recognizeRequest is a request to recognize the text in an image, which is
// a request to generate a reply to the prompt, with the image, without a
// template's chat messages, as an OCR model is trained on its prompts alone
// Reference: https://github.com/ollama/ollama/blob/main/docs/api.md
type recognizeRequest struct {
	Model     string         `json:"model"`
	Prompt    string         `json:"prompt"`
	Images    [][]byte       `json:"images"`
	Stream    bool           `json:"stream"`
	KeepAlive string         `json:"keep_alive,omitempty"`
	Options   map[string]any `json:"options,omitempty"`
	regions   bool           // Whether the text has its regions
}

// recognizeResponse is the reply, which is the text, and the tokens used
type recognizeResponse struct {
	Response        string `json:"response"`
	PromptEvalCount int    `json:"prompt_eval_count"`
	EvalCount       int    `json:"eval_count"`
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// ocrModels are the models which recognize text
var ocrModels = []ocrModel{
	{regexp.MustCompile(`(^|/)deepseek-ocr\b`), "<image>\nConvert the document to markdown.", "<image>\n<|grounding|>Convert the document to markdown."},
	{regexp.MustCompile(`(^|/)glm-ocr\b`), "Text Recognition:", ""},
}

// recognizePrompt is the prompt of other models which see images
const recognizePrompt = "Convert the document to markdown."

// reRegion matches the start of a region in the text of a model which
// recognizes regions, which is its type, and its boxes, such as
// "text[[23, 20, 319, 72]]", which are on a scale of 0 to 999, as
// deepseek-ocr's are, once Ollama has removed the tokens around them
var reRegion = regexp.MustCompile(`(?m)^(\w+)(\[\[[\d\s,\[\]]+\]\])[ \t]*\n?`)

// regionScale is the scale of the boxes of regions
const regionScale = 999

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Recognize returns the text in the image, as a document with one page, in
// Markdown, and its regions, when they're asked for, and the tokens used,
// and the page. The model must be an Ollama model which sees images, such
// as deepseek-ocr, which is asked with its own prompt, and recognizes
// regions. Ollama's models read images, not documents, so a PDF isn't
// recognized. An image should be large enough for its text to be read, as
// a model doesn't read small text. The request isn't timed out, as a model
// can take a while to load, but is cancelled with the context.
func (c *Client) Recognize(ctx context.Context, model llm.Model, resource llm.Resource, opts ...llm.Opt) (llm.Document, llm.Usage, error) {
	if model == nil || model.Provider() == nil || model.Provider().Name() != llm.Ollama {
		return llm.Document{}, llm.Usage{}, llm.ErrBadParameter.With("not an ollama model")
	} else if resource == nil {
		return llm.Document{}, llm.Usage{}, llm.ErrBadParameter.With("missing resource")
	} else if !isImage(resource.Type()) {
		return llm.Document{}, llm.Usage{}, llm.ErrBadParameter.Withf("ollama recognizes text in images, not %q", resource.Type())
	}

	// The image, and the options
	data, err := llm.ReadAll(ctx, resource)
	if err != nil {
		return llm.Document{}, llm.Usage{}, err
	} else if len(data) == 0 {
		return llm.Document{}, llm.Usage{}, llm.ErrBadParameter.With("image is empty")
	}
	request := recognizeRequest{
		Model:   model.Name(),
		Images:  [][]byte{data},
		Options: map[string]any{"temperature": 0},
	}
	if err := request.apply(opts...); err != nil {
		return llm.Document{}, llm.Usage{}, err
	}

	// The model's prompt, for the text, or for its regions, which the model
	// must recognize
	ocr := findOCRModel(model.Name())
	switch {
	case !request.regions && ocr != nil:
		request.Prompt = ocr.prompt
	case !request.regions:
		request.Prompt = recognizePrompt
	case ocr != nil && ocr.grounding != "":
		request.Prompt = ocr.grounding
	default:
		return llm.Document{}, llm.Usage{}, llm.ErrBadParameter.Withf("option %q: model %q doesn't recognize regions", llm.RegionsKey, model.Name())
	}

	// Send the request, and receive the text
	req, err := client.NewJSONRequest(request)
	if err != nil {
		return llm.Document{}, llm.Usage{}, err
	}
	var response recognizeResponse
	if err := c.DoWithContext(ctx, req, &response, client.OptPath("generate"), client.OptNoTimeout()); err != nil {
		return llm.Document{}, llm.Usage{}, err
	}

	// Return the text, as the one page, with its regions
	page := llm.DocumentPage{Text: strings.TrimSpace(response.Response)}
	if request.regions {
		page = parseRegions(response.Response)
	}
	return llm.Document{Pages: []llm.DocumentPage{page}}, llm.Usage{
		InputTokens:  uint(max(response.PromptEvalCount, 0)),
		OutputTokens: uint(max(response.EvalCount, 0)),
		Pages:        1,
	}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

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
		case llm.RegionsKey:
			r.regions, err = llm.GetOpt[bool](o, key)
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

// parseRegions returns the page of the text of a model which recognizes
// regions, which is each region's type and boxes, and then its text, until
// the next region. A region's box is the box around its boxes, as a
// fraction of the page. The page's text is the text of its regions, and any
// text before them, separated by blank lines. A region whose boxes aren't
// numbers has an empty box.
func parseRegions(text string) llm.DocumentPage {
	var page llm.DocumentPage
	var texts []string
	matches := reRegion.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 || matches[0][0] > 0 {
		end := len(text)
		if len(matches) > 0 {
			end = matches[0][0]
		}
		if before := strings.TrimSpace(text[:end]); before != "" {
			texts = append(texts, before)
		}
	}
	for i, match := range matches {
		end := len(text)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		region := llm.DocumentRegion{
			Type: text[match[2]:match[3]],
			Text: strings.TrimSpace(text[match[1]:end]),
		}
		var boxes [][]float64
		if json.Unmarshal([]byte(text[match[4]:match[5]]), &boxes) == nil {
			region.Box = boundingBox(boxes)
		}
		page.Regions = append(page.Regions, region)
		if region.Text != "" {
			texts = append(texts, region.Text)
		}
	}
	page.Text = strings.Join(texts, "\n\n")
	return page
}

// boundingBox returns the box around the boxes, which are their left, top,
// right and bottom, on the scale of the regions, as fractions of the page,
// from 0 to 1. A box which isn't four numbers is skipped.
func boundingBox(boxes [][]float64) [4]float64 {
	var result [4]float64
	first := true
	for _, box := range boxes {
		if len(box) != 4 {
			continue
		}
		for i := range box {
			box[i] = min(max(box[i]/regionScale, 0), 1)
		}
		if first {
			result, first = [4]float64(box), false
			continue
		}
		result[0], result[1] = min(result[0], box[0]), min(result[1], box[1])
		result[2], result[3] = max(result[2], box[2]), max(result[3], box[3])
	}
	return result
}
