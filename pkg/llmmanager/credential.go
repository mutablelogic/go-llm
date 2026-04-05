package manager

import (
	"context"
	"encoding/json"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	pg "github.com/mutablelogic/go-pg"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	types "github.com/mutablelogic/go-server/pkg/types"
	attribute "go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// CreateCredential persists an encrypted credential row and returns the public
// credential shape, excluding passphrase version and encrypted payload.
func (m *Manager) CreateCredential(ctx context.Context, req schema.CredentialInsert) (_ *schema.Credential, err error) {
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "CreateCredential",
		attribute.String("req", req.RedactedString()),
	)
	defer func() { endSpan(err) }()

	// Encrypt the credential data
	pv, credentials, err := m.encryptCredentials(req.Credentials)
	if err != nil {
		return nil, err
	} else {
		req.Credentials = credentials
	}

	// Insert the credential record
	var result schema.Credential
	if err := m.PoolConn.Tx(ctx, func(conn pg.Conn) error {
		return conn.With("pv", pv).Insert(ctx, &result, req)
	}); err != nil {
		return nil, pg.NormalizeError(err)
	}

	// Return success
	return types.Ptr(result), nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func (m *Manager) encryptCredentials(v any) (uint64, []byte, error) {
	// Turn the credentials into JSON. If the credentials are empty this will
	// return an empty JSON object, which we can treat as an empty byte array.
	data, err := json.Marshal(v)
	if err != nil {
		return 0, nil, httpresponse.ErrBadRequest.With(err)
	} else if string(data) == "{}" {
		return 0, []byte{}, nil
	}

	// Check for at least one passphrase configured
	if len(m.passphrases.Keys()) == 0 {
		return 0, nil, httpresponse.ErrServiceUnavailable.Withf("no encryption passphrase configured for credentials")
	}

	// Get the encryption passphrase for the current passphrase version. If there is no
	// passphrase configured for the current version, return an error
	if pv, crypted, err := m.passphrases.Encrypt(0, data); err != nil {
		return 0, nil, httpresponse.ErrBadRequest.With(err)
	} else {
		return pv, []byte(crypted), nil
	}
}

func (m *Manager) decryptCredentials(encrypted []byte, pv uint64, decrypted any) error {
	if len(encrypted) == 0 {
		return nil
	}

	// Check for at least one passphrase configured
	if len(m.passphrases.Keys()) == 0 {
		return httpresponse.ErrServiceUnavailable.Withf("no encryption passphrase configured for credentials")
	}

	// Decrypt the credentials using the passphrase version and encrypted data, and
	// then unmarshal the JSON into the provided decrypted structure.
	if data, err := m.passphrases.Decrypt(pv, string(encrypted)); err != nil {
		return httpresponse.ErrBadRequest.With(err)
	} else if err := json.Unmarshal([]byte(data), decrypted); err != nil {
		return httpresponse.ErrBadRequest.With(err)
	} else {
		return nil
	}
}
