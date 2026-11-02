package ollama

import (
	"testing"

	// Packages
	assert "github.com/stretchr/testify/assert"
)

func TestModelName(t *testing.T) {
	for name, want := range map[string]string{
		"gemma":                               "gemma:latest",
		"gemma:2b":                            "gemma:2b",
		"llama3.2:latest":                     "llama3.2:latest",
		" gemma ":                             "gemma:latest",
		"x/flux2-klein":                       "x/flux2-klein:latest",
		"x/flux2-klein:q4":                    "x/flux2-klein:q4",
		"registry.example:5000/team/model":    "registry.example:5000/team/model:latest",
		"registry.example:5000/team/model:v1": "registry.example:5000/team/model:v1",
		"":                                    "",
	} {
		assert.Equal(t, want, modelName(name), "name %q", name)
	}
}
