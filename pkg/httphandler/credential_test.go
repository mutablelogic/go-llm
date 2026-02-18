package httphandler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	// Packages
	manager "github.com/mutablelogic/go-llm/pkg/manager"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	store "github.com/mutablelogic/go-llm/pkg/store"
	oauth2 "golang.org/x/oauth2"
)

///////////////////////////////////////////////////////////////////////////////
// HELPERS

func newCredentialManager(t *testing.T) *manager.Manager {
	t.Helper()
	cs, err := store.NewMemoryCredentialStore("test-passphrase")
	if err != nil {
		t.Fatal(err)
	}
	m, err := manager.NewManager(manager.WithCredentialStore(cs))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func credentialPath(rawURL string) string {
	return "/credential/" + url.PathEscape(rawURL)
}

///////////////////////////////////////////////////////////////////////////////
// TESTS

func TestCredential_GetNotFound(t *testing.T) {
	mgr := newCredentialManager(t)
	mux := serveMux(mgr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, credentialPath("https://example.com"), nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCredential_SetAndGet(t *testing.T) {
	mgr := newCredentialManager(t)
	mux := serveMux(mgr)

	cred := schema.OAuthCredentials{
		Token: &oauth2.Token{
			AccessToken:  "access-123",
			RefreshToken: "refresh-456",
			TokenType:    "Bearer",
		},
		ClientID: "client-abc",
		Endpoint: "https://example.com",
		TokenURL: "https://example.com/token",
	}

	body, err := json.Marshal(cred)
	if err != nil {
		t.Fatal(err)
	}

	// POST credential
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, credentialPath("https://example.com"), bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNoContent {
		t.Fatalf("POST: expected 204, got %d: %s", w.Code, w.Body.String())
	}

	// GET credential
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodGet, credentialPath("https://example.com"), nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("GET: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var got schema.OAuthCredentials
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.AccessToken != "access-123" {
		t.Fatalf("expected AccessToken=access-123, got %q", got.AccessToken)
	}
	if got.ClientID != "client-abc" {
		t.Fatalf("expected ClientID=client-abc, got %q", got.ClientID)
	}
	if got.TokenURL != "https://example.com/token" {
		t.Fatalf("expected TokenURL=https://example.com/token, got %q", got.TokenURL)
	}
}

func TestCredential_Delete(t *testing.T) {
	mgr := newCredentialManager(t)
	mux := serveMux(mgr)

	cred := schema.OAuthCredentials{
		Token:    &oauth2.Token{AccessToken: "token-1"},
		ClientID: "client-1",
		Endpoint: "https://example.com",
		TokenURL: "https://example.com/token",
	}

	body, _ := json.Marshal(cred)

	// POST
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, credentialPath("https://example.com"), bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusNoContent {
		t.Fatalf("POST: expected 204, got %d: %s", w.Code, w.Body.String())
	}

	// DELETE
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodDelete, credentialPath("https://example.com"), nil)
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusNoContent {
		t.Fatalf("DELETE: expected 204, got %d: %s", w.Code, w.Body.String())
	}

	// GET should 404
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodGet, credentialPath("https://example.com"), nil)
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("GET after DELETE: expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCredential_DeleteNotFound(t *testing.T) {
	mgr := newCredentialManager(t)
	mux := serveMux(mgr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodDelete, credentialPath("https://nonexistent.example.com"), nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCredential_MethodNotAllowed(t *testing.T) {
	mgr := newCredentialManager(t)
	mux := serveMux(mgr)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPatch, credentialPath("https://example.com"), nil)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCredential_SetOverwrites(t *testing.T) {
	mgr := newCredentialManager(t)
	mux := serveMux(mgr)

	cred1 := schema.OAuthCredentials{
		Token:    &oauth2.Token{AccessToken: "old-token"},
		ClientID: "client-1",
		Endpoint: "https://example.com",
		TokenURL: "https://example.com/token",
	}
	cred2 := schema.OAuthCredentials{
		Token:    &oauth2.Token{AccessToken: "new-token"},
		ClientID: "client-2",
		Endpoint: "https://example.com",
		TokenURL: "https://example.com/token",
	}

	// POST first credential
	body, _ := json.Marshal(cred1)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, credentialPath("https://example.com"), bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusNoContent {
		t.Fatalf("POST #1: expected 204, got %d: %s", w.Code, w.Body.String())
	}

	// POST second credential (overwrite)
	body, _ = json.Marshal(cred2)
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodPost, credentialPath("https://example.com"), bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusNoContent {
		t.Fatalf("POST #2: expected 204, got %d: %s", w.Code, w.Body.String())
	}

	// GET should return updated credential
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodGet, credentialPath("https://example.com"), nil)
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("GET: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var got schema.OAuthCredentials
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.AccessToken != "new-token" {
		t.Fatalf("expected AccessToken=new-token, got %q", got.AccessToken)
	}
	if got.ClientID != "client-2" {
		t.Fatalf("expected ClientID=client-2, got %q", got.ClientID)
	}
}
