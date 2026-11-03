package openai

import (
	"context"
	"encoding/base64"
	"mime"
	"strings"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// INTERFACE CHECK

var _ llm.Recognizer = (*Client)(nil)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Recognize returns the text in the resource, which is an image, or a
// document such as a PDF, as a document with the text of each of its pages,
// in Markdown, with its tables in the table format, in order, and their
// regions, which are Mistral's blocks, when they're asked for, and the pages
// recognized, with Mistral's OCR API, which other servers haven't. The model must be one of this client's which
// recognizes text, such as mistral-ocr-latest. The images in a document
// aren't returned, but the text has links to them. The request isn't timed
// out, as a document can take a while, but is cancelled with the context.
func (c *Client) Recognize(ctx context.Context, model llm.Model, resource llm.Resource, opts ...llm.Opt) (llm.Document, llm.Usage, error) {
	if m, ok := model.(*Model); !ok || m.provider != c {
		return llm.Document{}, llm.Usage{}, llm.ErrBadParameter.With("not a model of this provider")
	} else if resource == nil {
		return llm.Document{}, llm.Usage{}, llm.ErrBadParameter.With("missing resource")
	} else if c.server() != serverMistral {
		return llm.Document{}, llm.Usage{}, llm.ErrNotImplemented.With("only mistral recognizes text")
	}

	// The image, or document, as a data URL
	contentType, _, err := mime.ParseMediaType(resource.Type())
	if err != nil {
		return llm.Document{}, llm.Usage{}, llm.ErrBadParameter.Withf("invalid type %q", resource.Type())
	}
	data, err := llm.ReadAll(ctx, resource)
	if err != nil {
		return llm.Document{}, llm.Usage{}, err
	} else if len(data) == 0 {
		return llm.Document{}, llm.Usage{}, llm.ErrBadParameter.With("resource is empty")
	}
	url := "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data)
	request := ocrRequest{Model: model.Name(), IncludeImages: types.Ptr(false), IncludeBlocks: types.Ptr(false)}
	if strings.HasPrefix(contentType, "image/") {
		request.Document = ocrDocument{Type: "image_url", ImageURL: url}
	} else {
		request.Document = ocrDocument{Type: "document_url", DocumentURL: url}
	}
	if err := request.apply(opts...); err != nil {
		return llm.Document{}, llm.Usage{}, err
	}

	// Send the request, and receive the text of each page
	req, err := client.NewJSONRequest(request)
	if err != nil {
		return llm.Document{}, llm.Usage{}, err
	}
	var response ocrResponse
	if err := c.DoWithContext(ctx, req, &response, client.OptPath("ocr"), client.OptNoTimeout()); err != nil {
		return llm.Document{}, llm.Usage{}, err
	}

	// Return the text of each page, in order, and its regions, with their
	// boxes as fractions of the page's size, which a page without a size
	// hasn't
	document := llm.Document{Pages: make([]llm.DocumentPage, 0, len(response.Pages))}
	for _, page := range response.Pages {
		// The tables, in place of the links to them
		var tables []string
		for _, table := range page.Tables {
			tables = append(tables, "["+table.ID+"]("+table.ID+")", table.Content)
		}
		replacer := strings.NewReplacer(tables...)

		result := llm.DocumentPage{Text: strings.TrimSpace(replacer.Replace(page.Markdown))}
		width, height := page.Dimensions.Width, page.Dimensions.Height
		if types.Value(request.IncludeBlocks) && width > 0 && height > 0 {
			for _, block := range page.Blocks {
				result.Regions = append(result.Regions, llm.DocumentRegion{
					Type: block.Type,
					Text: strings.TrimSpace(replacer.Replace(block.Content)),
					Box:  [4]float64{block.Left / width, block.Top / height, block.Right / width, block.Bottom / height},
				})
			}
		}
		document.Pages = append(document.Pages, result)
	}
	if len(document.Pages) == 0 {
		return llm.Document{}, llm.Usage{}, llm.ErrInternalServerError.With("no pages were recognized")
	}
	return document, llm.Usage{Pages: response.Usage.PagesProcessed}, nil
}
