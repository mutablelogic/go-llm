package registry

import (
	"context"
	"encoding/json"
	"errors"

	// Packages
	goauth "github.com/mutablelogic/go-auth"
	credential "github.com/mutablelogic/go-auth/credential"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/mcp/schema"
)

///////////////////////////////////////////////////////////////////////////////
// INTERFACES

// CredentialsProvider expands credentials, such as a connector's client's
// secret, whose placeholders, such as ${GITHUB_CLIENT_SECRET}, are replaced
// with the credentials they name, and keeps how each connector's server is
// signed in to, under its CredentialKey. It's go-auth's
// CredentialProvider, so one provider is shared by the registry, the toolkit and the
// connectors.
type CredentialsProvider = credential.CredentialProvider

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// clientCredentials returns the identifier and secret of the connector's
// own client, from the credentials provider, each time they're used, so
// they aren't kept, or none, when it has none, or ErrBadParameter, when a
// credential doesn't exist, so the connector isn't valid, as a provider's
// API key isn't
func (r *Registry) clientCredentials(ctx context.Context, connector schema.Connector) (string, string, error) {
	id, err := r.credential(ctx, "client id", connector.ClientIDCredential)
	if err != nil {
		return "", "", err
	}
	secret, err := r.credential(ctx, "client secret", connector.ClientSecretCredential)
	if err != nil {
		return "", "", err
	}
	return id, secret, nil
}

// credential returns the credential with the name, from the credentials
// provider, or none, when there's no name
func (r *Registry) credential(ctx context.Context, what, name string) (string, error) {
	if name == "" {
		return "", nil
	}
	value, err := r.credentialsProvider.Expand(ctx, "${"+name+"}")
	if errors.Is(err, goauth.ErrNotFound) {
		return "", llm.ErrBadParameter.Withf("%s: %v", what, err)
	}
	return value, err
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC FUNCTIONS

// CredentialKey returns the key which how the connector with the name is
// signed in to, such as its token, is kept under, with the credentials
// provider, as JSON. It's an identifier, as go-auth's keys are, of at most
// 64 characters, as a connector's name is at most 60.
func CredentialKey(name string) string {
	return credentialPrefix + name
}

// credentialPrefix is the prefix of the keys of connectors' credentials
const credentialPrefix = "mcp_"

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS - SIGN-IN

// getCredential returns how the connector's server is signed in to, from
// the credentials provider, or from memory, when the provider can't keep
// it, or ErrNotFound, when it has none
func (r *Registry) getCredential(ctx context.Context, name string) (*schema.Credential, error) {
	value, err := r.credentialsProvider.Get(ctx, CredentialKey(name))
	if errors.Is(err, goauth.ErrNotFound) {
		return r.credentials.get(name)
	} else if err != nil {
		return nil, err
	}
	var credential schema.Credential
	if err := json.Unmarshal(value, &credential); err != nil {
		return nil, llm.ErrInternalServerError.Withf("%s: %v", CredentialKey(name), err)
	}
	return &credential, nil
}

// putCredential sets how the connector's server is signed in to, with the
// credentials provider, or in memory, when the provider can't keep it
func (r *Registry) putCredential(ctx context.Context, name string, credential schema.Credential) error {
	data, err := json.Marshal(credential)
	if err != nil {
		return err
	}
	if err := r.credentialsProvider.Set(ctx, CredentialKey(name), data); errors.Is(err, goauth.ErrNotImplemented) {
		r.credentials.put(name, credential)
	} else if err != nil {
		return err
	}
	return nil
}

// deleteCredential deletes how the connector's server is signed in to,
// from the credentials provider, and from memory, or does nothing, when
// it has none
func (r *Registry) deleteCredential(ctx context.Context, name string) error {
	r.credentials.delete(name)
	if err := r.credentialsProvider.Delete(ctx, CredentialKey(name)); err != nil && !errors.Is(err, goauth.ErrNotFound) && !errors.Is(err, goauth.ErrNotImplemented) {
		return err
	}
	return nil
}
