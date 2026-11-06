package schema

import (
	"encoding/json"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// EmbedRequest asks a model for a vector for each of the messages, such as
// for semantic search, with the options. It's one model, as vectors from
// different models can't be compared.
type EmbedRequest struct {
	Model    string                     `json:"model" help:"Model for the embeddings, by name, or by provider and name" example:"ollama:embeddinggemma-2:latest"`
	Messages []EmbedMessage             `json:"messages" help:"Messages to embed, each by its text, and its images and audio for a model which embeds them"`
	Opt      map[string]json.RawMessage `json:"opt,omitzero" help:"Options for embeddings, by key, such as dimensions=256" example:"{\"dimensions\":256}" optional:""`
}

// EmbedMessage is a message to embed, by its text, and its images and audio,
// inline, as a message has them, without the role, thinking or tools, which
// aren't embedded
type EmbedMessage struct {
	Content []EmbedContent `json:"content" help:"Text and attachments of the message to embed" example:"[{\"text\":\"A cat asleep in the sun\"}]"`
}

// EmbedContent is a piece of a message to embed: its text, or an attachment
type EmbedContent struct {
	Text       *string          `json:"text,omitempty" help:"Text to embed" example:"A cat asleep in the sun" optional:""`
	Attachment *EmbedAttachment `json:"attachment,omitempty" help:"Image or audio to embed, for a model which embeds them" optional:""`
}

// EmbedAttachment is an image or audio to embed, by its type and data, or
// the URI of a resource, on its own, whose content is read
type EmbedAttachment struct {
	Type string `json:"type,omitempty" help:"MIME type of the attachment, which is needed with its data" example:"image/png" optional:""`
	Data []byte `json:"data,omitempty" help:"Data of the attachment, encoded as base64, or its URL" example:"iVBORw0KGgo=" optional:""`
	URL  string `json:"url,omitempty" help:"URI of a resource, which is resource://<hash>, on its own, whose content is read, rather than the data, whose type is the resource's, unless it's given" example:"resource://9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08" optional:""`
}

// EmbedForm is the form for embedding one message, such as from a web
// browser, which is a file, uploaded as text, an image or audio, which is
// read as an EmbedRequest with one message, as RecognizeForm is. The
// options are a JSON object, as a form's values are strings.
type EmbedForm struct {
	Model string     `json:"model" help:"Model for the embedding, by name, or by provider and name" example:"ollama:embeddinggemma-2:latest"`
	Opt   string     `json:"opt,omitempty" help:"Options for the embedding, as a JSON object, such as {\"dimensions\":256}" example:"{\"dimensions\":256}" optional:""`
	File  types.File `json:"file,omitzero" help:"Text, image or audio to embed, uploaded as a file, which is embedded as text when its type is text, such as text/plain, or otherwise as an attachment, for a model which embeds them"`
}

// EmbedResponse is a vector for each of the messages, in the same order,
// with the model which embedded them, and the tokens used
type EmbedResponse struct {
	Model   string       `json:"model" help:"Canonical name of the model which embedded the messages" example:"ollama:embeddinggemma-2:latest"`
	Vectors []llm.Vector `json:"vectors" help:"A vector for each of the messages, in the same order"`
	Usage   llm.Usage    `json:"usage" help:"Tokens used, which are input tokens"`
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// EmbedBodyLimit is the largest request body, in bytes, when messages are
// embedded, which allows for images and audio in the messages
const EmbedBodyLimit = 32 << 20

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Message returns the message to embed, as a message from the user, with
// its text, and its attachments, or an error when it has neither, or an
// attachment has no type, or no data
func (m EmbedMessage) Message() (llm.Message, error) {
	message := &providerschema.Message{MessageMeta: providerschema.MessageMeta{Sender: "user"}}
	for _, content := range m.Content {
		if text := strings.TrimSpace(types.Value(content.Text)); text != "" {
			message.Content = append(message.Content, providerschema.ContentBlock{Text: types.Ptr(text)})
		}
		if a := content.Attachment; a != nil {
			resource, err := attachment(a.Type, a.Data, a.URL)
			if err != nil {
				return nil, llm.ErrBadParameter.Withf("attachment: %v", err)
			}
			message.Content = append(message.Content, providerschema.ContentBlock{Resource: resource})
		}
	}
	if len(message.Content) == 0 {
		return nil, llm.ErrBadParameter.With("message has no text or attachment")
	}
	return message, nil
}

// Opts returns the options, for the provider to decode
func (r EmbedRequest) Opts() []llm.Opt {
	opts := make([]llm.Opt, 0, len(r.Opt))
	for key, value := range r.Opt {
		opts = append(opts, llm.WithOpt(key, value))
	}
	return opts
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (r EmbedRequest) String() string {
	return types.Stringify(r)
}

func (r EmbedResponse) String() string {
	return types.Stringify(r)
}
