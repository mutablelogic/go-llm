package gemini

import (
	"context"
	"slices"
	"strings"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// INTERFACE CHECK

var _ llm.Embedder = (*Client)(nil)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// taskTypes are the tasks an embedding is for, which the API takes, in
// upper case, such as RETRIEVAL_QUERY, and the option gives in any case
var taskTypes = []string{
	"SEMANTIC_SIMILARITY",
	"CLASSIFICATION",
	"CLUSTERING",
	"RETRIEVAL_DOCUMENT",
	"RETRIEVAL_QUERY",
	"CODE_RETRIEVAL_QUERY",
	"QUESTION_ANSWERING",
	"FACT_VERIFICATION",
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Embed returns a vector for each of the messages, which must be for a model
// of this client which embeds, in the same order, in one request to
// batchEmbedContents. A message is embedded by its text, and its
// attachments, such as images, audio, and PDFs, which a multimodal model,
// such as gemini-embedding-2, embeds, together. The options set what the
// vectors are for, as a task type, such as retrieval_query, the title of
// the document the messages are from, and the number of dimensions of the
// vectors. The API doesn't say the tokens it used, so the usage is none.
// The request isn't timed out, but is cancelled with the context.
func (c *Client) Embed(ctx context.Context, model llm.Model, messages []llm.Message, opts ...llm.Opt) ([]llm.Vector, llm.Usage, error) {
	if m, ok := model.(*Model); !ok || m.provider != c {
		return nil, llm.Usage{}, llm.ErrBadParameter.With("not a model of this provider")
	} else if len(messages) == 0 {
		return nil, llm.Usage{}, llm.ErrBadParameter.With("missing messages")
	}

	// The options, which are the same for each message
	template, err := embedOptions(opts...)
	if err != nil {
		return nil, llm.Usage{}, err
	}
	template.Model = modelPrefix + model.Name()

	// A request for each message, with its text, and its attachments
	request := batchEmbedRequest{Requests: make([]embedRequest, 0, len(messages))}
	for i, message := range messages {
		if message == nil {
			return nil, llm.Usage{}, llm.ErrBadParameter.Withf("message %d is missing", i)
		}
		var parts []part
		if text := message.Text(); text != "" {
			parts = append(parts, part{Text: text})
		}
		for _, resource := range message.Resources() {
			data, err := llm.ReadAll(ctx, resource)
			if err != nil {
				return nil, llm.Usage{}, err
			}
			parts = append(parts, part{InlineData: &blob{MimeType: resource.Type(), Data: data}})
		}
		if len(parts) == 0 {
			return nil, llm.Usage{}, llm.ErrBadParameter.Withf("message %d has no text, or attachments", i)
		}
		embed := template
		embed.Content = content{Parts: parts}
		request.Requests = append(request.Requests, embed)
	}

	// Send the request, and receive a vector for each message
	payload, err := client.NewJSONRequest(request)
	if err != nil {
		return nil, llm.Usage{}, err
	}
	var response batchEmbedResponse
	if err := c.DoWithContext(ctx, payload, &response, client.OptPath("models", model.Name()+":batchEmbedContents"), client.OptNoTimeout()); err != nil {
		return nil, llm.Usage{}, err
	} else if len(response.Embeddings) != len(messages) {
		return nil, llm.Usage{}, llm.ErrInternalServerError.Withf("%d vectors for %d messages", len(response.Embeddings), len(messages))
	}
	vectors := make([]llm.Vector, len(messages))
	for i, embedding := range response.Embeddings {
		if len(embedding.Values) == 0 {
			return nil, llm.Usage{}, llm.ErrInternalServerError.Withf("no vector for message %d", i)
		}
		vectors[i] = llm.Vector(embedding.Values)
	}
	return vectors, llm.Usage{}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// embedOptions returns a request to embed a message, with the options: what
// it's for, which is one of the task types, in any case, the title of the
// document it's from, and the number of dimensions of its vector. Returns
// ErrBadParameter for a task type which isn't one, and any other option.
func embedOptions(opts ...llm.Opt) (embedRequest, error) {
	var request embedRequest
	o, err := llm.ApplyOpts(opts...)
	if err != nil {
		return request, err
	}
	for _, key := range o.Keys() {
		var err error
		switch key {
		case llm.TaskTypeKey:
			var taskType string
			if taskType, err = llm.GetOpt[string](o, key); err == nil {
				request.TaskType = strings.ToUpper(strings.TrimSpace(taskType))
				if !slices.Contains(taskTypes, request.TaskType) {
					err = llm.ErrBadParameter.Withf("unknown task type %q, which isn't one of %s", taskType, strings.ToLower(strings.Join(taskTypes, ", ")))
				}
			}
		case llm.TitleKey:
			request.Title, err = llm.GetOpt[string](o, key)
		case llm.DimensionsKey:
			request.OutputDimensionality, err = llm.GetOpt[uint](o, key)
		default:
			err = llm.ErrBadParameter.Withf("option %q isn't supported by gemini for embeddings", key)
		}
		if err != nil {
			return embedRequest{}, err
		}
	}
	return request, nil
}
