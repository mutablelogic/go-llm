// Package credentials expands credentials, such as a provider's API key, a
// tool's, or a connector's client's secret, whose placeholders, such as
// ${OPENAI_API_KEY} or $OPENAI_API_KEY, are replaced with the credentials
// they name, so they're never stored. One provider is shared by the
// registry, the toolkit and the connectors, which the server makes.
package credentials

import (
	"context"
	"errors"
	"os"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// INTERFACES

// Provider expands credentials, whose placeholders, such as ${OPENAI_KEY}
// or $OPENAI_KEY, are replaced with the credentials they name, such as from
// the environment, or a secrets manager
type Provider interface {
	// Expand returns the value with each placeholder replaced with its
	// credential, or an error when a credential doesn't exist, which is
	// ErrNotFound
	Expand(ctx context.Context, value string) (string, error)
}

///////////////////////////////////////////////////////////////////////////////
// TYPES

// env is the provider whose credentials are environment variables
type env struct{}

var _ Provider = env{}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// Env returns the provider whose credentials are environment variables,
// which is each package's, unless it's given another
func Env() Provider {
	return env{}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Expand replaces each placeholder with the environment variable it names,
// which must be set, and returns ErrNotFound, with each which isn't
func (env) Expand(_ context.Context, value string) (string, error) {
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
