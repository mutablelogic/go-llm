package impl_test

import (
	"context"
	"testing"

	// Packages
	"github.com/mutablelogic/go-llm"
	"github.com/mutablelogic/go-llm/pkg/internal/impl"
	"github.com/mutablelogic/go-llm/pkg/tool"
	"github.com/stretchr/testify/assert"
)

func Test_Opt_001(t *testing.T) {
	assert := assert.New(t)
	opts, err := llm.ApplyOpts()
	if assert.NoError(err) {
		assert.NotNil(opts)
	}

	t.Run("OptFrequencyPenalty", func(t *testing.T) {
		opts, err := llm.ApplyOpts(llm.WithFrequencyPenalty(-0.5))
		if assert.NoError(err) {
			assert.NotNil(opts)
			assert.Equal(-0.5, impl.OptFrequencyPenalty(opts))
		}
	})

	t.Run("OptPresencePenalty", func(t *testing.T) {
		opts, err := llm.ApplyOpts(llm.WithPresencePenalty(-0.5))
		if assert.NoError(err) {
			assert.NotNil(opts)
			assert.Equal(-0.5, impl.OptPresencePenalty(opts))
		}
	})

	t.Run("OptMaxTokens", func(t *testing.T) {
		opts, err := llm.ApplyOpts(llm.WithMaxTokens(100))
		if assert.NoError(err) {
			assert.NotNil(opts)
			assert.Equal(uint64(100), impl.OptMaxTokens(nil, opts))
		}
	})

	t.Run("OptStream", func(t *testing.T) {
		opts, err := llm.ApplyOpts(llm.WithStream(func(llm.Completion) {}))
		if assert.NoError(err) {
			assert.NotNil(opts)
			assert.True(impl.OptStream(opts))
		}
	})

	t.Run("OptStreamOptions", func(t *testing.T) {
		opts, err := llm.ApplyOpts(llm.WithStream(func(llm.Completion) {}))
		if assert.NoError(err) {
			assert.NotNil(opts)
			assert.Equal(impl.NewStreamOptions(true), impl.OptStreamOptions(opts))
		}
	})

	t.Run("OptStreamOptionsNil", func(t *testing.T) {
		if assert.NoError(err) {
			assert.NotNil(opts)
			assert.Nil(impl.OptStreamOptions(opts))
		}
	})

	t.Run("OptStopSequencesNil", func(t *testing.T) {
		if assert.NoError(err) {
			assert.NotNil(opts)
			assert.Nil(impl.OptStopSequences(opts))
		}
	})

	t.Run("OptStopSequences", func(t *testing.T) {
		opts, err := llm.ApplyOpts(llm.WithStopSequence("a", "b"))
		if assert.NoError(err) {
			assert.NotNil(opts)
			assert.Equal([]string{"a", "b"}, impl.OptStopSequences(opts))
		}
	})

	t.Run("OptTemperature", func(t *testing.T) {
		opts, err := llm.ApplyOpts(llm.WithTemperature(0.5))
		if assert.NoError(err) {
			assert.NotNil(opts)
			assert.Equal(0.5, impl.OptTemperature(opts))
		}
	})

	t.Run("OptTopP", func(t *testing.T) {
		opts, err := llm.ApplyOpts(llm.WithTopP(0.5))
		if assert.NoError(err) {
			assert.NotNil(opts)
			assert.Equal(0.5, impl.OptTopP(opts))
		}
	})

	t.Run("OptTools", func(t *testing.T) {
		toolkit := tool.NewToolKit()
		toolkit.Register(new(example_tool))
		opts, err := llm.ApplyOpts(llm.WithToolKit(toolkit))
		if assert.NoError(err) {
			assert.NotNil(opts)
			assert.Len(impl.OptTools(nil, opts), 1)
		}
	})

	t.Run("OptToolChoiceNil", func(t *testing.T) {
		if assert.NoError(err) {
			assert.NotNil(opts)
			assert.Nil(nil, impl.OptToolChoice(opts))
		}
	})

	t.Run("OptToolChoiceAuto", func(t *testing.T) {
		toolkit := tool.NewToolKit()
		opts, err := llm.ApplyOpts(llm.WithToolKit(toolkit), llm.WithToolChoice("auto"))
		if assert.NoError(err) {
			assert.NotNil(opts)
			assert.Equal("auto", impl.OptToolChoice(opts))
		}
	})

	t.Run("OptToolChoiceNone", func(t *testing.T) {
		toolkit := tool.NewToolKit()
		opts, err := llm.ApplyOpts(llm.WithToolKit(toolkit), llm.WithToolChoice("none"))
		if assert.NoError(err) {
			assert.NotNil(opts)
			assert.Equal("none", impl.OptToolChoice(opts))
		}
	})

	t.Run("OptToolChoiceRequired", func(t *testing.T) {
		toolkit := tool.NewToolKit()
		opts, err := llm.ApplyOpts(llm.WithToolKit(toolkit), llm.WithToolChoice("required"))
		if assert.NoError(err) {
			assert.NotNil(opts)
			assert.Equal("required", impl.OptToolChoice(opts))
		}
	})
}

type example_tool struct {
}

func (example_tool) Description() string {
	return "An example tool"
}

func (example_tool) Name() string {
	return "example_tool"
}

func (example_tool) Run(ctx context.Context) (any, error) {
	return nil, nil
}
