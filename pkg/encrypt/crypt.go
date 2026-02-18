package encrypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Encrypt generates a fresh salt, derives a key from the passphrase, and
// encrypts plaintext using AES-256-GCM. The returned blob is:
//
//	salt (16 bytes) || nonce (12 bytes) || ciphertext + tag
//
// Example usage:
//
//	blob, err := encrypt.Encrypt("my-passphrase", []byte("secret"))
//	blob, err := encrypt.Encrypt("my-passphrase", "secret")
func Encrypt[T interface{ []byte | string }](passphrase string, plaintext T) ([]byte, error) {
	salt, err := GenerateSalt()
	if err != nil {
		return nil, fmt.Errorf("encrypt: %w", err)
	}
	key := DeriveKey(passphrase, salt)
	ct, err := key.Encrypt([]byte(plaintext))
	if err != nil {
		return nil, err
	}
	// Prepend salt to the sealed output
	return append(salt, ct...), nil
}

// Decrypt splits the salt from the blob, re-derives the key, and decrypts
// ciphertext produced by Encrypt. The type parameter controls the return type.
//
// Example usage:
//
//	plaintext, err := encrypt.Decrypt[[]byte]("my-passphrase", blob)
//	text, err := encrypt.Decrypt[string]("my-passphrase", blob)
func Decrypt[T interface{ []byte | string }](passphrase string, blob []byte) (T, error) {
	var zero T
	if len(blob) < SaltSize {
		return zero, fmt.Errorf("decrypt: data too short")
	}
	salt, ct := blob[:SaltSize], blob[SaltSize:]
	key := DeriveKey(passphrase, salt)
	plaintext, err := key.Decrypt(ct)
	if err != nil {
		return zero, err
	}
	return T(plaintext), nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Encrypt encrypts plaintext using AES-256-GCM with a random nonce.
// Returns nonce || ciphertext + tag.
func (k Key) Encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(k)
	if err != nil {
		return nil, fmt.Errorf("encrypt: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("encrypt: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("encrypt: %w", err)
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt decrypts ciphertext (nonce || ciphertext + tag) using AES-256-GCM.
func (k Key) Decrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(k)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	if len(ciphertext) < gcm.NonceSize() {
		return nil, fmt.Errorf("decrypt: ciphertext too short")
	}
	nonce, data := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, data, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	return plaintext, nil
}
