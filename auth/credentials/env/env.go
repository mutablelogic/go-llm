// Package env is the provider of credentials whose credentials are
// environment variables, which is the registry's, the toolkit's and the
// connectors', unless they're given another
package env

import (
	"context"
	"errors"
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

// Expand replaces each placeholder with the environment variable it names,
// which must be set, and returns ErrNotFound, with each which isn't
func (Env) Expand(_ context.Context, value string) (string, error) {
	var err error
	result := os.Expand(value, func(key string) string {
		v, ok := os.LookupEnv(key)
		if !ok {
			err = errors.Join(err, llm.ErrNotFound.Withf("credential %q isn't set", key))
		}
		return v
	})
	if err != nil {
		return "", err
	}
	return result, nil
}
