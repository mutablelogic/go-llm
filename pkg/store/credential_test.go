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
	Name: "MultipleURLs",
	Fn: func(t *testing.T, s schema.CredentialStore) {
		assert := assert.New(t)
		ctx := context.Background()

		cred1 := schema.OAuthCredentials{
			Token:    &oauth2.Token{AccessToken: "token-a"},
			ClientID: "client-a",
			Endpoint: "https://a.example.com",
			TokenURL: "https://a.example.com/token",
		}
		cred2 := schema.OAuthCredentials{
			Token:    &oauth2.Token{AccessToken: "token-b"},
			ClientID: "client-b",
			Endpoint: "https://b.example.com",
			TokenURL: "https://b.example.com/token",
		}

		assert.NoError(s.SetCredential(ctx, "https://a.example.com", cred1))
		assert.NoError(s.SetCredential(ctx, "https://b.example.com", cred2))

		got1, err := s.GetCredential(ctx, "https://a.example.com")
		assert.NoError(err)
		assert.Equal("token-a", got1.AccessToken)

		got2, err := s.GetCredential(ctx, "https://b.example.com")
		assert.NoError(err)
		assert.Equal("token-b", got2.AccessToken)
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
