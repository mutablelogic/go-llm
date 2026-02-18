package manager

import (
	"context"
	"errors"
	"fmt"

	// Packages
	"github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	"go.opentelemetry.io/otel/attribute"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// GetCredential retrieves the credential for the given server URL.
func (m *Manager) GetCredential(ctx context.Context, url string) (result *schema.OAuthCredentials, err error) {
	// Otel span — redact error to prevent credential leakage into traces
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "GetCredential",
		attribute.String("url", url),
	)
	defer func() { endSpan(redactCredentialErr(err)) }()

	if m.credentialStore == nil {
		return nil, llm.ErrNotImplemented.With("credential store not configured")
	}
	return m.credentialStore.GetCredential(ctx, url)
}

// SetCredential stores (or updates) the credential for the given server URL.
func (m *Manager) SetCredential(ctx context.Context, url string, cred schema.OAuthCredentials) (err error) {
	// Otel span — redact error to prevent credential leakage into traces
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "SetCredential",
		attribute.String("url", url),
	)
	defer func() { endSpan(redactCredentialErr(err)) }()

	if m.credentialStore == nil {
		return llm.ErrNotImplemented.With("credential store not configured")
	}
	return m.credentialStore.SetCredential(ctx, url, cred)
}

// DeleteCredential removes the credential for the given server URL.
func (m *Manager) DeleteCredential(ctx context.Context, url string) (err error) {
	// Otel span — redact error to prevent credential leakage into traces
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "DeleteCredential",
		attribute.String("url", url),
	)
	defer func() { endSpan(redactCredentialErr(err)) }()

	if m.credentialStore == nil {
		return llm.ErrNotImplemented.With("credential store not configured")
	}
	return m.credentialStore.DeleteCredential(ctx, url)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// redactCredentialErr returns a sanitised error for OTel spans.
// Known sentinel errors (ErrNotFound, ErrNotImplemented, etc.) pass through
// unchanged. All other errors are replaced with a generic message so that
// credential data never leaks into traces via json marshal/unmarshal messages.
func redactCredentialErr(err error) error {
	if err == nil {
		return nil
	}
	var llmErr llm.Err
	if errors.As(err, &llmErr) {
		return err // sentinel-wrapped errors are safe — they contain no credential data
	}
	return fmt.Errorf("credential operation failed")
}
