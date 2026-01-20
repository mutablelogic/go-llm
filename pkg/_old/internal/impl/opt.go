package impl

import (
	"strings"

	// Packages
	"github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type ResponseFormat struct {
	// Supported response format types are text, json_object or json_schema
	Type string `json:"type"`
}

type StreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type ToolChoice struct {
	Type     string `json:"type"`
	Function struct {
		Name string `json:"name"`
	} `json:"function"`
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func NewToolChoice(function string) *ToolChoice {
	choice := new(ToolChoice)
	choice.Type = "function"
	choice.Function.Name = strings.TrimSpace(strings.ToLower(function))
	return choice
}

func NewStreamOptions(include_usage bool) *StreamOptions {
	return &StreamOptions{IncludeUsage: include_usage}
}

func NewResponseFormat(format string) *ResponseFormat {
	format = strings.TrimSpace(strings.ToLower(format))
	switch format {
	case "text", "json_object":
		return &ResponseFormat{Type: format}
	default:
		// json_schema is not yet supported
		return nil
	}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func OptFrequencyPenalty(opts *llm.Opts) float64 {
	return opts.GetFloat64("frequency_penalty")
}

func OptPresencePenalty(opts *llm.Opts) float64 {
	return opts.GetFloat64("presence_penalty")
}

func OptMaxTokens(model llm.Model, opts *llm.Opts) uint64 {
	return opts.GetUint64("max_tokens")
}

func OptStream(opts *llm.Opts) bool {
	return opts.StreamFn() != nil
}

func OptStreamOptions(opts *llm.Opts) *StreamOptions {
	if OptStream(opts) {
		return NewStreamOptions(true)
	} else {
		return nil
	}
}

func OptStopSequences(opts *llm.Opts) []string {
	if opts.Has("stop") {
		if stop, ok := opts.Get("stop").([]string); ok {
			return stop
		}
	}
	return nil
}

func OptTemperature(opts *llm.Opts) float64 {
	return opts.GetFloat64("temperature")
}

func OptTopP(opts *llm.Opts) float64 {
	return opts.GetFloat64("top_p")
}

func OptResponseFormat(opts *llm.Opts) *ResponseFormat {
	if format := NewResponseFormat(opts.GetString("format")); format != nil {
		return format
	} else {
		return nil
	}
}

func OptTools(agent llm.Agent, opts *llm.Opts) []llm.Tool {
	toolkit := opts.ToolKit()
	if toolkit == nil {
		return nil
	}
	return toolkit.Tools(agent.Name())
}

func OptToolChoice(opts *llm.Opts) any {
	// nil if no toolkit is defined
	toolkit := opts.ToolKit()
	if toolkit == nil {
		return nil
	}

	// Get the tool choice
	choices, ok := opts.Get("tool_choice").([]string)
	if !ok || len(choices) == 0 {
		return nil
	}

	// We only support one choice
	choice := strings.TrimSpace(strings.ToLower(choices[0]))
	switch choice {
	case "auto", "none", "required":
		return choice
	case "":
		return nil
	default:
		return NewToolChoice(choice)
	}
}

func OptLogProbs(opts *llm.Opts) bool {
	return opts.GetBool("logprobs")
}

func OptTopLogProbs(opts *llm.Opts) uint64 {
	return opts.GetUint64("top_logprobs")
}
