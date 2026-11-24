package schema_test

import (
	"strings"
	"testing"

	// Packages
	schema "github.com/mutablelogic/go-llm/webhook/schema"
	assert "github.com/stretchr/testify/assert"
)

func Test_HideToken(t *testing.T) {
	const token = "AAAAAAAAAAAAAAAAAAAAA_BBBBBBBBBBBBBBBB-CCCC"
	for text, expected := range map[string]string{
		"Reading https://example.com/api/hook/" + token:           "Reading https://example.com/api/hook/{token}",
		"https://example.com/api/hook/" + token + "?a=1 and more": "https://example.com/api/hook/{token}?a=1 and more",
		"hook%2F" + token + " hook%252f" + token:                  "hook%2F{token} hook%252f{token}",
		`hook\/` + token + ` hook\` + "u002f" + token:             `hook\/{token} hook\` + "u002f{token}",
		"hook/%41" + token[1:] + " hook/%2541" + token[1:]:        "hook/{token} hook/{token}",
		"hook/" + strings.Repeat("%41", 43):                       "hook/{token}",
		"%68o%6Fk/" + token:                                       "%68o%6Fk/{token}",
		"hook/\\" + "u0041" + token[1:]:                           "hook/{token}",
		"hook/%41" + token[2:]:                                    "hook/%41" + token[2:],
		"hook/" + token + "longer/page":                           "hook/{token}/page",
		"https://example.com/hook/short":                          "https://example.com/hook/short",
		"https://example.com/page":                                "https://example.com/page",
		"":                                                        "",
	} {
		assert.Equal(t, expected, schema.HideToken(text))
	}
}
