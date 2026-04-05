package schema_test

import (
	"errors"
	"testing"
	"time"

	// Packages
	uuid "github.com/google/uuid"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	pg "github.com/mutablelogic/go-pg"
	assert "github.com/stretchr/testify/assert"
)

type credentialMockRow struct {
	values []any
}

func (r credentialMockRow) Scan(dest ...any) error {
	if len(dest) != len(r.values) {
		return errors.New("unexpected scan arity")
	}
	for i := range dest {
		switch target := dest[i].(type) {
		case *string:
			*target = r.values[i].(string)
		case **uuid.UUID:
			switch value := r.values[i].(type) {
			case *uuid.UUID:
				*target = value
			case uuid.UUID:
				uuidValue := value
				*target = &uuidValue
			case nil:
				*target = nil
			default:
				return errors.New("unsupported uuid scan source")
			}
		case *time.Time:
			*target = r.values[i].(time.Time)
		default:
			return errors.New("unsupported scan type")
		}
	}
	return nil
}

func TestCredentialScan(t *testing.T) {
	assert := assert.New(t)
	createdAt := time.Unix(100, 0).UTC()
	user := uuid.New()

	var credential schema.Credential
	err := credential.Scan(credentialMockRow{values: []any{
		"https://example.com/sse",
		user,
		createdAt,
	}})
	if !assert.NoError(err) {
		return
	}

	assert.Equal("https://example.com/sse", credential.URL)
	if assert.NotNil(credential.User) {
		assert.Equal(user, *credential.User)
	}
	assert.Equal(createdAt, credential.CreatedAt)
}

func TestCredentialScanGlobal(t *testing.T) {
	assert := assert.New(t)
	createdAt := time.Unix(100, 0).UTC()

	var credential schema.Credential
	err := credential.Scan(credentialMockRow{values: []any{
		"https://example.com/sse",
		nil,
		createdAt,
	}})
	if !assert.NoError(err) {
		return
	}

	assert.Equal("https://example.com/sse", credential.URL)
	assert.Nil(credential.User)
	assert.Equal(createdAt, credential.CreatedAt)
}

func TestCredentialInsert(t *testing.T) {
	assert := assert.New(t)
	b := pg.NewBind("schema", "llm", "credential.insert", "INSERT", "pv", uint64(7))
	user := uuid.New()

	query, err := (schema.CredentialInsert{
		CredentialKey: schema.CredentialKey{
			URL:  "HTTPS://Example.COM/sse?token=abc#frag",
			User: &user,
		},
		Credentials: []byte("encrypted"),
	}).Insert(b)
	if !assert.NoError(err) {
		return
	}

	assert.Equal("INSERT", query)
	assert.Equal("https://example.com/sse", b.Get("url"))
	assert.Equal(user, b.Get("user"))
	assert.Equal(uint64(7), b.Get("pv"))
	assert.Equal([]byte("encrypted"), b.Get("credentials"))
}

func TestCredentialInsertGlobal(t *testing.T) {
	assert := assert.New(t)
	b := pg.NewBind("schema", "llm", "credential.insert", "INSERT", "pv", uint64(0))

	query, err := (schema.CredentialInsert{
		CredentialKey: schema.CredentialKey{URL: "https://example.com/sse"},
		Credentials:   []byte("encrypted"),
	}).Insert(b)
	if !assert.NoError(err) {
		return
	}

	assert.Equal("INSERT", query)
	assert.Equal("https://example.com/sse", b.Get("url"))
	assert.Nil(b.Get("user"))
	assert.Equal(uint64(0), b.Get("pv"))
	assert.Equal([]byte("encrypted"), b.Get("credentials"))
}

func TestCredentialInsertRequiresPVBinding(t *testing.T) {
	assert := assert.New(t)
	b := pg.NewBind("schema", "llm", "credential.insert", "INSERT")

	_, err := (schema.CredentialInsert{
		CredentialKey: schema.CredentialKey{URL: "https://example.com/sse"},
		Credentials:   []byte("encrypted"),
	}).Insert(b)
	if assert.Error(err) {
		assert.ErrorIs(err, schema.ErrInternalServerError)
	}
}

func TestCredentialInsertRequiresCredentials(t *testing.T) {
	assert := assert.New(t)
	b := pg.NewBind("schema", "llm", "credential.insert", "INSERT")

	_, err := (schema.CredentialInsert{CredentialKey: schema.CredentialKey{URL: "https://example.com/sse"}}).Insert(b)
	if assert.Error(err) {
		assert.ErrorIs(err, schema.ErrBadParameter)
	}
}

func TestCredentialInsertRedactedString(t *testing.T) {
	assert := assert.New(t)
	user := uuid.New()
	redacted := (schema.CredentialInsert{
		CredentialKey: schema.CredentialKey{
			URL:  "https://example.com/sse",
			User: &user,
		},
		Credentials: []byte("encrypted"),
	}).RedactedString()

	assert.Contains(redacted, `"url": "https://example.com/sse"`)
	assert.Contains(redacted, `"credentials": null`)
	assert.NotContains(redacted, `"pv"`)
	assert.NotContains(redacted, "encrypted")
}
