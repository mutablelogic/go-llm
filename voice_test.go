package llm_test

import (
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	assert "github.com/stretchr/testify/assert"
)

func Test_FindVoices(t *testing.T) {
	voices := []llm.Voice{{ID: "XrExE9yKIg1WjnnlVkGX", Title: "Matilda - Knowledgable, Professional"}, {ID: "alloy", Title: "Alloy"}, {ID: "Alfie", Title: "Alfie"}}
	ids := func(voices []llm.Voice) []string {
		var result []string
		for _, voice := range voices {
			result = append(result, voice.ID)
		}
		return result
	}
	assert.Equal(t, []string{"XrExE9yKIg1WjnnlVkGX"}, ids(llm.FindVoices(voices, " MATILDA ")), "by title, ignoring case")
	assert.Equal(t, []string{"XrExE9yKIg1WjnnlVkGX"}, ids(llm.FindVoices(voices, "xrexe9")), "by id")
	assert.Equal(t, []string{"XrExE9yKIg1WjnnlVkGX", "alloy", "Alfie"}, ids(llm.FindVoices(voices, "al")), "in order, as Professional has al")
	assert.Len(t, llm.FindVoices(voices, ""), 3, "all of them")
	assert.Empty(t, llm.FindVoices(voices, "nobody"))
}
