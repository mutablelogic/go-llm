package schema_test

import (
	"testing"

	// Packages
	schema "github.com/mutablelogic/go-llm/provider/schema"
	assert "github.com/stretchr/testify/assert"
)

func Test_LanguageCode(t *testing.T) {
	// Whisper's names are codes, in any case, and other languages are
	// themselves, in lower case
	for language, want := range map[string]string{
		"english":        "en",
		" French ":       "fr",
		"haitian creole": "ht",
		"EN":             "en",
		"haw":            "haw",
		"":               "",
	} {
		assert.Equal(t, want, schema.LanguageCode(language), language)
	}
}
