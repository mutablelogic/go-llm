package lmstudio

import (
	"context"
	"mime"
	"regexp"
	"slices"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// INTERFACES

var _ llm.Recognizer = (*Client)(nil)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// ocrModel is a model which recognizes text, by a regular expression of its
// name, with the prompt it's trained on, as each is sensitive to its own,
// the format of its tables, and whether its text starts with front matter,
// which is taken out
type ocrModel struct {
	re          *regexp.Regexp
	prompt      string
	tables      string
	frontMatter bool
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// ocrModels are the models which recognize text. olmOCR's prompt is the one
// it's trained on, from olmocr's build_no_anchoring_v4_yaml_prompt, whose
// text starts with front matter, of its language, and the page's rotation,
// and whose tables are HTML.
var ocrModels = []ocrModel{
	{regexp.MustCompile(`(?i)(^|/)olmocr\b`), "Attached is one page of a document that you must process. " +
		"Just return the plain text representation of this document as if you were reading it naturally. Convert equations to LateX and tables to HTML.\n" +
		"If there are any figures or charts, label them with the following markdown syntax ![Alt text describing the contents of the figure](page_startx_starty_width_height.png)\n" +
		"Return your output as markdown, with a front matter section on top specifying values for the primary_language, is_rotation_valid, rotation_correction, is_table, and is_diagram parameters.", "html", true},
	{regexp.MustCompile(`(?i)(^|/)deepseek-ocr\b`), "Convert the document to markdown.", "markdown", false},
	{regexp.MustCompile(`(?i)(^|/)glm-ocr\b`), "Text Recognition:", "markdown", false},
}

// recognizePrompt is the prompt of other models which see images
const recognizePrompt = "Convert the document to markdown."

// recognizeTokens is the most tokens of a page's text, which is olmOCR's
const recognizeTokens = 8000

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Recognize returns the text in the image, which is one page, in Markdown,
// and the tokens used, and the page. The model must be an LM Studio model
// which sees images, such as an OCR model, such as olmOCR, which is asked
// with the prompt it's trained on, or another, which is asked to convert the
// image to Markdown. It's asked with LM Studio's OpenAI-compatible Chat
// Completions API, as models read images, not documents, so a PDF isn't
// recognized. An image should be large enough for its text to be read, such
// as 1288 pixels on its longest side, as olmOCR's pages are. Regions aren't
// recognized. The request isn't timed out, as a model can take a while to
// load, but is cancelled with the context.
func (c *Client) Recognize(ctx context.Context, model llm.Model, resource llm.Resource, opts ...llm.Opt) (llm.Document, llm.Usage, error) {
	compatible, err := c.compatible(model)
	if err != nil {
		return llm.Document{}, llm.Usage{}, err
	} else if resource == nil {
		return llm.Document{}, llm.Usage{}, llm.ErrBadParameter.With("missing resource")
	} else if !isImage(resource.Type()) {
		return llm.Document{}, llm.Usage{}, llm.ErrBadParameter.Withf("LM Studio recognizes text in images, not %q", resource.Type())
	}

	// The model's prompt, and the options
	ocr := findOCRModel(model.Name())
	prompt, tables := recognizePrompt, "markdown"
	if ocr != nil {
		prompt, tables = ocr.prompt, ocr.tables
	}
	if err := applyRecognize(tables, opts...); err != nil {
		return llm.Document{}, llm.Usage{}, err
	}

	// The image
	data, err := llm.ReadAll(ctx, resource)
	if err != nil {
		return llm.Document{}, llm.Usage{}, err
	} else if len(data) == 0 {
		return llm.Document{}, llm.Usage{}, llm.ErrBadParameter.With("image is empty")
	}

	// Ask with the prompt, and then the image, as olmOCR is
	message := &providerschema.Message{MessageMeta: providerschema.MessageMeta{Sender: "user", Content: []providerschema.ContentBlock{
		{Text: types.Ptr(prompt)},
		{Resource: &providerschema.Resource{ResourceMeta: providerschema.ResourceMeta{ContentType: resource.Type(), Data: data}}},
	}}}
	reply, usage, err := c.openai.WithoutSession(ctx, compatible, []llm.Message{message}, nil, nil, llm.WithTemperature(0), llm.WithMaxTokens(recognizeTokens))
	if err != nil {
		return llm.Document{}, llm.Usage{}, err
	}
	text := reply.Text()
	if ocr != nil && ocr.frontMatter {
		text = withoutFrontMatter(text)
	}
	usage.Pages = 1
	return llm.Document{Pages: []llm.DocumentPage{{Text: strings.TrimSpace(text)}}}, usage, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// applyRecognize returns an error if any option isn't valid, or isn't
// supported. An image is one page, zero, so the pages must have it, and the
// tables are the model's format.
func applyRecognize(tables string, opts ...llm.Opt) error {
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
			if format, err = llm.GetOpt[string](o, key); err == nil && format != tables {
				err = llm.ErrBadParameter.Withf("option %q: the model's tables are %s, not %q", key, tables, format)
			}
		case llm.RegionsKey:
			var regions bool
			if regions, err = llm.GetOpt[bool](o, key); err == nil && regions {
				err = llm.ErrBadParameter.Withf("option %q: LM Studio's models don't recognize regions", key)
			}
		default:
			err = llm.ErrBadParameter.Withf("option %q isn't supported by LM Studio for recognition", key)
		}
		if err != nil {
			return err
		}
	}
	return nil
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

// withoutFrontMatter returns the text without the front matter it starts
// with, between lines of "---", or the text, when it has none
func withoutFrontMatter(text string) string {
	rest, ok := strings.CutPrefix(strings.TrimLeft(text, " \t\r\n"), "---")
	if !ok {
		return text
	}
	rest = strings.TrimLeft(rest, " \t\r")
	if !strings.HasPrefix(rest, "\n") {
		return text
	}
	if i := strings.Index(rest, "\n---"); i >= 0 {
		after := rest[i+len("\n---"):]
		if j := strings.IndexByte(after, '\n'); j >= 0 {
			return after[j+1:]
		} else if strings.TrimSpace(after) == "" {
			return ""
		}
	}
	return text
}

// isImage returns whether the type is an image's
func isImage(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	return err == nil && strings.HasPrefix(mediaType, "image/")
}
