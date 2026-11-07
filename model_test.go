package llm_test

import (
	"encoding/json"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_ModelCap(t *testing.T) {

	t.Run("String", func(t *testing.T) {
		// The capabilities which are known to be true
		assert.Empty(t, llm.ModelCap{}.String())
		assert.Equal(t, "chat, tools, thinking, embedding", llm.ModelCap{
			Chat: types.Ptr(true), Tools: types.Ptr(true), Thinking: types.Ptr(true), ImageInput: types.Ptr(false), Embedding: types.Ptr(true),
		}.String())
	})

	t.Run("JSON", func(t *testing.T) {
		// Capabilities which aren't known are left out, and those which
		// are known to be false aren't
		for _, test := range []struct {
			cap  llm.ModelCap
			json string
		}{
			{llm.ModelCap{}, `{}`},
			{llm.ModelCap{Chat: types.Ptr(true), ImageInput: types.Ptr(false), Thinking: types.Ptr(false)}, `{"chat":true,"thinking":false,"image_input":false}`},
		} {
			data, err := json.Marshal(test.cap)
			require.NoError(t, err)
			assert.JSONEq(t, test.json, string(data))
			var result llm.ModelCap
			require.NoError(t, json.Unmarshal(data, &result))
			assert.Equal(t, test.cap, result)
		}
	})
}

// capModel is a model with only its name and capabilities
type capModel struct {
	llm.Model
	name string
	caps llm.ModelCap
}

func (m capModel) Name() string               { return m.name }
func (m capModel) Capabilities() llm.ModelCap { return m.caps }

func Test_WithCapabilities(t *testing.T) {
	yes, no := types.Ptr(true), types.Ptr(false)
	models := []llm.Model{
		capModel{name: "chat", caps: llm.ModelCap{Chat: yes, Tools: yes, ImageInput: no}},
		capModel{name: "eyes", caps: llm.ModelCap{Chat: yes, ImageInput: yes}},
		capModel{name: "ocr", caps: llm.ModelCap{OCR: yes}},
		capModel{name: "unknown"},
	}
	names := func(models []llm.Model) []string {
		var result []string
		for _, model := range models {
			result = append(result, model.Name())
		}
		return result
	}

	// Without capabilities, every model, and with them, those known to have
	// every one, by their names, in any case
	all, err := llm.WithCapabilities(models)
	require.NoError(t, err)
	assert.Len(t, all, 4)
	got, err := llm.WithCapabilities(models, "chat")
	require.NoError(t, err)
	assert.Equal(t, []string{"chat", "eyes"}, names(got))
	got, err = llm.WithCapabilities(models, "Chat", " image_input ")
	require.NoError(t, err)
	assert.Equal(t, []string{"eyes"}, names(got))
	got, err = llm.WithCapabilities(models, "speech_output")
	require.NoError(t, err)
	assert.Empty(t, got)

	// A capability which isn't one, or the temperature, which is a setting
	for _, name := range []string{"telepathy", "temperature"} {
		_, err = llm.WithCapabilities(models, name)
		assert.ErrorIs(t, err, llm.ErrBadParameter, name)
	}
	assert.Equal(t, []string{"chat", "image_input"}, llm.ModelCap{Chat: yes, ImageInput: yes, Tools: no}.Names())
}
