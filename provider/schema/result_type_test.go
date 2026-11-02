package schema_test

import (
	"encoding/json"
	"testing"

	// Packages
	schema "github.com/mutablelogic/go-llm/provider/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_ResultType_JSON(t *testing.T) {
	for _, r := range []schema.ResultType{
		schema.ResultStop, schema.ResultMaxTokens, schema.ResultBlocked, schema.ResultToolCall,
		schema.ResultError, schema.ResultOther, schema.ResultMaxIterations,
	} {
		t.Run(r.String(), func(t *testing.T) {
			data, err := json.Marshal(r)
			require.NoError(t, err)
			assert.Equal(t, `"`+r.String()+`"`, string(data))

			var out schema.ResultType
			require.NoError(t, json.Unmarshal(data, &out))
			assert.Equal(t, r, out)
		})
	}

	t.Run("Unknown", func(t *testing.T) {
		var out schema.ResultType
		assert.Error(t, json.Unmarshal([]byte(`"finished"`), &out))
	})
}
