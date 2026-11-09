// Package env is the provider of credentials whose credentials are
// environment variables, which is the registry's, the toolkit's and the
// connectors', unless they're given another. It can't keep credentials.
package env

import (
	"context"
	"os"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	credentials "github.com/mutablelogic/go-llm/auth/credentials"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Env is the provider of credentials whose credentials are environment
// variables
type Env struct{}

var _ credentials.Provider = Env{}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns the provider of credentials whose credentials are
// environment variables
func New() credentials.Provider {
	return Env{}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Get returns the environment variable with the key, or ErrNotFound when
// it isn't set
func (Env) Get(_ context.Context, key string) (string, error) {
	if value, ok := os.LookupEnv(key); ok {
		return value, nil
	}
	return "", llm.ErrNotFound.Withf("credential %q isn't set", key)
}

// Set returns ErrNotImplemented, as environment variables aren't kept
func (Env) Set(context.Context, string, string) error {
	return llm.ErrNotImplemented.With("credentials can't be set in the environment")
}

// Delete returns ErrNotImplemented, as environment variables aren't kept
func (Env) Delete(context.Context, string) error {
	return llm.ErrNotImplemented.With("credentials can't be deleted from the environment")
}

// Expand replaces each placeholder with the environment variable it names,
// which must be set, and returns ErrNotFound, with each which isn't
func (e Env) Expand(ctx context.Context, value string) (string, error) {
	return credentials.Expand(ctx, e.Get, value)
}
