package registry

import (
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/mcp/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
	oauth2 "golang.org/x/oauth2"
)

func Test_CredentialMemory(t *testing.T) {
	// With the environment's provider, which can't keep credentials, how a
	// connector's server is signed in to is kept in memory
	ctx := t.Context()
	r, err := New()
	require.NoError(t, err)
	_, err = r.getCredential(ctx, "memory")
	assert.ErrorIs(t, err, llm.ErrNotFound)
	require.NoError(t, r.putCredential(ctx, "memory", schema.Credential{ClientID: "client", Token: &oauth2.Token{AccessToken: "access"}}))
	credential, err := r.getCredential(ctx, "memory")
	require.NoError(t, err)
	assert.Equal(t, "client", credential.ClientID)
	require.NotNil(t, credential.Token)
	assert.Equal(t, "access", credential.Token.AccessToken)

	// And deleted, once or twice
	require.NoError(t, r.deleteCredential(ctx, "memory"))
	require.NoError(t, r.deleteCredential(ctx, "memory"))
	_, err = r.getCredential(ctx, "memory")
	assert.ErrorIs(t, err, llm.ErrNotFound)
}
