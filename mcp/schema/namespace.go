package schema

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"unicode"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// AgentNamespace is the namespace of the registry's agents, which a
	// connector's namespace can't be, as the registry's AgentNamespace is
	AgentNamespace = "agent"

	// DefaultNamespace is a connector's namespace, when its name has nothing
	// which can be one
	DefaultNamespace = "mcp"

	// maxNamespace is the longest namespace, as it's an identifier
	maxNamespace = 64

	// namespaceDigits is how many random digits are added to a namespace
	// which is taken
	namespaceDigits = 4
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC FUNCTIONS

// Namespace returns the namespace of the tools of the connector with the
// name, in a toolkit: the name, without the characters an identifier can't
// have, starting with a letter, without two underscores together, or one at
// its end, which the toolkit's namespaces can't have, or DefaultNamespace,
// when nothing's left of it. When it's AgentNamespace, or taken, such as by
// a namespace in the toolkit, random digits are added to it, until it
// isn't. The function which says whether a namespace is taken can be nil.
func Namespace(name string, taken func(string) bool) string {
	base := sanitize(name)
	if base == "" {
		base = DefaultNamespace
	}
	clashes := func(namespace string) bool {
		return strings.EqualFold(namespace, AgentNamespace) || (taken != nil && taken(namespace))
	}
	if !clashes(base) {
		return base
	}

	// Add random digits, keeping it as long as a namespace can be
	if len(base) > maxNamespace-namespaceDigits {
		base = strings.TrimRight(base[:maxNamespace-namespaceDigits], "_-")
	}
	for {
		namespace := fmt.Sprintf("%s%0*d", base, namespaceDigits, rand.N(pow10(namespaceDigits)))
		if !clashes(namespace) {
			return namespace
		}
	}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// sanitize returns the name, with only the characters an identifier can
// have, which are ASCII letters, numbers, underscores and hyphens, starting
// with a letter, without two underscores together, or one at its end, and
// no longer than a namespace can be
func sanitize(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r > unicode.MaxASCII:
			continue
		case b.Len() == 0 && !unicode.IsLetter(r):
			// It starts with a letter
			continue
		case unicode.IsLetter(r), unicode.IsDigit(r), r == '-':
			b.WriteRune(r)
		case r == '_':
			// Not two together
			if !strings.HasSuffix(b.String(), "_") {
				b.WriteRune(r)
			}
		}
	}
	result := b.String()
	if len(result) > maxNamespace {
		result = result[:maxNamespace]
	}
	return strings.TrimRight(result, "_")
}

// pow10 returns ten to the power of n
func pow10(n int) int {
	result := 1
	for range n {
		result *= 10
	}
	return result
}
