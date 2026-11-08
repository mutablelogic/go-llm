package openai

import (
	"encoding/json"
	"regexp"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// reasoningModel matches the name of one of OpenAI's reasoning models, with
// the least effort it accepts: "none" for GPT-5.1 and later, and GPT-6 Sol
// and Luna, which turns reasoning off, "minimal" for GPT-5, and "low" for
// the o-series and GPT-6 Astra and GPT-6.1 Sol, which can't turn it off.
var reasoningModel = []struct {
	re     *regexp.Regexp
	effort string
}{
	{regexp.MustCompile(`^gpt-6(\.\d+)?-astra|^gpt-6\.1-sol`), "low"},
	{regexp.MustCompile(`^gpt-6`), "none"},
	{regexp.MustCompile(`^gpt-5\.\d`), "none"},
	{regexp.MustCompile(`^gpt-5(-|$)`), "minimal"},
	{regexp.MustCompile(`^o\d(-|$)`), "low"},
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// apply sets the options on the request to embed, or returns an error if
// any option isn't valid or isn't supported by the Embeddings API. The
// number of dimensions is the API's dimensions, which OpenAI's
// text-embedding-3 models support, or Mistral's output_dimension, and the
// user identifies the end user, which Mistral doesn't support. The API has
// no task type or title, so they're ignored, as a caller can set them for
// any provider.
func (r *embedRequest) apply(server server, opts ...llm.Opt) error {
	o, err := llm.ApplyOpts(opts...)
	if err != nil {
		return err
	}
	for _, key := range o.Keys() {
		var err error
		switch key {
		case llm.DimensionsKey:
			if server == serverMistral {
				r.OutputDimension, err = llm.GetOpt[uint](o, key)
			} else {
				r.Dimensions, err = llm.GetOpt[uint](o, key)
			}
		case llm.UserKey:
			if server == serverMistral {
				err = llm.ErrBadParameter.Withf("option %q isn't supported by mistral", key)
			} else {
				r.User, err = llm.GetOpt[string](o, key)
			}
		case llm.TaskTypeKey, llm.TitleKey:
			// The embeddings API has no task type or title
		default:
			err = llm.ErrBadParameter.Withf("option %q isn't supported by openai for embeddings", key)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// transcribeOptions are the options of a transcript which aren't in the
// request: the language, when it's set, which a model may not return, and
// whether the transcript has its segments, and their speakers
type transcribeOptions struct {
	language           string
	segments, speakers bool
}

// apply sets the options on the request to transcribe, or translate, and
// the format of the response, for the model's family, or returns an error
// if any option isn't valid or isn't supported. Mistral returns segments,
// and their speakers, which it's asked for. OpenAI's diarizing models
// return segments with their speakers, and whisper, and other models,
// return segments, without speakers, which OpenAI's other models don't
// return. A translation is into English, so it has no language.
func (r *transcribeRequest) apply(family transcribeFamily, model string, translate bool, opts ...llm.Opt) (transcribeOptions, error) {
	var options transcribeOptions
	o, err := llm.ApplyOpts(opts...)
	if err != nil {
		return options, err
	}
	for _, key := range o.Keys() {
		var err error
		switch key {
		case llm.LanguageKey:
			if translate {
				err = llm.ErrBadParameter.Withf("option %q: a translation is into english", key)
			} else if options.language, err = llm.GetOpt[string](o, key); err == nil {
				// The language is checked as WithLanguage checks it, as it
				// can be set with WithOpt, such as from a request
				if _, err = llm.ApplyOpts(llm.WithLanguage(options.language)); err == nil {
					r.Language = options.language
				}
			}
		case llm.TemperatureKey:
			r.Temperature, err = getOpt[float64](o, key)
		case llm.SegmentsKey:
			options.segments, err = llm.GetOpt[bool](o, key)
		case llm.SpeakersKey:
			options.speakers, err = llm.GetOpt[bool](o, key)
		default:
			err = llm.ErrBadParameter.Withf("option %q isn't supported for transcription", key)
		}
		if err != nil {
			return options, err
		}
	}

	// The format of the response, and its segments, for the model's family
	switch family {
	case transcribeMistral:
		if options.speakers {
			r.Diarize = true
		}
		if options.segments || options.speakers {
			r.TimestampGranularity = "segment"
		}
	case transcribeDiarize:
		r.ResponseFormat, r.ChunkingStrategy = "diarized_json", "auto"
	case transcribeText:
		if options.segments || options.speakers {
			return options, llm.ErrBadParameter.Withf("model %q doesn't return segments", model)
		}
		r.ResponseFormat = "json"
	default:
		if options.speakers {
			return options, llm.ErrBadParameter.Withf("option %q: model %q doesn't distinguish speakers", llm.SpeakersKey, model)
		}
		r.ResponseFormat = "verbose_json"
		if options.segments {
			r.TimestampGranularities = "segment"
		}
	}
	return options, nil
}

// apply sets the options on the request to recognize text, or returns an
// error if any option isn't valid or isn't supported by Mistral's OCR API
func (r *ocrRequest) apply(opts ...llm.Opt) error {
	o, err := llm.ApplyOpts(opts...)
	if err != nil {
		return err
	}
	for _, key := range o.Keys() {
		var err error
		switch key {
		case llm.PagesKey:
			r.Pages, err = llm.GetPages(o)
		case llm.TableFormatKey:
			if r.TableFormat, err = llm.GetOpt[string](o, key); err == nil && r.TableFormat != "markdown" && r.TableFormat != "html" {
				err = llm.ErrBadParameter.Withf("option %q: %q must be markdown or html", key, r.TableFormat)
			}
		case llm.RegionsKey:
			var regions bool
			if regions, err = llm.GetOpt[bool](o, key); err == nil {
				r.IncludeBlocks = types.Ptr(regions)
			}
		default:
			err = llm.ErrBadParameter.Withf("option %q isn't supported by mistral for recognition", key)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// apply sets the options on the request, or returns an error if any option
// isn't valid or isn't supported. The system prompt is prepended to the
// messages, so they should be set first, as should the tools, which are
// removed when the tool choice is "none". The most tokens to generate are
// max_completion_tokens for OpenAI's own API, and max_tokens otherwise. The
// context window is the server's, and isn't sent. The seed is Mistral's
// random_seed, and Mistral has no user, as it rejects fields it doesn't
// know. Thinking turns DeepSeek's thinking mode on or off, and off is the
// least reasoning effort of the model for OpenAI's own API, as not every
// model can turn it off, and for Mistral's, "none", and on, "high", unless
// there's an effort. Thinking is ignored by other servers, which have no way
// to say it, but an effort is sent. OpenAI's models which can turn reasoning off
// have it off when there are tools, as Chat Completions only lets them call
// tools without reasoning. The request's model should be set first.
// Reference: https://platform.openai.com/docs/api-reference/chat/create
func (r *chatRequest) apply(server server, opts ...llm.Opt) error {
	o, err := llm.ApplyOpts(opts...)
	if err != nil {
		return err
	}
	for _, key := range o.Keys() {
		var err error
		switch key {
		case llm.TemperatureKey:
			r.Temperature, err = getOpt[float64](o, key)
		case llm.TopPKey:
			r.TopP, err = getOpt[float64](o, key)
		case llm.MaxTokensKey:
			if server == serverOpenAI {
				r.MaxCompletionTokens, err = getOpt[uint](o, key)
			} else {
				r.MaxTokens, err = getOpt[uint](o, key)
			}
		case llm.StopSequencesKey:
			r.Stop, err = llm.GetOpt[[]string](o, key)
		case llm.SeedKey:
			if server == serverMistral {
				r.RandomSeed, err = getOpt[uint](o, key)
			} else {
				r.Seed, err = getOpt[uint](o, key)
			}
		case llm.PresencePenaltyKey:
			r.PresencePenalty, err = getOpt[float64](o, key)
		case llm.FrequencyPenaltyKey:
			r.FrequencyPenalty, err = getOpt[float64](o, key)
		case llm.ContextWindowKey:
			_, err = llm.GetOpt[uint](o, key)
		case llm.SystemPromptKey:
			var prompt string
			if prompt, err = llm.GetOpt[string](o, key); err == nil {
				r.Messages = append([]chatMessage{{Role: "system", Content: prompt}}, r.Messages...)
			}
		case llm.ThinkingKey:
			// Ignored by other servers, which have no way to say it, as a
			// conversation keeps its options as its model changes
			_, err = llm.GetOpt[bool](o, key)
		case llm.ThinkingEffortKey:
			r.ReasoningEffort, err = llm.GetOpt[string](o, key)
		case llm.JSONSchemaKey:
			var schema json.RawMessage
			if schema, err = llm.GetOpt[json.RawMessage](o, key); err == nil {
				r.ResponseFormat = &responseFormat{Type: "json_schema", JSONSchema: &responseSchema{Name: "response", Schema: schema}}
			}
		case llm.ToolChoiceKey:
			var choice string
			if choice, err = llm.GetOpt[string](o, key); err == nil {
				switch choice {
				case "none":
					r.Tools = nil
				case "auto":
					r.ToolChoice = "auto"
				case "any":
					r.ToolChoice = "required"
				default:
					err = llm.ErrBadParameter.Withf("tool choice %q isn't supported", choice)
				}
			}
		case llm.UserKey:
			if server == serverMistral {
				err = llm.ErrBadParameter.Withf("option %q isn't supported by mistral", key)
			} else {
				r.User, err = llm.GetOpt[string](o, key)
			}
		case llm.ServiceTierKey:
			r.ServiceTier, err = llm.GetOpt[string](o, key)
		default:
			err = llm.ErrBadParameter.Withf("option %q isn't supported by openai", key)
		}
		if err != nil {
			return err
		}
	}

	// Thinking is DeepSeek's thinking mode, or when it's off, the least
	// reasoning effort for OpenAI's reasoning models, and Mistral's, rather
	// than the effort, and when it's on, Mistral's most, unless there's an
	// effort
	if thinking, err := llm.GetOpt[bool](o, llm.ThinkingKey); err == nil {
		switch {
		case server == serverDeepSeek && thinking:
			r.Thinking = &chatThinking{Type: "enabled"}
		case server == serverDeepSeek:
			r.Thinking = &chatThinking{Type: "disabled"}
		case server == serverOpenAI && !thinking:
			r.ReasoningEffort = leastReasoningEffort(r.Model)
		case server == serverMistral && !thinking:
			r.ReasoningEffort = "none"
		case server == serverMistral && r.ReasoningEffort == "":
			r.ReasoningEffort = "high"
		}
	}

	// OpenAI's newer models only call tools with Chat Completions when their
	// reasoning is off, so it's off when there are tools, until the
	// Responses API is used, which can do both
	// Reference: https://developers.openai.com/api/docs/guides/latest-model
	if server == serverOpenAI && len(r.Tools) > 0 && leastReasoningEffort(r.Model) == "none" {
		r.ReasoningEffort = "none"
	}

	// A tool choice needs tools
	if len(r.Tools) == 0 {
		r.ToolChoice = ""
	}

	// Return success
	return nil
}

// leastReasoningEffort returns the least reasoning effort of one of OpenAI's
// models, or empty for a model which doesn't reason, or isn't known, which
// isn't sent an effort, as it doesn't accept one
func leastReasoningEffort(model string) string {
	for _, family := range reasoningModel {
		if family.re.MatchString(model) {
			return family.effort
		}
	}
	return ""
}

// getOpt returns the option with the key, as a pointer to its value
func getOpt[T any](o *llm.Opts, key string) (*T, error) {
	value, err := llm.GetOpt[T](o, key)
	if err != nil {
		return nil, err
	}
	return types.Ptr(value), nil
}

// chatTools returns the tools as the tools in a request, with each tool's
// input schema as its parameters, or nil when there are no tools. A tool
// without an input schema has no parameters. Returns an error if a tool is
// nil, or its name isn't valid or unique.
func chatTools(tools []llm.Tool) ([]chatTool, error) {
	if len(tools) == 0 {
		return nil, nil
	}
	result := make([]chatTool, 0, len(tools))
	names := make(map[string]bool, len(tools))
	for _, tool := range tools {
		if tool == nil {
			return nil, llm.ErrBadParameter.With("missing tool")
		} else if name := tool.Name(); !llm.IsToolName(name) {
			return nil, llm.ErrBadParameter.Withf("invalid tool name %q", name)
		} else if names[name] {
			return nil, llm.ErrBadParameter.Withf("duplicate tool %q", name)
		} else {
			names[name] = true
		}
		parameters := json.RawMessage(`{"type":"object","properties":{}}`)
		if schema := tool.InputSchema(); schema != nil {
			data, err := json.Marshal(schema)
			if err != nil {
				return nil, llm.ErrBadParameter.Withf("tool %q: %v", tool.Name(), err)
			}
			parameters = data
		}
		result = append(result, chatTool{
			Type: "function",
			Function: chatToolFunction{
				Name:        tool.Name(),
				Description: tool.Description(),
				Parameters:  parameters,
			},
		})
	}
	return result, nil
}
