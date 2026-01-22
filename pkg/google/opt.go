package google

import (
	"fmt"

	opt "github.com/mutablelogic/go-llm/pkg/opt"
)

////////////////////////////////////////////////////////////////////////////////
// GOOGLE OPTIONS

// WithSystemPrompt sets the system instruction for the request
func WithSystemPrompt(value string) opt.Opt {
	return opt.SetString("system", value)
}

// WithTemperature sets the temperature for the request (0.0 to 2.0)
func WithTemperature(value float64) opt.Opt {
	if value < 0 || value > 2 {
		return opt.Error(fmt.Errorf("temperature must be between 0.0 and 2.0"))
	}
	return opt.SetFloat64("temperature", value)
}

// WithMaxTokens sets the maximum number of tokens to generate (minimum 1)
func WithMaxTokens(value uint) opt.Opt {
	if value < 1 {
		return opt.Error(fmt.Errorf("max_tokens must be at least 1"))
	}
	return opt.SetUint("max_tokens", value)
}

// WithTopK sets the top K sampling parameter (minimum 1)
func WithTopK(value uint) opt.Opt {
	if value < 1 {
		return opt.Error(fmt.Errorf("top_k must be at least 1"))
	}
	return opt.SetUint("top_k", value)
}

// WithTopP sets the nucleus sampling parameter (0.0 to 1.0)
func WithTopP(value float64) opt.Opt {
	if value < 0 || value > 1 {
		return opt.Error(fmt.Errorf("top_p must be between 0.0 and 1.0"))
	}
	return opt.SetFloat64("top_p", value)
}

// WithStopSequences sets custom stop sequences for the request
func WithStopSequences(values ...string) opt.Opt {
	if len(values) == 0 {
		return opt.Error(fmt.Errorf("at least one stop sequence is required"))
	}
	return opt.AddString("stop_sequences", values...)
}
