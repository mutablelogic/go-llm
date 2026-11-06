package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	// Packages
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	httpclient "github.com/mutablelogic/go-llm/registry/httpclient"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	server "github.com/mutablelogic/go-server"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// InferenceCommands are the commands which ask models to do something with
// messages
type InferenceCommands struct {
	Ask        AskCmd        `cmd:"" name:"ask" help:"Ask a model a question, and print its reply, and the tools it calls." group:"LLM INFERENCE"`
	Chat       ChatCmd       `cmd:"" name:"chat" help:"Chat with a session's model, or in the terminal without a question." group:"LLM INFERENCE"`
	Embed      EmbedCmd      `cmd:"" name:"embed" help:"Get a vector for text, an image or audio." group:"LLM INFERENCE"`
	OCR        RecognizeCmd  `cmd:"" name:"ocr" help:"Ask a model for the text in an image or document, which is printed as Markdown." group:"LLM INFERENCE"`
	Transcribe TranscribeCmd `cmd:"" name:"transcribe" help:"Ask a model for the speech in audio, as text, or its segments." group:"LLM INFERENCE"`
	Translate  TranslateCmd  `cmd:"" name:"translate" help:"Translate the speech in audio into English text." group:"LLM INFERENCE"`
}

type EmbedCmd struct {
	Text   []string                   `arg:"" name:"text" help:"The text to embed, which can be more than one argument." optional:""`
	Model  string                     `name:"model" help:"Model name, or provider and model name separated by a colon, such as ollama:embeddinggemma-2:latest. Defaults to the model last used for embeddings." optional:""`
	Attach string                     `name:"attach" short:"a" aliases:"file" help:"An image or audio file to embed with the text, or a resource, by its URI, resource://<hash>, for a model which embeds them." optional:""`
	Opt    map[string]json.RawMessage `name:"opt" help:"Options, by key, such as dimensions=256, which can be given more than once." optional:""`
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// defaultEmbedModelKey is the key of the default model for embeddings, which
// is the model last used for them, as a model which replies to messages
// usually can't embed them
const defaultEmbedModelKey = "llm.embed_model"

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Run prints the vector of the text, and the file, as JSON, or the response,
// with debugging, and keeps the model, by its canonical name, as the
// default for embeddings
func (cmd *EmbedCmd) Run(ctx server.Cmd) error {
	// The model, or the model last used for embeddings
	model := strings.TrimSpace(cmd.Model)
	if model == "" {
		model = strings.TrimSpace(ctx.GetString(defaultEmbedModelKey))
	}
	if model == "" {
		return fmt.Errorf("missing model: set one with --model, and it's used until another is set")
	}

	// The message, which is the text, and the file
	var message schema.EmbedMessage
	if text := strings.TrimSpace(strings.Join(cmd.Text, " ")); text != "" {
		message.Content = append(message.Content, schema.EmbedContent{Text: types.Ptr(text)})
	}
	if isResourceURI(cmd.Attach) {
		message.Content = append(message.Content, schema.EmbedContent{Attachment: &schema.EmbedAttachment{URL: strings.TrimSpace(cmd.Attach)}})
	} else if cmd.Attach != "" {
		data, err := os.ReadFile(cmd.Attach)
		if err != nil {
			return err
		}
		message.Content = append(message.Content, schema.EmbedContent{Attachment: &schema.EmbedAttachment{
			Type: providerschema.ResourceType(data, filepath.Base(cmd.Attach)),
			Data: data,
		}})
	}
	if len(message.Content) == 0 {
		return fmt.Errorf("missing text or file")
	}

	debug := ctx.IsDebug()
	return withClient(ctx, "llm.EmbedCmd", func(reqctx context.Context, client *httpclient.Client) error {
		response, err := client.Embed(reqctx, schema.EmbedRequest{
			Model:    model,
			Messages: []schema.EmbedMessage{message},
			Opt:      quoteOpts(cmd.Opt),
		})
		if err != nil {
			return err
		}

		// The model is the default once it's embedded the message, by its
		// canonical name, as its name could match another provider's model
		// later
		if err := ctx.Set(defaultEmbedModelKey, response.Model); err != nil {
			return err
		}

		// With debugging, the response, otherwise the vector
		if debug {
			fmt.Println(response)
			return nil
		}
		for _, vector := range response.Vectors {
			data, err := json.Marshal(vector)
			if err != nil {
				return err
			}
			fmt.Println(string(data))
		}
		return nil
	})
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// quoteOpts returns the options, with each value which isn't JSON as a JSON
// string, so --opt effort=high is the same as --opt 'effort="high"'
func quoteOpts(opts map[string]json.RawMessage) map[string]json.RawMessage {
	for key, value := range opts {
		if !json.Valid(value) {
			if quoted, err := json.Marshal(string(value)); err == nil {
				opts[key] = quoted
			}
		}
	}
	return opts
}
