package schema

import (
	"context"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// CredentialStore is the interface for credential storage backends.
type CredentialStore interface {
	// GetCredential retrieves the credential for the given server URL.
	GetCredential(ctx context.Context, url string) (*OAuthCredentials, error)

	// SetCredential stores (or updates) the credential for the given server URL.
	SetCredential(ctx context.Context, url string, cred OAuthCredentials) error

	// DeleteCredential removes the credential for the given server URL.
	DeleteCredential(ctx context.Context, url string) error
}
