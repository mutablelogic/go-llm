//go:build !client

package main

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	// Packages
	credential "github.com/mutablelogic/go-auth/credential"
	filecredential "github.com/mutablelogic/go-auth/credential/file"
	llm "github.com/mutablelogic/go-llm"
	server "github.com/mutablelogic/go-server"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// credentialsDir is the directory, in the directory of the command's
	// defaults, which the server keeps credentials in, encrypted, such as
	// how connectors' servers are signed in to
	credentialsDir = "credentials"

	// credentialsKeyFile is the file, beside the credentials directory,
	// which has the passphrase they're encrypted with, which is made the
	// first time the server runs. Without it, they can't be read, and the
	// connectors are signed in to again.
	credentialsKeyFile = "credentials.key"
)

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// newServerCredentials returns the provider of credentials, which are
// environment variables, or are kept, encrypted, in the credentials
// directory, in the directory of the command's defaults
func newServerCredentials(ctx server.Cmd) (credential.CredentialProvider, error) {
	dir, err := serverDir(ctx)
	if err != nil {
		return nil, err
	}
	passphrase, err := credentialsKey(filepath.Join(dir, credentialsKeyFile))
	if err != nil {
		return nil, err
	}
	return filecredential.New(filepath.Join(dir, credentialsDir), filecredential.WithEnv(), filecredential.WithPassphrase(1, passphrase), filecredential.WithTracer(ctx.Tracer()))
}

// credentialsKey returns the passphrase in the file, or makes one, of 32
// random bytes, and writes it, readable only by its owner, when there's
// none. It's written to a temporary file, which is linked to the path, so
// a server which starts at the same time reads the same one.
func credentialsKey(path string) (string, error) {
	if passphrase, err := readCredentialsKey(path); err == nil || !errors.Is(err, fs.ErrNotExist) {
		return passphrase, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	data := make([]byte, 32)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return "", err
	}
	defer os.Remove(temp.Name())
	if _, err := temp.WriteString(base64.RawURLEncoding.EncodeToString(data) + "\n"); err != nil {
		temp.Close()
		return "", err
	} else if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return "", err
	} else if err := temp.Close(); err != nil {
		return "", err
	}
	if err := os.Link(temp.Name(), path); err != nil && !errors.Is(err, fs.ErrExist) {
		return "", err
	}
	return readCredentialsKey(path)
}

// readCredentialsKey returns the passphrase in the file, which is only
// readable by its owner
func readCredentialsKey(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	} else if info.Mode().Perm()&0o077 != 0 {
		return "", llm.ErrBadParameter.Withf("%s is readable by others than its owner", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	passphrase := strings.TrimSpace(string(data))
	if passphrase == "" {
		return "", llm.ErrBadParameter.Withf("%s is empty", path)
	}
	return passphrase, nil
}
