package agent

import (
	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	anthropic "github.com/mutablelogic/go-llm/pkg/anthropic"
	"github.com/mutablelogic/go-llm/pkg/deepseek"
	gemini "github.com/mutablelogic/go-llm/pkg/gemini"
	mistral "github.com/mutablelogic/go-llm/pkg/mistral"
	ollama "github.com/mutablelogic/go-llm/pkg/ollama"
	openai "github.com/mutablelogic/go-llm/pkg/openai"
)

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func WithOllama(endpoint string, opts ...client.ClientOpt) llm.Opt {
	return func(o *llm.Opts) error {
		client, err := ollama.New(endpoint, opts...)
		if err != nil {
			return err
		} else {
			return llm.WithAgent(client)(o)
		}
	}
}

func WithAnthropic(key string, opts ...client.ClientOpt) llm.Opt {
	return func(o *llm.Opts) error {
		client, err := anthropic.New(key, opts...)
		if err != nil {
			return err
		} else {
			return llm.WithAgent(client)(o)
		}
	}
}

func WithMistral(key string, opts ...client.ClientOpt) llm.Opt {
	return func(o *llm.Opts) error {
		client, err := mistral.New(key, opts...)
		if err != nil {
			return err
		} else {
			return llm.WithAgent(client)(o)
		}
	}
}

func WithOpenAI(key string, opts ...client.ClientOpt) llm.Opt {
	return func(o *llm.Opts) error {
		client, err := openai.New(key, opts...)
		if err != nil {
			return err
		} else {
			return llm.WithAgent(client)(o)
		}
	}
}

func WithGemini(key string, opts ...client.ClientOpt) llm.Opt {
	return func(o *llm.Opts) error {
		client, err := gemini.New(key)
		if err != nil {
			return err
		} else {
			return llm.WithAgent(client)(o)
		}
	}
}

func WithDeepSeek(key string, opts ...client.ClientOpt) llm.Opt {
	return func(o *llm.Opts) error {
		client, err := deepseek.New(key, opts...)
		if err != nil {
			return err
		} else {
			return llm.WithAgent(client)(o)
		}
	}
}
