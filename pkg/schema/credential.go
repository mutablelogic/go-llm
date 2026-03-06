package schema

import (
	"context"

	// Packages
	oauth2 "golang.org/x/oauth2"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// OAuthCredentials bundles an OAuth token with the metadata needed to
// refresh or reuse it later without re-discovering or re-registering.
type OAuthCredentials struct {
	*oauth2.Token

	// ClientID is the OAuth client ID used to obtain this token.
	ClientID string `json:"client_id"`

	// ClientSecret is the OAuth client secret, if any (for confidential clients).
	// Needed for token refresh with servers that require client authentication.
	ClientSecret string `json:"client_secret,omitempty"`

	// Endpoint is the MCP/OAuth server endpoint (used for discovery).
	Endpoint string `json:"endpoint"`

	// TokenURL is the OAuth token endpoint URL (used for refresh without re-discovery).
	TokenURL string `json:"token_url"`
}

// CredentialStore is the interface for credential storage backends.
type CredentialStore interface {
	// GetCredential retrieves the credential for the given server URL.
	GetCredential(ctx context.Context, url string) (*OAuthCredentials, error)

	// SetCredential stores (or updates) the credential for the given server URL.
	SetCredential(ctx context.Context, url string, cred OAuthCredentials) error

	// DeleteCredential removes the credential for the given server URL.
	DeleteCredential(ctx context.Context, url string) error
}
