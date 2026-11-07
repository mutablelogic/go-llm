package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	// Packages
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
	httpclient "github.com/mutablelogic/go-llm/registry/httpclient"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	resourceschema "github.com/mutablelogic/go-llm/resource/schema"
	server "github.com/mutablelogic/go-server"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// IllustrateCmd asks a model to make images, from a prompt, or by changing
// images, and writes them to files
type IllustrateCmd struct {
	Prompt      []string                   `arg:"" name:"prompt" optional:"" help:"What to make, or how to change the images, which can be more than one argument, or is read from standard input."`
	Model       string                     `name:"model" help:"Model name, or provider and model name separated by a colon, such as gemini:gemini-3.1-flash-lite-image. Defaults to the model last used for making images." optional:""`
	Attach      []string                   `name:"attach" short:"a" sep:"none" help:"An image to change, or combine, as a file, or a resource, by its URI, resource:///<hash>, which can be given more than once." optional:""`
	AspectRatio string                     `name:"aspect-ratio" help:"Aspect ratio of the images, such as 16:9." optional:""`
	Size        string                     `name:"size" help:"Size of the images, such as 1K, 2K or 4K." optional:""`
	Opt         map[string]json.RawMessage `name:"opt" help:"Options, by key, which can be given more than once." optional:""`
	Output      string                     `name:"output" short:"o" type:"path" help:"File to write the image to, which is numbered, such as banana-2.png, when there's more than one, or its name, such as image.png, when it's omitted." optional:""`
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// defaultIllustrateModelKey is the key of the default model for making
// images, which is the model last used for it
const defaultIllustrateModelKey = "llm.illustrate_model"

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Run asks the model for images, from the prompt, and the images, which are
// files, sent as their data, or resources, by their URIs, and writes each
// to a file, saying what was written
func (cmd *IllustrateCmd) Run(ctx server.Cmd) error {
	model := strings.TrimSpace(cmd.Model)
	if model == "" {
		model = strings.TrimSpace(ctx.GetString(defaultIllustrateModelKey))
	}
	if model == "" {
		return fmt.Errorf("missing model: set one with --model, and it's used until another is set")
	}

	// The prompt, from the arguments, or standard input
	prompt := strings.TrimSpace(strings.Join(cmd.Prompt, " "))
	if prompt == "" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		prompt = strings.TrimSpace(string(data))
	}
	if prompt == "" {
		return fmt.Errorf("missing prompt")
	}

	// The images, and the options
	req := schema.IllustrateRequest{Model: model, Prompt: prompt, Opt: quoteOpts(cmd.Opt)}
	for _, value := range cmd.Attach {
		image, err := illustrateImage(value)
		if err != nil {
			return err
		}
		req.Images = append(req.Images, image)
	}
	for key, value := range map[string]string{"aspect_ratio": cmd.AspectRatio, "image_size": cmd.Size} {
		if value = strings.TrimSpace(value); value != "" {
			if req.Opt == nil {
				req.Opt = map[string]json.RawMessage{}
			}
			req.Opt[key], _ = json.Marshal(value)
		}
	}
	resources, err := newResourceClient(ctx)
	if err != nil {
		return err
	}

	return withClient(ctx, "llm.IllustrateCmd", func(reqctx context.Context, client *httpclient.Client) error {
		response, err := client.Illustrate(reqctx, req)
		if err != nil {
			return err
		}
		// Each image is written to its file, from its resource, or its data
		for i, image := range response.Images {
			path := outputPath(cmd.Output, image.Name, image.Type, i, len(response.Images))
			if err := writeImage(reqctx, path, image, func(w io.Writer) error {
				hash, err := resourceschema.ParseURI(image.URI)
				if err != nil {
					return err
				}
				_, err = resources.ReadResource(reqctx, hash, resourceschema.ResourceRequest{}, w)
				return err
			}); err != nil {
				return err
			}
			fmt.Printf("%s: %s, by %s\n", path, image.Type, response.Model)
		}

		// The model is used next time, once the images are written
		if response.Model != "" {
			return ctx.Set(defaultIllustrateModelKey, response.Model)
		}
		return nil
	})
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// illustrateImage returns an image to change: a resource, by its URI, or a
// file, as its data, with its type
func illustrateImage(value string) (schema.IllustrateImage, error) {
	if isResourceURI(value) {
		return schema.IllustrateImage{URL: value}, nil
	}
	data, err := os.ReadFile(value)
	if err != nil {
		return schema.IllustrateImage{}, err
	}
	contentType := providerschema.ResourceType(data, filepath.Base(value))
	if !strings.HasPrefix(contentType, "image/") {
		return schema.IllustrateImage{}, fmt.Errorf("%s: not an image", value)
	}
	return schema.IllustrateImage{Type: contentType, Data: data}, nil
}

// outputPath returns the path an image is written to: the output, numbered
// from the second, when there's more than one, or its name, or image, with
// the extension of its type
func outputPath(output, name, contentType string, i, n int) string {
	if output == "" {
		output = name
		if output == "" {
			output = "image" + imageExt(contentType)
		}
	}
	if n > 1 && i > 0 {
		ext := filepath.Ext(output)
		output = fmt.Sprintf("%s-%d%s", strings.TrimSuffix(output, ext), i+1, ext)
	}
	return output
}

// imageExt returns the extension of an image of the type
func imageExt(contentType string) string {
	switch contentType {
	case "image/jpeg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	}
	return ".png"
}

// writeImage writes the image to the file at the path, from its data, or,
// when it's a resource, as read reads it, removing the file when it isn't
// written whole
func writeImage(_ context.Context, path string, image schema.Illustration, read func(io.Writer) error) (err error) {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := f.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			_ = os.Remove(path)
		}
	}()
	if len(image.Data) > 0 {
		_, err = f.Write(image.Data)
		return err
	}
	return read(f)
}
