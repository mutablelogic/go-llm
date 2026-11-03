package llm_test

import (
	"encoding/json"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_ModelCap(t *testing.T) {
	yes, no := true, false

	t.Run("String", func(t *testing.T) {
		// The capabilities which are known to be true
		assert.Empty(t, llm.ModelCap{}.String())
		assert.Equal(t, "completion, tools, thinking, embedding", llm.ModelCap{
			Completion: &yes, Tools: &yes, Thinking: &yes, Vision: &no, Embedding: &yes,
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
			{llm.ModelCap{Completion: &yes, Vision: &no, Thinking: &no}, `{"completion":true,"thinking":false,"vision":false}`},
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
