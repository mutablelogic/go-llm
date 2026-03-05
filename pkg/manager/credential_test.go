package manager

import (
	"context"
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	store "github.com/mutablelogic/go-llm/pkg/store"
	assert "github.com/stretchr/testify/assert"
	oauth2 "golang.org/x/oauth2"
)

///////////////////////////////////////////////////////////////////////////////
// CREDENTIAL TESTS

// Test operations fail when no credential store is configured
func Test_credential_001(t *testing.T) {
	assert := assert.New(t)

	m, err := NewManager("test", "0.0.0")
	assert.NoError(err)

	_, err = m.GetCredential(context.TODO(), "https://example.com")
	assert.ErrorIs(err, llm.ErrNotImplemented)

	err = m.SetCredential(context.TODO(), "https://example.com", schema.OAuthCredentials{})
	assert.ErrorIs(err, llm.ErrNotImplemented)

	err = m.DeleteCredential(context.TODO(), "https://example.com")
	assert.ErrorIs(err, llm.ErrNotImplemented)
}

// Test WithCredentialStore rejects nil store
func Test_credential_002(t *testing.T) {
	assert := assert.New(t)

	_, err := NewManager("test", "0.0.0", WithCredentialStore(nil))
	assert.ErrorIs(err, llm.ErrBadParameter)
}

// Test SetCredential and GetCredential round-trip
func Test_credential_003(t *testing.T) {
	assert := assert.New(t)

	cs, err := store.NewMemoryCredentialStore("test-passphrase")
	assert.NoError(err)

	m, err := NewManager("test", "0.0.0", WithCredentialStore(cs))
	assert.NoError(err)

	cred := schema.OAuthCredentials{
		Token: &oauth2.Token{
			AccessToken:  "access-123",
			RefreshToken: "refresh-456",
			TokenType:    "Bearer",
		},
		ClientID: "client-abc",
		Endpoint: "https://example.com",
		TokenURL: "https://example.com/token",
	}

	err = m.SetCredential(context.TODO(), "https://example.com", cred)
	assert.NoError(err)

	got, err := m.GetCredential(context.TODO(), "https://example.com")
	assert.NoError(err)
	assert.Equal("access-123", got.AccessToken)
	assert.Equal("refresh-456", got.RefreshToken)
	assert.Equal("client-abc", got.ClientID)
	assert.Equal("https://example.com/token", got.TokenURL)
}

// Test GetCredential returns error for unknown URL
func Test_credential_004(t *testing.T) {
	assert := assert.New(t)

	cs, err := store.NewMemoryCredentialStore("test-passphrase")
	assert.NoError(err)

	m, err := NewManager("test", "0.0.0", WithCredentialStore(cs))
	assert.NoError(err)

	_, err = m.GetCredential(context.TODO(), "https://unknown.example.com")
	assert.Error(err)
}

// Test DeleteCredential removes a credential
func Test_credential_005(t *testing.T) {
	assert := assert.New(t)

	cs, err := store.NewMemoryCredentialStore("test-passphrase")
	assert.NoError(err)

	m, err := NewManager("test", "0.0.0", WithCredentialStore(cs))
	assert.NoError(err)

	cred := schema.OAuthCredentials{
		Token:    &oauth2.Token{AccessToken: "token-1"},
		ClientID: "client-1",
		Endpoint: "https://example.com",
		TokenURL: "https://example.com/token",
	}

	assert.NoError(m.SetCredential(context.TODO(), "https://example.com", cred))
	assert.NoError(m.DeleteCredential(context.TODO(), "https://example.com"))

	_, err = m.GetCredential(context.TODO(), "https://example.com")
	assert.Error(err)
}

// Test DeleteCredential returns error for unknown URL
func Test_credential_006(t *testing.T) {
	assert := assert.New(t)

	cs, err := store.NewMemoryCredentialStore("test-passphrase")
	assert.NoError(err)

	m, err := NewManager("test", "0.0.0", WithCredentialStore(cs))
	assert.NoError(err)

	err = m.DeleteCredential(context.TODO(), "https://unknown.example.com")
	assert.Error(err)
}

// Test invalid URL is rejected by all credential operations
func Test_credential_008(t *testing.T) {
	assert := assert.New(t)

	cs, err := store.NewMemoryCredentialStore("test-passphrase")
	assert.NoError(err)

	m, err := NewManager("test", "0.0.0", WithCredentialStore(cs))
	assert.NoError(err)

	cred := schema.OAuthCredentials{Token: &oauth2.Token{AccessToken: "tok"}}

	// Missing scheme
	assert.Error(m.SetCredential(context.TODO(), "example.com", cred))
	_, err = m.GetCredential(context.TODO(), "example.com")
	assert.Error(err)
	assert.Error(m.DeleteCredential(context.TODO(), "example.com"))

	// Unsupported scheme
	assert.Error(m.SetCredential(context.TODO(), "ftp://example.com", cred))
}

// Test URL canonicalisation: set with non-canonical URL, retrieve with canonical
func Test_credential_009(t *testing.T) {
	assert := assert.New(t)

	cs, err := store.NewMemoryCredentialStore("test-passphrase")
	assert.NoError(err)

	m, err := NewManager("test", "0.0.0", WithCredentialStore(cs))
	assert.NoError(err)

	cred := schema.OAuthCredentials{
		Token:    &oauth2.Token{AccessToken: "canon-tok"},
		ClientID: "c",
		Endpoint: "https://example.com",
		TokenURL: "https://example.com/token",
	}

	// Store with uppercase scheme+host and a spurious query string.
	assert.NoError(m.SetCredential(context.TODO(), "HTTPS://Example.COM?x=1", cred))

	// Retrieve with canonical URL.
	got, err := m.GetCredential(context.TODO(), "https://example.com")
	assert.NoError(err)
	assert.Equal("canon-tok", got.AccessToken)

	// Delete with non-canonical URL must succeed.
	assert.NoError(m.DeleteCredential(context.TODO(), "HTTPS://Example.COM?x=1"))
	_, err = m.GetCredential(context.TODO(), "https://example.com")
	assert.Error(err)
}

// Test SetCredential overwrites an existing credential
func Test_credential_007(t *testing.T) {
	assert := assert.New(t)

	cs, err := store.NewMemoryCredentialStore("test-passphrase")
	assert.NoError(err)

	m, err := NewManager("test", "0.0.0", WithCredentialStore(cs))
	assert.NoError(err)

	cred1 := schema.OAuthCredentials{
		Token:    &oauth2.Token{AccessToken: "old-token"},
		ClientID: "old-client",
		Endpoint: "https://example.com",
		TokenURL: "https://example.com/token",
	}
	cred2 := schema.OAuthCredentials{
		Token:    &oauth2.Token{AccessToken: "new-token"},
		ClientID: "new-client",
		Endpoint: "https://example.com",
		TokenURL: "https://example.com/token",
	}

	assert.NoError(m.SetCredential(context.TODO(), "https://example.com", cred1))
	assert.NoError(m.SetCredential(context.TODO(), "https://example.com", cred2))

	got, err := m.GetCredential(context.TODO(), "https://example.com")
	assert.NoError(err)
	assert.Equal("new-token", got.AccessToken)
	assert.Equal("new-client", got.ClientID)
}
