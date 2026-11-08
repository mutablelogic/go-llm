package registry

import (
	"context"
	"errors"
	"os"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/mcp/schema"
)

///////////////////////////////////////////////////////////////////////////////
// INTERFACES

// CredentialsProvider expands credentials, such as a connector's client
// secret, whose placeholders, such as ${GITHUB_CLIENT_SECRET}, are replaced
// with the credentials they name, as a provider's API key's are
type CredentialsProvider interface {
	// Expand returns the value with each placeholder replaced with its
	// credential, or an error when a credential doesn't exist
	Expand(ctx context.Context, value string) (string, error)
}

///////////////////////////////////////////////////////////////////////////////
// TYPES

// envCredentials is the credentials provider when there's none, whose
// credentials are environment variables
type envCredentials struct{}

var _ CredentialsProvider = envCredentials{}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Expand replaces each placeholder with the environment variable it names,
// which must be set
func (envCredentials) Expand(_ context.Context, value string) (string, error) {
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

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// clientCredentials returns the identifier and secret of the connector's own client,
// with their credentials expanded, or ErrBadParameter, when a credential
// doesn't exist, so the connector isn't valid, as a provider's API key isn't
func (r *Registry) clientCredentials(ctx context.Context, connector schema.Connector) (string, string, error) {
	id, err := r.credentialsProvider.Expand(ctx, connector.ClientID)
	if errors.Is(err, llm.ErrNotFound) {
		return "", "", llm.ErrBadParameter.Withf("client id: %v", err)
	} else if err != nil {
		return "", "", err
	}
	secret, err := r.credentialsProvider.Expand(ctx, connector.ClientSecret)
	if errors.Is(err, llm.ErrNotFound) {
		return "", "", llm.ErrBadParameter.Withf("client secret: %v", err)
	} else if err != nil {
		return "", "", err
	}
	return id, secret, nil
}
