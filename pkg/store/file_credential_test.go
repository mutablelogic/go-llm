package store_test

import (
	"testing"

	// Packages
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	store "github.com/mutablelogic/go-llm/pkg/store"
	assert "github.com/stretchr/testify/assert"
)

func Test_file_credential_001(t *testing.T) {
	assert := assert.New(t)

	s, err := store.NewFileCredentialStore("test-passphrase", t.TempDir())
	assert.NoError(err)
	assert.NotNil(s)

	// Empty passphrase rejected
	_, err = store.NewFileCredentialStore("", t.TempDir())
	assert.Error(err)

	// Too short passphrase rejected
	_, err = store.NewFileCredentialStore("short", t.TempDir())
	assert.Error(err)

	// Whitespace-only passphrase rejected
	_, err = store.NewFileCredentialStore("       ", t.TempDir())
	assert.Error(err)
}

func Test_file_credential_002(t *testing.T) {
	runCredentialStoreTests(t, func() schema.CredentialStore {
		s, _ := store.NewFileCredentialStore("test-passphrase", t.TempDir())
		return s
	})
}
