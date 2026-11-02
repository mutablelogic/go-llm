// Package test has a registry and a HTTP server of its API, for tests
package test

import (
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	registry "github.com/mutablelogic/go-llm/registry"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// NewRegistry returns a registry, with the options, and the providers, each
// by its own name, such as its type, or fails the test. As the registry
// isn't stored, each test has a registry of its own.
func NewRegistry(t *testing.T, providers []llm.Provider, opts ...registry.Opt) *registry.Registry {
	t.Helper()
	r, err := registry.New(opts...)
	if err != nil {
		t.Fatal(err)
	}
	for _, provider := range providers {
		if err := r.Add("", provider); err != nil {
			t.Fatal(err)
		}
	}
	return r
}
