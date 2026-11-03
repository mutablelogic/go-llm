package openai

import (
	"context"
	"encoding/base64"
	"mime"
	"strings"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/provider/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// INTERFACE CHECK

var _ llm.Recognizer = (*Client)(nil)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Recognize returns the text in the resource, which is an image, or a
// document such as a PDF, as a message with the text of each of its pages,
// in Markdown, in order, and the pages recognized, with Mistral's OCR API,
// which other servers haven't. The model must be one of this client's which
// recognizes text, such as mistral-ocr-latest. The images in a document
// aren't returned, but the text has links to them. The request isn't timed
// out, as a document can take a while, but is cancelled with the context.
func (c *Client) Recognize(ctx context.Context, model llm.Model, resource llm.Resource, opts ...llm.Opt) (llm.Message, llm.Usage, error) {
	if m, ok := model.(*Model); !ok || m.provider != c {
		return nil, llm.Usage{}, llm.ErrBadParameter.With("not a model of this provider")
	} else if resource == nil {
		return nil, llm.Usage{}, llm.ErrBadParameter.With("missing resource")
	} else if c.server() != serverMistral {
		return nil, llm.Usage{}, llm.ErrNotImplemented.With("only mistral recognizes text")
	}

	// The image, or document, as a data URL
	contentType, _, err := mime.ParseMediaType(resource.Type())
	if err != nil {
		return nil, llm.Usage{}, llm.ErrBadParameter.Withf("invalid type %q", resource.Type())
	}
	data, err := llm.ReadAll(ctx, resource)
	if err != nil {
		return nil, llm.Usage{}, err
	} else if len(data) == 0 {
		return nil, llm.Usage{}, llm.ErrBadParameter.With("resource is empty")
	}
	url := "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data)
	request := ocrRequest{Model: model.Name(), IncludeImages: types.Ptr(false)}
	if strings.HasPrefix(contentType, "image/") {
		request.Document = ocrDocument{Type: "image_url", ImageURL: url}
	} else {
		request.Document = ocrDocument{Type: "document_url", DocumentURL: url}
	}
	if err := request.apply(opts...); err != nil {
		return nil, llm.Usage{}, err
	}

	// Send the request, and receive the text of each page
	req, err := client.NewJSONRequest(request)
	if err != nil {
		return nil, llm.Usage{}, err
	}
	var response ocrResponse
	if err := c.DoWithContext(ctx, req, &response, client.OptPath("ocr"), client.OptNoTimeout()); err != nil {
		return nil, llm.Usage{}, err
	}

	// Return the text of each page, in order
	message := &schema.Message{MessageMeta: schema.MessageMeta{Sender: "assistant"}}
	for _, page := range response.Pages {
		message.Content = append(message.Content, schema.ContentBlock{Text: types.Ptr(strings.TrimSpace(page.Markdown))})
	}
	if len(message.Content) == 0 {
		return nil, llm.Usage{}, llm.ErrInternalServerError.With("no pages were recognized")
	}
	return message, llm.Usage{Pages: response.Usage.PagesProcessed}, nil
}
