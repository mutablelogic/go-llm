package store_test

import (
	"context"
	"testing"
	"time"

	// Packages
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	assert "github.com/stretchr/testify/assert"
	oauth2 "golang.org/x/oauth2"
)

// credentialStoreTests defines shared behavioural tests for any
// CredentialStore implementation.
var credentialStoreTests = []struct {
	Name string
	Fn   func(t *testing.T, s schema.CredentialStore)
}{{
	Name: "GetNotFound",
	Fn: func(t *testing.T, s schema.CredentialStore) {
		assert := assert.New(t)
		_, err := s.GetCredential(context.Background(), "https://example.com")
		assert.Error(err)
	},
}, {
	Name: "SetAndGet",
	Fn: func(t *testing.T, s schema.CredentialStore) {
		assert := assert.New(t)
		ctx := context.Background()

		cred := schema.OAuthCredentials{
			Token: &oauth2.Token{
				AccessToken:  "access-123",
				RefreshToken: "refresh-456",
				TokenType:    "Bearer",
				Expiry:       time.Now().Add(time.Hour).Truncate(time.Second),
			},
			ClientID: "client-abc",
			Endpoint: "https://example.com",
			TokenURL: "https://example.com/token",
		}

		err := s.SetCredential(ctx, "https://example.com", cred)
		assert.NoError(err)

		got, err := s.GetCredential(ctx, "https://example.com")
		assert.NoError(err)
		assert.Equal("access-123", got.AccessToken)
		assert.Equal("refresh-456", got.RefreshToken)
		assert.Equal("client-abc", got.ClientID)
		assert.Equal("https://example.com/token", got.TokenURL)
	},
}, {
	Name: "Delete",
	Fn: func(t *testing.T, s schema.CredentialStore) {
		assert := assert.New(t)
		ctx := context.Background()

		cred := schema.OAuthCredentials{
			Token: &oauth2.Token{
				AccessToken: "token-1",
			},
			ClientID: "client-1",
			Endpoint: "https://example.com",
			TokenURL: "https://example.com/token",
		}

		assert.NoError(s.SetCredential(ctx, "https://example.com", cred))
		assert.NoError(s.DeleteCredential(ctx, "https://example.com"))

		// Get after delete returns error
		_, err := s.GetCredential(ctx, "https://example.com")
		assert.Error(err)
	},
}, {
	Name: "DeleteNotFound",
	Fn: func(t *testing.T, s schema.CredentialStore) {
		assert := assert.New(t)
		err := s.DeleteCredential(context.Background(), "https://example.com")
		assert.Error(err)
	},
}, {
	Name: "SetOverwrites",
	Fn: func(t *testing.T, s schema.CredentialStore) {
		assert := assert.New(t)
		ctx := context.Background()

		cred1 := schema.OAuthCredentials{
			Token:    &oauth2.Token{AccessToken: "old"},
			ClientID: "c1",
			Endpoint: "https://example.com",
			TokenURL: "https://example.com/token",
		}
		cred2 := schema.OAuthCredentials{
			Token:    &oauth2.Token{AccessToken: "new"},
			ClientID: "c2",
			Endpoint: "https://example.com",
			TokenURL: "https://example.com/token",
		}

		assert.NoError(s.SetCredential(ctx, "https://example.com", cred1))
		assert.NoError(s.SetCredential(ctx, "https://example.com", cred2))

		got, err := s.GetCredential(ctx, "https://example.com")
		assert.NoError(err)
		assert.Equal("new", got.AccessToken)
		assert.Equal("c2", got.ClientID)
	},
}, {
	Name: "InvalidURL",
	Fn: func(t *testing.T, s schema.CredentialStore) {
		assert := assert.New(t)
		ctx := context.Background()
		cred := schema.OAuthCredentials{Token: &oauth2.Token{AccessToken: "tok"}}
		// Missing scheme
		assert.Error(s.SetCredential(ctx, "example.com", cred))
		_, err := s.GetCredential(ctx, "example.com")
		assert.Error(err)
		assert.Error(s.DeleteCredential(ctx, "example.com"))
		// Unsupported scheme
		assert.Error(s.SetCredential(ctx, "ftp://example.com", cred))
		// Empty string
		assert.Error(s.SetCredential(ctx, "", cred))
	},
}, {
	Name: "URLCanonicalised",
	Fn: func(t *testing.T, s schema.CredentialStore) {
		assert := assert.New(t)
		ctx := context.Background()
		cred := schema.OAuthCredentials{
			Token:    &oauth2.Token{AccessToken: "tok-canon"},
			ClientID: "c",
			Endpoint: "https://example.com",
			TokenURL: "https://example.com/token",
		}
		// Store with non-canonical URL (uppercase scheme + host, spurious query).
		assert.NoError(s.SetCredential(ctx, "HTTPS://Example.COM?x=1", cred))
		// Retrieve with the canonical form.
		got, err := s.GetCredential(ctx, "https://example.com")
		assert.NoError(err)
		assert.Equal("tok-canon", got.AccessToken)
		// Retrieve with the original non-canonical URL must also work.
		got2, err := s.GetCredential(ctx, "HTTPS://Example.COM?x=1")
		assert.NoError(err)
		assert.Equal("tok-canon", got2.AccessToken)
		// Delete with non-canonical URL.
		assert.NoError(s.DeleteCredential(ctx, "HTTPS://Example.COM?x=1"))
		_, err = s.GetCredential(ctx, "https://example.com")
		assert.Error(err)
	},
}}

// runCredentialStoreTests runs every shared behavioural test against a
// credential store implementation. The factory is called once per subtest
// so each gets a clean, independent store.
func runCredentialStoreTests(t *testing.T, factory func() schema.CredentialStore) {
	t.Helper()
	for _, tt := range credentialStoreTests {
		t.Run(tt.Name, func(t *testing.T) {
			tt.Fn(t, factory())
		})
	}
}
