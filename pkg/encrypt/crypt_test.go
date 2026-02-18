package encrypt_test

import (
	"bytes"
	"testing"

	encrypt "github.com/mutablelogic/go-llm/pkg/encrypt"
	assert "github.com/stretchr/testify/assert"
)

func Test_crypt_001(t *testing.T) {
	// Round-trip with []byte plaintext
	assert := assert.New(t)
	plaintext := []byte("hello, world")
	blob, err := encrypt.Encrypt("passphrase", plaintext)
	assert.NoError(err)
	assert.NotNil(blob)

	got, err := encrypt.Decrypt[[]byte]("passphrase", blob)
	assert.NoError(err)
	assert.True(bytes.Equal(plaintext, got))
}

func Test_crypt_002(t *testing.T) {
	// Round-trip with string plaintext
	assert := assert.New(t)
	blob, err := encrypt.Encrypt("passphrase", "hello, world")
	assert.NoError(err)
	assert.NotNil(blob)

	got, err := encrypt.Decrypt[string]("passphrase", blob)
	assert.NoError(err)
	assert.Equal("hello, world", got)
}

func Test_crypt_003(t *testing.T) {
	// Wrong passphrase fails to decrypt
	assert := assert.New(t)
	blob, err := encrypt.Encrypt("correct", []byte("secret"))
	assert.NoError(err)

	_, err = encrypt.Decrypt[[]byte]("wrong", blob)
	assert.Error(err)
}

func Test_crypt_004(t *testing.T) {
	// Empty plaintext round-trips
	assert := assert.New(t)
	blob, err := encrypt.Encrypt("pass", []byte(""))
	assert.NoError(err)

	got, err := encrypt.Decrypt[[]byte]("pass", blob)
	assert.NoError(err)
	assert.Empty(got)
}

func Test_crypt_005(t *testing.T) {
	// Truncated blob fails
	assert := assert.New(t)
	_, err := encrypt.Decrypt[[]byte]("pass", []byte("short"))
	assert.Error(err)
}

func Test_crypt_006(t *testing.T) {
	// Two encryptions of the same data produce different blobs (unique salt + nonce)
	assert := assert.New(t)
	blob1, err := encrypt.Encrypt("pass", []byte("data"))
	assert.NoError(err)
	blob2, err := encrypt.Encrypt("pass", []byte("data"))
	assert.NoError(err)
	assert.False(bytes.Equal(blob1, blob2))
}

func Test_crypt_007(t *testing.T) {
	// Key.Encrypt / Key.Decrypt round-trip
	assert := assert.New(t)
	salt, err := encrypt.GenerateSalt()
	assert.NoError(err)

	key := encrypt.DeriveKey("passphrase", salt)
	ct, err := key.Encrypt([]byte("secret data"))
	assert.NoError(err)

	got, err := key.Decrypt(ct)
	assert.NoError(err)
	assert.Equal("secret data", string(got))
}

func Test_crypt_008(t *testing.T) {
	// Key.Decrypt with different key fails
	assert := assert.New(t)
	salt1, _ := encrypt.GenerateSalt()
	salt2, _ := encrypt.GenerateSalt()

	key1 := encrypt.DeriveKey("pass", salt1)
	key2 := encrypt.DeriveKey("pass", salt2)

	ct, err := key1.Encrypt([]byte("data"))
	assert.NoError(err)

	_, err = key2.Decrypt(ct)
	assert.Error(err)
}
