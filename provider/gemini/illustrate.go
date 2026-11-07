package gemini

import (
	"context"
	"strings"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/provider/schema"
)

///////////////////////////////////////////////////////////////////////////////
// INTERFACES

var _ llm.Illustrator = (*Client)(nil)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Illustrate returns the images the model, such as Nano Banana, makes from
// the prompt, or by changing, or combining, the images, as the prompt says,
// with generateContent, which is asked for images, with the aspect ratio,
// and size, the options set, and the tokens used. The images it makes as it
// thinks, which are drafts, are left out. Returns ErrNotImplemented for a
// model without generateContent, such as Imagen, which predicts images.
// Returns an error, with what the model said, when it made no image, such
// as when it refused. The request
// isn't timed out, as an image can take a while, but is cancelled with the
// context.
func (c *Client) Illustrate(ctx context.Context, model llm.Model, prompt string, images []llm.Resource, opts ...llm.Opt) ([]llm.Resource, llm.Usage, error) {
	m, ok := model.(*Model)
	if !ok || m.provider != c {
		return nil, llm.Usage{}, llm.ErrBadParameter.With("not a model of this provider")
	} else if !m.supports("generateContent") {
		return nil, llm.Usage{}, llm.ErrNotImplemented.Withf("%s doesn't make images with generateContent", m.Name())
	} else if prompt = strings.TrimSpace(prompt); prompt == "" {
		return nil, llm.Usage{}, llm.ErrBadParameter.With("missing prompt")
	}

	// The images to change, and the prompt, which says how, with the
	// options
	parts := make([]part, 0, len(images)+1)
	for i, image := range images {
		if image == nil {
			return nil, llm.Usage{}, llm.ErrBadParameter.Withf("image %d is missing", i)
		}
		data, err := llm.ReadAll(ctx, image)
		if err != nil {
			return nil, llm.Usage{}, err
		}
		parts = append(parts, part{InlineData: &blob{MimeType: image.Type(), Data: data}})
	}
	parts = append(parts, part{Text: prompt})
	config, err := illustrateOptions(opts...)
	if err != nil {
		return nil, llm.Usage{}, err
	}
	request := generateRequest{Contents: []content{{Role: "user", Parts: parts}}, GenerationConfig: config}

	// Send the request, and receive the images, and what the model said
	payload, err := client.NewJSONRequest(request)
	if err != nil {
		return nil, llm.Usage{}, err
	}
	var response generateResponse
	if err := c.DoWithContext(ctx, payload, &response, client.OptPath("models", m.Name()+":generateContent"), client.OptNoTimeout()); err != nil {
		return nil, llm.Usage{}, err
	}
	var usage llm.Usage
	if response.UsageMetadata != nil {
		usage = replyUsage(*response.UsageMetadata)
	}
	var result []llm.Resource
	var said []string
	for _, candidate := range response.Candidates {
		if candidate.Content == nil {
			continue
		}
		for _, p := range candidate.Content.Parts {
			switch {
			case p.Thought:
				// Drafts, and thinking
			case p.InlineData != nil:
				result = append(result, &schema.Resource{ResourceMeta: schema.ResourceMeta{ContentType: p.InlineData.MimeType, Data: p.InlineData.Data}})
			case strings.TrimSpace(p.Text) != "":
				said = append(said, strings.TrimSpace(p.Text))
			}
		}
	}
	if len(result) == 0 {
		switch {
		case response.PromptFeedback != nil && response.PromptFeedback.BlockReason != "":
			return nil, usage, llm.ErrBadParameter.Withf("the prompt was blocked: %s", response.PromptFeedback.BlockReason)
		case len(said) > 0:
			return nil, usage, llm.ErrBadParameter.Withf("the model made no image, and said: %s", strings.Join(said, " "))
		default:
			return nil, usage, llm.ErrBadParameter.With("the model made no image")
		}
	}
	return result, usage, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// illustrateOptions returns how a model makes images: as images, with any
// text it says, at the aspect ratio, and size, the options set. Returns
// ErrBadParameter for any other option.
func illustrateOptions(opts ...llm.Opt) (*generationConfig, error) {
	o, err := llm.ApplyOpts(opts...)
	if err != nil {
		return nil, err
	}
	config := &generationConfig{ResponseModalities: []string{"TEXT", "IMAGE"}}
	for _, key := range o.Keys() {
		var value string
		switch key {
		case llm.AspectRatioKey:
			if value, err = llm.GetOpt[string](o, key); err == nil {
				config.imageConfig().AspectRatio = strings.TrimSpace(value)
			}
		case llm.ImageSizeKey:
			if value, err = llm.GetOpt[string](o, key); err == nil {
				config.imageConfig().ImageSize = strings.ToUpper(strings.TrimSpace(value))
			}
		default:
			err = llm.ErrBadParameter.Withf("option %q isn't supported by gemini for images", key)
		}
		if err != nil {
			return nil, err
		}
	}
	return config, nil
}

// imageConfig returns the configuration's image configuration, which is
// created when it has none
func (c *generationConfig) imageConfig() *imageConfig {
	if c.ImageConfig == nil {
		c.ImageConfig = &imageConfig{}
	}
	return c.ImageConfig
}
