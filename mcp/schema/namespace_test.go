package schema_test

import (
	"regexp"
	"strings"
	"testing"

	// Packages
	schema "github.com/mutablelogic/go-llm/mcp/schema"
	llmregistry "github.com/mutablelogic/go-llm/registry"
	toolschema "github.com/mutablelogic/go-llm/toolkit/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
)

// valid returns true when the namespace can be a toolkit's
func valid(namespace string) bool {
	return types.IsIdentifier(namespace) && toolschema.IsModelToolPart(namespace)
}

func Test_Namespace(t *testing.T) {
	t.Run("Sanitized", func(t *testing.T) {
		// A name loses what an identifier, and a toolkit's namespace, can't
		// have, and is the default, when nothing's left of it
		for name, want := range map[string]string{
			"seolinkmap":            "seolinkmap",
			"GitHub":                "GitHub",
			"my-server":             "my-server",
			"my server!":            "myserver",
			"9lives":                "lives",
			"_private":              "private",
			"a__b___c":              "a_b_c",
			"trailing_":             "trailing",
			"café":                  "caf",
			"":                      schema.DefaultNamespace,
			"123":                   schema.DefaultNamespace,
			"!!!":                   schema.DefaultNamespace,
			strings.Repeat("a", 70): strings.Repeat("a", 64),
		} {
			got := schema.Namespace(name, nil)
			assert.Equal(t, want, got, name)
			assert.True(t, valid(got), got)
		}
	})

	t.Run("Agent", func(t *testing.T) {
		// The agents' namespace, which is the registry's, whatever its case,
		// has digits added
		assert.Equal(t, llmregistry.AgentNamespace, schema.AgentNamespace)
		re := regexp.MustCompile(`^agent\d{4}$`)
		for _, name := range []string{"agent", "Agent", "AGENT"} {
			got := schema.Namespace(name, nil)
			assert.Regexp(t, regexp.MustCompile(`(?i)`+re.String()), got, name)
			assert.True(t, valid(got), got)
		}
	})

	t.Run("Taken", func(t *testing.T) {
		// One which is taken has digits added, until it isn't
		taken := map[string]bool{"memory": true}
		got := schema.Namespace("memory", func(ns string) bool { return taken[ns] })
		assert.Regexp(t, `^memory\d{4}$`, got)

		// Even when most are, so more than one is tried
		calls := 0
		got = schema.Namespace("memory", func(ns string) bool {
			calls++
			return calls < 5
		})
		assert.Regexp(t, `^memory\d{4}$`, got)
		assert.Equal(t, 5, calls)

		// And one as long as it can be is shortened, so it still is
		long := strings.Repeat("a", 64)
		got = schema.Namespace(long, func(ns string) bool { return ns == long })
		assert.Len(t, got, 64)
		assert.Regexp(t, `^a{60}\d{4}$`, got)
		assert.True(t, valid(got))
	})
}
