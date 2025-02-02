package mistral

import (
	"strings"

	"github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func WithPrediction(v string) llm.Opt {
	return func(o *llm.Opts) error {
		o.Set("prediction", v)
		return nil
	}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func optTemperature(opts *llm.Opts) float64 {
	return opts.GetFloat64("temperature")
}

func optTopP(opts *llm.Opts) float64 {
	return opts.GetFloat64("top_p")
}

func optMaxTokens(opts *llm.Opts) uint64 {
	return opts.GetUint64("max_tokens")
}

func optStream(opts *llm.Opts) bool {
	return opts.StreamFn() != nil
}

func optStopSequences(opts *llm.Opts) []string {
	if opts.Has("stop") {
		if stop, ok := opts.Get("stop").([]string); ok {
			return stop
		}
	}
	return nil
}

func optSeed(opts *llm.Opts) uint64 {
	return opts.GetUint64("seed")
}

func optFormat(opts *llm.Opts) any {
	var fmt struct {
		Type string `json:"type"`
	}
	format := opts.GetString("format")
	if format == "" {
		return nil
	} else {
		fmt.Type = format
	}
	return fmt
}

func optTools(agent llm.Agent, opts *llm.Opts) []llm.Tool {
	toolkit := opts.ToolKit()
	if toolkit == nil {
		return nil
	}
	return toolkit.Tools(agent)
}

func optToolChoice(opts *llm.Opts) any {
	choices, ok := opts.Get("tool_choice").([]string)
	if !ok || len(choices) == 0 {
		return nil
	}

	// We only support one choice
	choice := strings.TrimSpace(strings.ToLower(choices[0]))
	switch choice {
	case "auto", "none", "any", "required":
		return choice
	case "":
		return nil
	default:
		var fn struct {
			Type     string `json:"type"`
			Function struct {
				Name string `json:"name"`
			} `json:"function"`
		}
		fn.Type = "function"
		fn.Function.Name = choice
		return fn
	}
}

func optPresencePenalty(opts *llm.Opts) float64 {
	return opts.GetFloat64("presence_penalty")
}

func optFrequencyPenalty(opts *llm.Opts) float64 {
	return opts.GetFloat64("frequency_penalty")
}

func optNumCompletions(opts *llm.Opts) uint64 {
	return opts.GetUint64("num_completions")
}

func optPrediction(opts *llm.Opts) *Content {
	prediction := strings.TrimSpace(opts.GetString("prediction"))
	if prediction == "" {
		return nil
	}
	return NewContent("content", "", prediction)
}

func optSafePrompt(opts *llm.Opts) bool {
	return opts.GetBool("safe_prompt")
}
