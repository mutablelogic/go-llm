package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	httpclient "github.com/mutablelogic/go-llm/registry/httpclient"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	server "github.com/mutablelogic/go-server"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type RecognizeCmd struct {
	File  string                     `arg:"" name:"file" help:"The image or document, such as a PDF, whose text is recognized, or a resource, by its URI, resource://<hash>."`
	Model string                     `name:"model" help:"Model name, or provider and model name separated by a colon, such as mistral:mistral-ocr-latest. Defaults to the model last used for recognizing text." optional:""`
	Opt   map[string]json.RawMessage `name:"opt" help:"Options, by key, such as pages=0-2, table_format=html, or regions=true, which prints JSON, which can be given more than once." optional:""`
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// defaultRecognizeModelKey is the key of the default model for recognizing
// text, which is the model last used for it, as a model which replies to
// messages usually can't
const defaultRecognizeModelKey = "llm.recognize_model"

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Run prints the text of the file, in Markdown, with a rule between pages,
// or the response, as JSON, with debugging, or when the pages have their
// regions, and keeps the model, by its canonical
// name, as the default for recognizing text
func (cmd *RecognizeCmd) Run(ctx server.Cmd) error {
	// The model, or the model last used for recognizing text
	model := strings.TrimSpace(cmd.Model)
	if model == "" {
		model = strings.TrimSpace(ctx.GetString(defaultRecognizeModelKey))
	}
	if model == "" {
		return fmt.Errorf("missing model: set one with --model, and it's used until another is set")
	}

	// The request, for the image or document, or the resource, and whether
	// its regions are asked for, when the response is printed as JSON, even
	// when a page has none, such as a blank page
	req := schema.RecognizeRequest{Model: model, Opt: quoteOpts(cmd.Opt)}
	if isResourceURI(cmd.File) {
		req.URL = strings.TrimSpace(cmd.File)
	} else {
		data, err := os.ReadFile(cmd.File)
		if err != nil {
			return err
		}
		req.Type, req.Data = providerschema.ResourceType(data, filepath.Base(cmd.File)), data
	}
	opts, err := llm.ApplyOpts(req.Opts()...)
	if err != nil {
		return err
	}
	regions, _ := llm.GetOpt[bool](opts, llm.RegionsKey)

	debug := ctx.IsDebug()
	return withClient(ctx, "llm.RecognizeCmd", func(reqctx context.Context, client *httpclient.Client) error {
		response, err := client.Recognize(reqctx, req)
		if err != nil {
			return err
		}

		// The model is the default once it's recognized the text, by its
		// canonical name, as its name could match another provider's model
		// later
		if err := ctx.Set(defaultRecognizeModelKey, response.Model); err != nil {
			return err
		}

		// With debugging, or regions, the response, otherwise the text of
		// each page, with a rule between them
		if debug || regions {
			fmt.Println(response)
			return nil
		}
		pages := make([]string, 0, len(response.Pages))
		for _, page := range response.Pages {
			pages = append(pages, page.Text)
		}
		fmt.Println(strings.Join(pages, "\n\n---\n\n"))
		return nil
	})
}
