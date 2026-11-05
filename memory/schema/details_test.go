package schema_test

import (
	"encoding/json"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/memory/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_ParseField(t *testing.T) {
	for _, name := range []string{"name", "assistant", "location", "timezone", "language", " Location "} {
		field, err := schema.ParseField(name)
		require.NoError(t, err, name)
		assert.True(t, field.Valid())
	}
	_, err := schema.ParseField("age")
	assert.ErrorIs(t, err, llm.ErrBadParameter)
	assert.ErrorContains(t, err, "name, assistant, location, timezone, language")
}

func Test_DetailValidate(t *testing.T) {
	for name, test := range map[string]struct {
		field  schema.Field
		detail schema.Detail
		want   schema.Detail
		err    bool
	}{
		"Value":           {schema.FieldName, schema.Detail{Value: " David "}, schema.Detail{Value: "David"}, false},
		"Declined":        {schema.FieldLocation, schema.Detail{Declined: true}, schema.Detail{Declined: true}, false},
		"Both":            {schema.FieldLocation, schema.Detail{Value: "Berlin", Declined: true}, schema.Detail{}, true},
		"Neither":         {schema.FieldLocation, schema.Detail{Value: "  "}, schema.Detail{}, true},
		"Timezone":        {schema.FieldTimezone, schema.Detail{Value: "Europe/Berlin"}, schema.Detail{Value: "Europe/Berlin"}, false},
		"UnknownTimezone": {schema.FieldTimezone, schema.Detail{Value: "Mars/Olympus"}, schema.Detail{}, true},
		"LocalTimezone":   {schema.FieldTimezone, schema.Detail{Value: "Local"}, schema.Detail{}, true},
		"UnknownField":    {schema.Field("age"), schema.Detail{Value: "42"}, schema.Detail{}, true},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := test.detail.Validate(test.field)
			if test.err {
				assert.ErrorIs(t, err, llm.ErrBadParameter)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}

func Test_Details(t *testing.T) {
	// Each field has three states: not known, a value, or declined
	var details schema.Details
	assert.Nil(t, details.Get(schema.FieldName))
	require.NoError(t, details.Set(schema.FieldName, &schema.Detail{Value: "David"}))
	require.NoError(t, details.Set(schema.FieldLocation, &schema.Detail{Declined: true}))
	require.NoError(t, details.Set(schema.FieldTimezone, &schema.Detail{Value: "Europe/Berlin"}))

	value, ok := details.Value(schema.FieldName)
	assert.True(t, ok)
	assert.Equal(t, "David", value)
	_, ok = details.Value(schema.FieldLocation)
	assert.False(t, ok, "declined")
	assert.True(t, details.Get(schema.FieldLocation).Declined)
	_, ok = details.Value(schema.FieldLanguage)
	assert.False(t, ok, "not known")
	assert.Equal(t, "Europe/Berlin", details.Zone().String())

	// A copy of a detail doesn't change it
	details.Get(schema.FieldName).Value = "Someone"
	value, _ = details.Value(schema.FieldName)
	assert.Equal(t, "David", value)

	// Nil forgets it, and an invalid one isn't set
	require.NoError(t, details.Set(schema.FieldName, nil))
	assert.Nil(t, details.Get(schema.FieldName))
	assert.ErrorIs(t, details.Set(schema.FieldTimezone, &schema.Detail{Value: "Nowhere"}), llm.ErrBadParameter)
	assert.Equal(t, "Europe/Berlin", details.Zone().String())

	// The details which aren't known aren't in the JSON
	data, err := json.Marshal(details)
	require.NoError(t, err)
	assert.JSONEq(t, `{"location":{"declined":true},"timezone":{"value":"Europe/Berlin"}}`, string(data))
	var decoded schema.Details
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, details, decoded)
	assert.NoError(t, decoded.Validate())
}
