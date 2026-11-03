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
		assert.Equal(t, "completion, tools, thinking, embedding", llm.ModelCap{
			Completion: types.Ptr(true), Tools: types.Ptr(true), Thinking: types.Ptr(true), Vision: types.Ptr(false), Embedding: types.Ptr(true),
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
			{llm.ModelCap{Completion: types.Ptr(true), Vision: types.Ptr(false), Thinking: types.Ptr(false)}, `{"completion":true,"thinking":false,"vision":false}`},
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
