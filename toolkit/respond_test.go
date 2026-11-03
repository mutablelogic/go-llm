package toolkit_test

import (
	"context"
	"encoding/json"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_Respond(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}`)

	t.Run("Tool", func(t *testing.T) {
		// The input is an object, with the reply as its result
		tool, err := toolkit.NewRespond(schema)
		require.NoError(t, err)
		assert.Equal(t, toolkit.RespondName, tool.Name())
		assert.NotEmpty(t, tool.Description())
		assert.Nil(t, tool.OutputSchema())
		require.NotNil(t, tool.InputSchema())
		data, err := json.Marshal(tool.InputSchema())
		require.NoError(t, err)
		assert.JSONEq(t, `{"type":"object","properties":{"result":{"$id":"urn:llm:respond:result","type":"object","properties":{"city":{"type":"string"}},"required":["city"]}},"required":["result"],"additionalProperties":false}`, string(data))
	})

	t.Run("Run", func(t *testing.T) {
		// The result is the reply, when it conforms
		tool, err := toolkit.NewRespond(schema)
		require.NoError(t, err)
		result, err := tool.Run(context.Background(), json.RawMessage(`{"result":{"city":"London"}}`))
		require.NoError(t, err)
		assert.JSONEq(t, `{"city":"London"}`, string(result.(json.RawMessage)))

		// Otherwise it's an error the model can correct
		for _, input := range []string{``, `{}`, `{"result":{"city":1}}`, `{"result":{"city":"London"},"other":1}`} {
			_, err := tool.Run(context.Background(), json.RawMessage(input))
			assert.ErrorIs(t, err, llm.ErrBadParameter, input)
		}
	})

	t.Run("AnyJSON", func(t *testing.T) {
		// The reply can be any JSON, such as a list
		tool, err := toolkit.NewRespond(json.RawMessage(`{"type":"array","items":{"type":"integer"}}`))
		require.NoError(t, err)
		result, err := tool.Run(context.Background(), json.RawMessage(`{"result":[1,2,3]}`))
		require.NoError(t, err)
		assert.JSONEq(t, `[1,2,3]`, string(result.(json.RawMessage)))
	})

	t.Run("References", func(t *testing.T) {
		// A reference to a definition, or the schema itself, resolves
		// against the reply's schema, rather than the tool's input
		for _, tt := range []struct {
			schema  string
			valid   []string
			invalid []string
		}{
			{
				`{"$defs":{"City":{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}},"type":"array","items":{"$ref":"#/$defs/City"}}`,
				[]string{`[{"name":"London"}]`, `[]`},
				[]string{`[{"other":"London"}]`, `[1]`},
			},
			{
				`{"type":"object","properties":{"name":{"type":"string"},"children":{"type":"array","items":{"$ref":"#"}}},"required":["name"]}`,
				[]string{`{"name":"a"}`, `{"name":"a","children":[{"name":"b","children":[{"name":"c"}]}]}`},
				[]string{`{"name":"a","children":[{"other":"b"}]}`},
			},
			{
				// A schema with its own identifier keeps it
				`{"$id":"urn:example:city","$defs":{"City":{"type":"string"}},"$ref":"#/$defs/City"}`,
				[]string{`"London"`},
				[]string{`1`},
			},
		} {
			tool, err := toolkit.NewRespond(json.RawMessage(tt.schema))
			require.NoError(t, err, tt.schema)
			for _, result := range tt.valid {
				_, err := tool.Run(context.Background(), json.RawMessage(`{"result":`+result+`}`))
				assert.NoError(t, err, result)
			}
			for _, result := range tt.invalid {
				_, err := tool.Run(context.Background(), json.RawMessage(`{"result":`+result+`}`))
				assert.ErrorIs(t, err, llm.ErrBadParameter, result)
			}
		}
	})

	t.Run("Invalid", func(t *testing.T) {
		for _, schema := range []string{``, `null`, `{"type":`, `{"type":5}`} {
			_, err := toolkit.NewRespond(json.RawMessage(schema))
			assert.ErrorIs(t, err, llm.ErrBadParameter, schema)
		}
	})
}
