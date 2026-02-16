package store

import (
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	types "github.com/mutablelogic/go-server/pkg/types"
)

// validateLabels checks that all label keys are valid identifiers.
func validateLabels(labels map[string]string) error {
	for k := range labels {
		if !types.IsIdentifier(k) {
			return llm.ErrBadParameter.Withf("invalid label key: %q", k)
		}
	}
	return nil
}

// matchLabels returns true if the session's labels contain all of the
// required filter labels. An empty filter matches everything.
// Each filter entry is a "key:value" string.
func matchLabels(sessionLabels map[string]string, filter []string) bool {
	for _, f := range filter {
		k, v, ok := strings.Cut(f, ":")
		if !ok {
			continue
		}
		if sessionLabels[k] != v {
			return false
		}
	}
	return true
}
