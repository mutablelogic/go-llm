package schema

import (
	"fmt"
	"time"

	// Packages
	uuid "github.com/google/uuid"
	pg "github.com/mutablelogic/go-pg"
	types "github.com/mutablelogic/go-server/pkg/types"
	oauth2 "golang.org/x/oauth2"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type CredentialKey struct {
	URL  string     `json:"url" help:"Credential URL"`
	User *uuid.UUID `json:"user,omitempty" help:"Credential owner" optional:""`
}

// Credential is the public credential row returned from the database.
// Secret material and passphrase version are intentionally excluded.
type Credential struct {
	CredentialKey
	CreatedAt time.Time `json:"created_at" help:"Creation timestamp" readonly:""`
}

// CredentialInsert contains the values required to insert a credential row.
// The returned Credential omits PV and Credentials.
type CredentialInsert struct {
	CredentialKey
	Credentials []byte `json:"credentials" help:"Encrypted credential payload"`
}

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

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (c Credential) String() string {
	return types.Stringify(c)
}

func (c CredentialInsert) String() string {
	return types.Stringify(c)
}

func (c CredentialInsert) RedactedString() string {
	r := c
	r.Credentials = nil
	return types.Stringify(r)
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - READER

// Expected column order: url, user, created_at.
func (c *Credential) Scan(row pg.Row) error {
	if err := row.Scan(&c.URL, &c.User, &c.CreatedAt); err != nil {
		return err
	}
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - WRITER

func (c CredentialInsert) Insert(bind *pg.Bind) (string, error) {
	url, err := CanonicalURL(c.URL)
	if err != nil {
		return "", err
	}
	bind.Set("url", url)

	if c.User == nil || *c.User == uuid.Nil {
		bind.Set("user", nil)
	} else {
		bind.Set("user", *c.User)
	}

	if c.Credentials == nil {
		return "", ErrBadParameter.With("credential credentials are required")
	}
	if !bind.Has("pv") {
		return "", ErrInternalServerError.With("credential insert requires passphrase version binding")
	}
	bind.Set("credentials", c.Credentials)

	return bind.Query("credential.insert"), nil
}

func (c CredentialInsert) Update(_ *pg.Bind) error {
	return fmt.Errorf("CredentialInsert: update: not supported")
}
