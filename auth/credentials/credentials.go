// Package credentials has the provider of credentials, such as a
// provider's API key, a tool's, a connector's client's secret, or how a
// connector's server is signed in to. Placeholders, such as
// ${OPENAI_API_KEY} or $OPENAI_API_KEY, are replaced with the credentials
// they name, so they're never stored with what uses them. One provider is
// shared by the registry, the toolkit and the connectors, which the server
// makes: env's, whose credentials are environment variables, or file's,
// which also keeps credentials in a file.
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

// Provider gets, sets and deletes credentials by their keys, such as from
// the environment, a file, or a secrets manager, and expands placeholders
type Provider interface {
	// Get returns the credential with the key, or ErrNotFound when there's
	// none
	Get(ctx context.Context, key string) (string, error)

	// Set sets the credential with the key, or returns ErrNotImplemented,
	// when the provider can't keep credentials
	Set(ctx context.Context, key, value string) error

	// Delete deletes the credential with the key, or does nothing when
	// there's none, or returns ErrNotImplemented, when the provider can't
	// keep credentials
	Delete(ctx context.Context, key string) error

	// Expand returns the value with each placeholder replaced with its
	// credential, or an error when a credential doesn't exist, which is
	// ErrNotFound
	Expand(ctx context.Context, value string) (string, error)
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC FUNCTIONS

// Expand returns the value with each placeholder, such as ${KEY} or $KEY,
// replaced with the credential which get returns, or an error, which is
// ErrNotFound with each credential which doesn't exist, so a provider's
// Expand can be its Get
func Expand(ctx context.Context, get func(context.Context, string) (string, error), value string) (string, error) {
	var result error
	expanded := os.Expand(value, func(key string) string {
		v, err := get(ctx, key)
		if errors.Is(err, llm.ErrNotFound) {
			result = errors.Join(result, llm.ErrNotFound.Withf("credential %q isn't set", key))
		} else if err != nil {
			result = errors.Join(result, err)
		}
		return v
	})
	if result != nil {
		return "", result
	}
	return expanded, nil
}
