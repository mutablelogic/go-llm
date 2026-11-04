package registry_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	registry "github.com/mutablelogic/go-llm/registry"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// measuring is a provider which measures text as a token for each word,
// or fails with the error, when it's set
type measuring struct {
	*scripted
	err error
}

var _ llm.Measurer = (*measuring)(nil)

func (m *measuring) Self() llm.Provider { return m }

func (m *measuring) GetModel(ctx context.Context, name string) (llm.Model, error) {
	if _, err := m.scripted.GetModel(ctx, name); err != nil {
		return nil, err
	}
	return &model{name: name, provider: m}, nil
}

func (m *measuring) ListModels(context.Context) ([]llm.Model, error) {
	var result []llm.Model
	for _, name := range m.models {
		result = append(result, &model{name: name, provider: m})
	}
	return result, nil
}

func (m *measuring) Measure(_ context.Context, _ llm.Model, text string) (uint, error) {
	if m.err != nil {
		return 0, m.err
	}
	return uint(len(strings.Fields(text))), nil
}

func Test_Measure(t *testing.T) {
	ctx := context.Background()

	t.Run("Estimate", func(t *testing.T) {
		// A provider which doesn't measure text has it estimated, as a token
		// for every four bytes, or part of them
		r, err := registry.New()
		require.NoError(t, err)
		require.NoError(t, r.Add("", newScripted()))
		m, err := r.GetModel(ctx, "model")
		require.NoError(t, err)
		for text, tokens := range map[string]uint{"": 0, "a": 1, "abcd": 1, "abcde": 2, "one two three": 4} {
			measured, err := r.Measure(ctx, m, text)
			require.NoError(t, err)
			assert.Equal(t, tokens, measured, text)
		}
	})

	t.Run("Provider", func(t *testing.T) {
		// A provider which measures text does
		r, err := registry.New()
		require.NoError(t, err)
		require.NoError(t, r.Add("", &measuring{scripted: newScripted()}))
		m, err := r.GetModel(ctx, "model")
		require.NoError(t, err)
		measured, err := r.Measure(ctx, m, "one two three")
		require.NoError(t, err)
		assert.Equal(t, uint(3), measured)
	})

	t.Run("ProviderError", func(t *testing.T) {
		// The provider's error is returned
		r, err := registry.New()
		require.NoError(t, err)
		require.NoError(t, r.Add("", &measuring{scripted: newScripted(), err: errors.New("unavailable")}))
		m, err := r.GetModel(ctx, "model")
		require.NoError(t, err)
		_, err = r.Measure(ctx, m, "text")
		assert.ErrorContains(t, err, "unavailable")
	})

	t.Run("NotInRegistry", func(t *testing.T) {
		// A model whose provider isn't in the registry isn't measured
		r, err := registry.New()
		require.NoError(t, err)
		_, err = r.Measure(ctx, &model{name: "model", provider: newScripted()}, "text")
		assert.ErrorIs(t, err, llm.ErrNotFound)
	})
}
