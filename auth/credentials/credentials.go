// Package credentials has the provider of credentials, such as a
// provider's API key, a tool's, or a connector's client's secret, whose
// placeholders, such as ${OPENAI_API_KEY} or $OPENAI_API_KEY, are replaced
// with the credentials they name, so they're never stored. One provider is
// shared by the registry, the toolkit and the connectors, which the server
// makes, such as env's, whose credentials are environment variables.
package credentials

import (
	"context"
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
