package encrypt

import (
	"crypto/rand"
	"fmt"
	"strings"

	// Packages
	argon2 "golang.org/x/crypto/argon2"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Key is a 256-bit encryption key derived from a passphrase.
type Key []byte

///////////////////////////////////////////////////////////////////////////////
// CONSTANTS

const (
	// Argon2id parameters (OWASP recommended minimums).
	argonTime    = 3
	argonMemory  = 64 * 1024 // 64 MiB
	argonThreads = 4
	argonKeyLen  = 32 // 256-bit key

	// SaltSize is the length of a random salt in bytes.
	SaltSize = 16

	// MinPassphraseLen is the minimum acceptable passphrase length.
	MinPassphraseLen = 8
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// ValidatePassphrase checks that the passphrase meets minimum security
// requirements: non-empty, not whitespace-only, and at least
// MinPassphraseLen characters long.
func ValidatePassphrase(passphrase string) error {
	trimmed := strings.TrimSpace(passphrase)
	if len(trimmed) == 0 {
		return fmt.Errorf("passphrase must not be empty")
	}
	if len(trimmed) < MinPassphraseLen {
		return fmt.Errorf("passphrase must be at least %d characters", MinPassphraseLen)
	}
	return nil
}

// DeriveKey derives a 256-bit encryption key from a passphrase and salt
// using Argon2id.
func DeriveKey(passphrase string, salt []byte) Key {
	return Key(argon2.IDKey([]byte(passphrase), salt, argonTime, argonMemory, argonThreads, argonKeyLen))
}

// GenerateSalt returns a cryptographically random 16-byte salt.
func GenerateSalt() ([]byte, error) {
	salt := make([]byte, SaltSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	return salt, nil
}
