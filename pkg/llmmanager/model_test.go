package manager

import (
	"testing"

	// Packages
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	assert "github.com/stretchr/testify/assert"
)

func TestModelsByName(t *testing.T) {
	assert := assert.New(t)

	models := modelsByName([]schema.Model{
		{Name: "claude-sonnet", OwnedBy: "anthropic"},
		{Name: "gemini-pro", OwnedBy: "google"},
		{Name: "claude-sonnet", OwnedBy: "proxy"},
	}, "claude-sonnet")

	if assert.Len(models, 2) {
		assert.Equal("anthropic", models[0].OwnedBy)
		assert.Equal("proxy", models[1].OwnedBy)
	}
}

func TestSingleModel(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		assert := assert.New(t)
		_, err := singleModel(nil, "missing")
		if assert.Error(err) {
			assert.ErrorIs(err, schema.ErrNotFound)
		}
	})

	t.Run("single", func(t *testing.T) {
		assert := assert.New(t)
		model, err := singleModel([]schema.Model{{Name: "claude-sonnet", OwnedBy: "anthropic"}}, "claude-sonnet")
		if !assert.NoError(err) {
			return
		}
		assert.Equal("anthropic", model.OwnedBy)
	})

	t.Run("multiple", func(t *testing.T) {
		assert := assert.New(t)
		_, err := singleModel([]schema.Model{{Name: "claude-sonnet"}, {Name: "claude-sonnet"}}, "claude-sonnet")
		if assert.Error(err) {
			assert.ErrorIs(err, schema.ErrConflict)
		}
	})
}
