package schema_test

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	// Packages
	schema "github.com/mutablelogic/go-llm/registry/schema"
	assert "github.com/stretchr/testify/assert"
)

// Test_ProviderTypes checks the provider's type, in the API's schema, is
// one of the provider types, and every one of them
func Test_ProviderTypes(t *testing.T) {
	field, ok := reflect.TypeFor[schema.Provider]().FieldByName("Type")
	if !assert.True(t, ok) {
		return
	}
	enum := strings.Split(field.Tag.Get("enum"), ",")
	types := slices.Clone(schema.ProviderTypes())
	slices.Sort(enum)
	slices.Sort(types)
	assert.Equal(t, types, enum)
}
