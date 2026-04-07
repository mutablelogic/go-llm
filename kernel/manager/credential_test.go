package manager

import (
	"context"
	"testing"

	// Packages
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	llmtest "github.com/mutablelogic/go-llm/pkg/test"
	assert "github.com/stretchr/testify/assert"
)

func Test_encryptCredentialDataEmptyReturnsNoPayload(t *testing.T) {
	assert := assert.New(t)

	var manager Manager
	manager.defaults("test", "0.0.0")
	if err := manager.passphrases.Set(1, "test1234"); !assert.NoError(err) {
		return
	}

	pv, credentials, err := manager.encryptCredentials(nil)
	if !assert.NoError(err) {
		return
	}

	assert.Equal(uint64(0), pv)
	assert.Empty(credentials)
}

func Test_encryptCredentialDataRoundTrip(t *testing.T) {
	assert := assert.New(t)

	var manager Manager
	manager.defaults("test", "0.0.0")
	if err := manager.passphrases.Set(1, "test1234"); !assert.NoError(err) {
		return
	}

	pv, encrypted, err := manager.encryptCredentials([]byte("secret"))
	if !assert.NoError(err) {
		return
	}
	assert.NotZero(pv)
	assert.NotEqual([]byte("secret"), encrypted)

	var decrypted []byte
	if !assert.NoError(manager.decryptCredentials(encrypted, pv, &decrypted)) {
		return
	}
	assert.Equal([]byte("secret"), decrypted)
}

func TestCreateCredential(t *testing.T) {
	conn, m := newIntegrationManager(t)
	ctx := context.Background()

	if err := m.Exec(ctx, `TRUNCATE llm.credential CASCADE`); err != nil {
		t.Fatal(err)
	}

	user := llmtest.User(conn)
	userID := user.UUID()
	created, err := m.CreateCredential(ctx, schema.CredentialInsert{
		CredentialKey: schema.CredentialKey{
			URL:  "HTTPS://Example.COM/sse?token=abc#frag",
			User: &userID,
		},
		Credentials: []byte("encrypted"),
	})
	if err != nil {
		t.Fatal(err)
	}

	assert := assert.New(t)
	assert.Equal("https://example.com/sse", created.URL)
	if assert.NotNil(created.User) {
		assert.Equal(userID, *created.User)
	}
	assert.False(created.CreatedAt.IsZero())
}

func TestCreateCredentialGlobal(t *testing.T) {
	_, m := newIntegrationManager(t)
	ctx := context.Background()

	if err := m.Exec(ctx, `TRUNCATE llm.credential CASCADE`); err != nil {
		t.Fatal(err)
	}

	created, err := m.CreateCredential(ctx, schema.CredentialInsert{
		CredentialKey: schema.CredentialKey{
			URL: "https://example.com/sse",
		},
		Credentials: []byte("encrypted"),
	})
	if err != nil {
		t.Fatal(err)
	}

	assert := assert.New(t)
	assert.Equal("https://example.com/sse", created.URL)
	assert.Nil(created.User)
	assert.False(created.CreatedAt.IsZero())
}
