package toolkit

import (
	"context"
	"errors"
	"os"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// INTERFACES

// CredentialsProvider expands credentials, such as a tool's API key, whose
// placeholders, such as ${WEATHERAPI_KEY} or $WEATHERAPI_KEY, are replaced
// with the credentials they name, as the registry's credentials provider
// does, so one provider can be both's
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
