package store_test

import (
	"testing"

	// Packages
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	store "github.com/mutablelogic/go-llm/pkg/store"
	assert "github.com/stretchr/testify/assert"
)

func Test_memory_credential_001(t *testing.T) {
	assert := assert.New(t)
	s, err := store.NewMemoryCredentialStore("test-passphrase")
	assert.NoError(err)
	assert.NotNil(s)

	// Empty passphrase rejected
	_, err = store.NewMemoryCredentialStore("")
	assert.Error(err)

	// Too short passphrase rejected
	_, err = store.NewMemoryCredentialStore("short")
	assert.Error(err)

	// Whitespace-only passphrase rejected
	_, err = store.NewMemoryCredentialStore("       ")
	assert.Error(err)
}

func Test_memory_credential_002(t *testing.T) {
	runCredentialStoreTests(t, func() schema.CredentialStore {
		s, _ := store.NewMemoryCredentialStore("test-passphrase")
		return s
	})
}
