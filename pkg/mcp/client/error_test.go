package client

import (
	"errors"
	"net/http"
	"testing"
)

// Test_error_001: NewUnauthorizedError parses error and error_description fields.
func Test_error_001(t *testing.T) {
	h := http.Header{}
	h.Add("Www-Authenticate", `Bearer error="invalid_token", error_description="Token expired"`)
	e := NewUnauthorizedError(h)
	if e.Error() != "invalid_token: Token expired" {
		t.Errorf("unexpected Error(): %q", e.Error())
	}
	if e.Get("error") != "invalid_token" {
		t.Errorf("Get error: %q", e.Get("error"))
	}
	if e.Get("error_description") != "Token expired" {
		t.Errorf("Get error_description: %q", e.Get("error_description"))
	}
	if len(e.Keys()) != 2 {
		t.Errorf("expected 2 keys, got %d: %v", len(e.Keys()), e.Keys())
	}
}

// Test_error_002: Error() falls back gracefully when only error_description is set.
func Test_error_002(t *testing.T) {
	h := http.Header{}
	h.Add("Www-Authenticate", `Bearer error_description="Access denied"`)
	e := NewUnauthorizedError(h)
	if e.Error() != "Access denied" {
		t.Errorf("unexpected Error(): %q", e.Error())
	}
}

// Test_error_003: Error() returns "unauthorized" when header is empty.
func Test_error_003(t *testing.T) {
	e := NewUnauthorizedError(http.Header{})
	if e.Error() != "unauthorized" {
		t.Errorf("unexpected Error(): %q", e.Error())
	}
}

// Test_error_004: Unwrap wraps httpresponse.ErrNotAuthorized so IsUnauthorized works.
func Test_error_004(t *testing.T) {
	e := NewUnauthorizedError(http.Header{})
	if !IsUnauthorized(e) {
		t.Error("expected IsUnauthorized true")
	}
}

// Test_error_005: ResourceMetadata returns the resource_metadata field.
func Test_error_005(t *testing.T) {
	h := http.Header{}
	h.Add("Www-Authenticate", `Bearer resource_metadata="https://example.com/.well-known/oauth"`)
	e := NewUnauthorizedError(h)
	if e.ResourceMetadata() != "https://example.com/.well-known/oauth" {
		t.Errorf("ResourceMetadata: %q", e.ResourceMetadata())
	}
}

// Test_error_006: AsUnauthorized unwraps a wrapped *UnauthorizedError.
func Test_error_006(t *testing.T) {
	h := http.Header{}
	h.Add("Www-Authenticate", `Bearer error="access_denied"`)
	e := NewUnauthorizedError(h)
	wrapped := errors.Join(errors.New("outer"), e)
	got := AsUnauthorized(wrapped)
	if got == nil {
		t.Fatal("expected non-nil *UnauthorizedError")
	}
	if got.Get("error") != "access_denied" {
		t.Errorf("unexpected error field: %q", got.Get("error"))
	}
}

// Test_error_007: IsForbidden detects the MCP SDK transport Forbidden message.
func Test_error_007(t *testing.T) {
	if !IsForbidden(errors.New(`sending "tools/list": Forbidden`)) {
		t.Error("expected IsForbidden true")
	}
	if IsForbidden(errors.New("other error")) {
		t.Error("expected IsForbidden false")
	}
	if IsForbidden(nil) {
		t.Error("expected IsForbidden false for nil")
	}
}

// Test_error_008: IsAuthError matches both 401 and 403 patterns.
func Test_error_008(t *testing.T) {
	if !IsAuthError(NewUnauthorizedError(http.Header{})) {
		t.Error("expected IsAuthError true for 401")
	}
	if !IsAuthError(errors.New(`sending "tools/list": Forbidden`)) {
		t.Error("expected IsAuthError true for Forbidden")
	}
	if IsAuthError(errors.New("other error")) {
		t.Error("expected IsAuthError false")
	}
}
