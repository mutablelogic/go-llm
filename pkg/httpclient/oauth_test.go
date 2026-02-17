package httpclient_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	httpclient "github.com/mutablelogic/go-llm/pkg/httpclient"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	oauth2 "golang.org/x/oauth2"
)

///////////////////////////////////////////////////////////////////////////////
// MOCK OAUTH SERVER

type mockOAuthServer struct {
	*httptest.Server
	metadata         *schema.OAuthMetadata
	registeredClient *schema.OAuthClientInfo
	deviceCode       string
	deviceAuthorized bool
}

func newMockOAuthServer(t *testing.T) *mockOAuthServer {
	t.Helper()

	mock := &mockOAuthServer{
		deviceCode:       "test-device-code",
		deviceAuthorized: false,
	}

	mux := http.NewServeMux()

	// Discovery endpoint
	mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mock.metadata)
	})

	// Authorization endpoint (simulated - in real flow browser would redirect)
	mux.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) {
		// Extract callback URL and state, redirect with code
		redirectURI := r.URL.Query().Get("redirect_uri")
		state := r.URL.Query().Get("state")
		if redirectURI == "" || state == "" {
			http.Error(w, "missing redirect_uri or state", http.StatusBadRequest)
			return
		}
		// Redirect to callback with code
		u, _ := url.Parse(redirectURI)
		q := u.Query()
		q.Set("code", "test-auth-code")
		q.Set("state", state)
		u.RawQuery = q.Encode()
		http.Redirect(w, r, u.String(), http.StatusFound)
	})

	// Token endpoint
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		grantType := r.FormValue("grant_type")
		w.Header().Set("Content-Type", "application/json")

		switch grantType {
		case "authorization_code":
			code := r.FormValue("code")
			if code != "test-auth-code" {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant"})
				return
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token":  "test-access-token",
				"token_type":    "Bearer",
				"expires_in":    3600,
				"refresh_token": "test-refresh-token",
			})

		case "urn:ietf:params:oauth:grant-type:device_code":
			deviceCode := r.FormValue("device_code")
			if deviceCode != mock.deviceCode {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant"})
				return
			}
			if !mock.deviceAuthorized {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{"error": "authorization_pending"})
				return
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token":  "test-device-access-token",
				"token_type":    "Bearer",
				"expires_in":    3600,
				"refresh_token": "test-device-refresh-token",
			})

		case "client_credentials":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token": "test-client-credentials-token",
				"token_type":   "Bearer",
				"expires_in":   3600,
			})

		case "refresh_token":
			refreshToken := r.FormValue("refresh_token")
			if refreshToken == "" {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant"})
				return
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token":  "test-refreshed-access-token",
				"token_type":    "Bearer",
				"expires_in":    3600,
				"refresh_token": "test-new-refresh-token",
			})

		default:
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "unsupported_grant_type"})
		}
	})

	// Device authorization endpoint
	mux.HandleFunc("/device/code", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"device_code":               mock.deviceCode,
			"user_code":                 "ABCD-1234",
			"verification_uri":          mock.Server.URL + "/device",
			"verification_uri_complete": mock.Server.URL + "/device?user_code=ABCD-1234",
			"expires_in":                600,
			"interval":                  1,
		})
	})

	// Registration endpoint
	mux.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req schema.OAuthClientRegistration
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		mock.registeredClient = &schema.OAuthClientInfo{
			ClientID:                "registered-client-id",
			ClientSecret:            "",
			ClientName:              req.ClientName,
			RedirectURIs:            req.RedirectURIs,
			GrantTypes:              req.GrantTypes,
			ResponseTypes:           req.ResponseTypes,
			TokenEndpointAuthMethod: req.TokenEndpointAuthMethod,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(mock.registeredClient)
	})

	mock.Server = httptest.NewServer(mux)

	// Set up metadata after server is created (need URL)
	mock.metadata = &schema.OAuthMetadata{
		Issuer:                      mock.Server.URL,
		AuthorizationEndpoint:       mock.Server.URL + "/authorize",
		TokenEndpoint:               mock.Server.URL + "/token",
		DeviceAuthorizationEndpoint: mock.Server.URL + "/device/code",
		RegistrationEndpoint:        mock.Server.URL + "/register",
		GrantTypesSupported: []string{
			"authorization_code",
			"refresh_token",
			"client_credentials",
			"urn:ietf:params:oauth:grant-type:device_code",
		},
		ResponseTypesSupported:        []string{"code"},
		CodeChallengeMethodsSupported: []string{"S256", "plain"},
	}

	return mock
}

func (m *mockOAuthServer) AuthorizeDevice() {
	m.deviceAuthorized = true
}

func (m *mockOAuthServer) RegisteredClient() *schema.OAuthClientInfo {
	return m.registeredClient
}

///////////////////////////////////////////////////////////////////////////////
// TESTS

func TestInteractiveLogin(t *testing.T) {
	mock := newMockOAuthServer(t)
	defer mock.Server.Close()

	c, err := httpclient.New(mock.Server.URL)
	if err != nil {
		t.Fatal(err)
	}

	// Create listener for callback
	listener, redirectURI, err := httpclient.NewCallbackListener("")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	// Start a goroutine to simulate the browser redirect
	var authURL string
	go func() {
		// Wait a bit for the server to start
		time.Sleep(100 * time.Millisecond)

		// Simulate browser: hit the auth URL, which will redirect to our callback
		resp, err := http.Get(authURL)
		if err != nil {
			t.Logf("simulated browser request failed: %v", err)
			return
		}
		resp.Body.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg := &oauth2.Config{
		ClientID: "test-client",
		Scopes:   []string{"openid"},
		Endpoint: oauth2.Endpoint{AuthURL: mock.Server.URL},
	}
	token, err := c.Login(ctx, cfg, httpclient.OptInteractive(listener, func(url string) {
		authURL = url
	}))
	if err != nil {
		t.Fatal(err)
	}

	if token.AccessToken != "test-access-token" {
		t.Errorf("unexpected access token: %s", token.AccessToken)
	}
	if token.RefreshToken != "test-refresh-token" {
		t.Errorf("unexpected refresh token: %s", token.RefreshToken)
	}
	if token.ClientID != "test-client" {
		t.Errorf("unexpected client ID: %s", token.ClientID)
	}
	if token.Endpoint != mock.Server.URL {
		t.Errorf("unexpected endpoint: %s", token.Endpoint)
	}

	_ = redirectURI // Used in registration
}

func TestInteractiveLogin_AutoRegister(t *testing.T) {
	mock := newMockOAuthServer(t)
	defer mock.Server.Close()

	c, err := httpclient.New(mock.Server.URL)
	if err != nil {
		t.Fatal(err)
	}

	// Create listener for callback
	listener, _, err := httpclient.NewCallbackListener("")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	// Start a goroutine to simulate the browser redirect
	var authURL string
	go func() {
		time.Sleep(100 * time.Millisecond)
		resp, err := http.Get(authURL)
		if err != nil {
			t.Logf("simulated browser request failed: %v", err)
			return
		}
		resp.Body.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// No ClientID — should auto-register using OptClientName
	cfg := &oauth2.Config{
		Scopes:   []string{"openid"},
		Endpoint: oauth2.Endpoint{AuthURL: mock.Server.URL},
	}
	token, err := c.Login(ctx, cfg, httpclient.OptClientName("test-app"), httpclient.OptInteractive(listener, func(url string) {
		authURL = url
	}))
	if err != nil {
		t.Fatal(err)
	}

	// Should have received the registered client ID
	if token.ClientID != "registered-client-id" {
		t.Errorf("expected registered client ID, got: %s", token.ClientID)
	}
	if token.AccessToken != "test-access-token" {
		t.Errorf("unexpected access token: %s", token.AccessToken)
	}

	// Verify registration happened on the mock
	if mock.RegisteredClient() == nil {
		t.Fatal("expected client to be registered")
	}
	if mock.RegisteredClient().ClientName != "test-app" {
		t.Errorf("unexpected registered client name: %s", mock.RegisteredClient().ClientName)
	}
}

func TestClientCredentialsLogin(t *testing.T) {
	mock := newMockOAuthServer(t)
	defer mock.Server.Close()

	c, err := httpclient.New(mock.Server.URL)
	if err != nil {
		t.Fatal(err)
	}

	cfg := &oauth2.Config{
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Scopes:       []string{"api"},
		Endpoint:     oauth2.Endpoint{AuthURL: mock.Server.URL},
	}
	token, err := c.Login(context.Background(), cfg, httpclient.OptClientCredentials())
	if err != nil {
		t.Fatal(err)
	}

	if token.AccessToken != "test-client-credentials-token" {
		t.Errorf("unexpected access token: %s", token.AccessToken)
	}
}

func TestDeviceLogin(t *testing.T) {
	mock := newMockOAuthServer(t)
	defer mock.Server.Close()

	c, err := httpclient.New(mock.Server.URL)
	if err != nil {
		t.Fatal(err)
	}

	// Simulate user authorizing the device after a short delay
	go func() {
		time.Sleep(200 * time.Millisecond)
		mock.AuthorizeDevice()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var verificationURI, userCode string
	cfg := &oauth2.Config{
		ClientID: "test-client",
		Scopes:   []string{"openid"},
		Endpoint: oauth2.Endpoint{AuthURL: mock.Server.URL},
	}
	token, err := c.Login(ctx, cfg, httpclient.OptDevice(func(uri, code string) {
		verificationURI = uri
		userCode = code
	}))
	if err != nil {
		t.Fatal(err)
	}

	if verificationURI == "" {
		t.Error("expected verification URI")
	}
	if userCode != "ABCD-1234" {
		t.Errorf("unexpected user code: %s", userCode)
	}
	if token.AccessToken != "test-device-access-token" {
		t.Errorf("unexpected access token: %s", token.AccessToken)
	}
}

func TestNewCallbackListener(t *testing.T) {
	// Test with empty address (random port)
	listener, redirectURI, err := httpclient.NewCallbackListener("")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	// Accept either localhost or 127.0.0.1
	validPrefix := strings.HasPrefix(redirectURI, "http://localhost:") || strings.HasPrefix(redirectURI, "http://127.0.0.1:")
	if !validPrefix || !strings.HasSuffix(redirectURI, "/callback") {
		t.Errorf("unexpected redirect URI format: %s", redirectURI)
	}
}

func TestNewCallbackListener_NonLoopback(t *testing.T) {
	// Test that non-loopback addresses are rejected
	_, _, err := httpclient.NewCallbackListener("0.0.0.0:8080")
	if err == nil {
		t.Fatal("expected error for non-loopback address")
	}
	if !strings.Contains(err.Error(), "loopback") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestNewCallbackListener_MissingPort(t *testing.T) {
	// Test that missing port is rejected
	_, _, err := httpclient.NewCallbackListener("localhost")
	if err == nil {
		t.Fatal("expected error for missing port")
	}
}

func TestRefreshToken(t *testing.T) {
	mock := newMockOAuthServer(t)
	defer mock.Server.Close()

	c, err := httpclient.New(mock.Server.URL)
	if err != nil {
		t.Fatal(err)
	}

	// Create expired credentials with a refresh token
	oldCreds := &schema.OAuthCredentials{
		Token: &oauth2.Token{
			AccessToken:  "expired-access-token",
			RefreshToken: "test-refresh-token",
			// Expiry in the past forces a refresh
			Expiry: time.Now().Add(-time.Hour),
		},
		ClientID: "test-client",
		Endpoint: mock.Server.URL,
		TokenURL: mock.Server.URL + "/token",
	}

	newCreds, err := c.RefreshToken(context.Background(), oldCreds, true)
	if err != nil {
		t.Fatal(err)
	}

	if newCreds.AccessToken != "test-refreshed-access-token" {
		t.Errorf("unexpected access token: %s", newCreds.AccessToken)
	}
	if newCreds.RefreshToken != "test-new-refresh-token" {
		t.Errorf("unexpected refresh token: %s", newCreds.RefreshToken)
	}
	if newCreds.ClientID != "test-client" {
		t.Errorf("unexpected client ID: %s", newCreds.ClientID)
	}
	if newCreds.Endpoint != mock.Server.URL {
		t.Errorf("unexpected endpoint: %s", newCreds.Endpoint)
	}
	if newCreds.TokenURL != mock.Server.URL+"/token" {
		t.Errorf("unexpected token URL: %s", newCreds.TokenURL)
	}
}

func TestRefreshToken_NoRefreshToken(t *testing.T) {
	mock := newMockOAuthServer(t)
	defer mock.Server.Close()

	c, err := httpclient.New(mock.Server.URL)
	if err != nil {
		t.Fatal(err)
	}

	// Credentials without refresh token
	oldCreds := &schema.OAuthCredentials{
		Token: &oauth2.Token{
			AccessToken: "some-access-token",
		},
		ClientID: "test-client",
		Endpoint: mock.Server.URL,
		TokenURL: mock.Server.URL + "/token",
	}

	_, err = c.RefreshToken(context.Background(), oldCreds, true)
	if err == nil {
		t.Fatal("expected error for token without refresh token")
	}
	if !strings.Contains(err.Error(), "does not contain a refresh token") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRefreshToken_NotExpired(t *testing.T) {
	mock := newMockOAuthServer(t)
	defer mock.Server.Close()

	c, err := httpclient.New(mock.Server.URL)
	if err != nil {
		t.Fatal(err)
	}

	// Credentials that are still valid (expires in 1 hour)
	oldCreds := &schema.OAuthCredentials{
		Token: &oauth2.Token{
			AccessToken:  "still-valid-access-token",
			RefreshToken: "test-refresh-token",
			Expiry:       time.Now().Add(time.Hour),
		},
		ClientID: "test-client",
		Endpoint: mock.Server.URL,
		TokenURL: mock.Server.URL + "/token",
	}

	// With force=false, should return existing credentials
	result, err := c.RefreshToken(context.Background(), oldCreds, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.AccessToken != "still-valid-access-token" {
		t.Errorf("expected original token, got: %s", result.AccessToken)
	}

	// With force=true, should refresh even though not expired
	result, err = c.RefreshToken(context.Background(), oldCreds, true)
	if err != nil {
		t.Fatal(err)
	}
	if result.AccessToken != "test-refreshed-access-token" {
		t.Errorf("expected refreshed token, got: %s", result.AccessToken)
	}
}

///////////////////////////////////////////////////////////////////////////////
// DISCOVERY TESTS

// TestDiscovery_RootOAuth tests that discovery finds metadata at the root
// RFC 8414 path (/.well-known/oauth-authorization-server).
func TestDiscovery_RootOAuth(t *testing.T) {
	mock := newMockOAuthServer(t)
	defer mock.Server.Close()

	// The standard mock serves at root — Login with OptClientCredentials exercises discovery
	c, err := httpclient.New(mock.Server.URL)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &oauth2.Config{ClientID: "test-client", ClientSecret: "test-secret", Endpoint: oauth2.Endpoint{AuthURL: mock.Server.URL}}
	creds, err := c.Login(context.Background(), cfg, httpclient.OptClientCredentials())
	if err != nil {
		t.Fatal(err)
	}
	if creds.AccessToken != "test-client-credentials-token" {
		t.Errorf("unexpected access token: %s", creds.AccessToken)
	}
}

// TestDiscovery_FallbackOIDC tests that discovery falls back to the
// OpenID Connect path (/.well-known/openid-configuration) when RFC 8414 returns 404.
func TestDiscovery_FallbackOIDC(t *testing.T) {
	metadata := &schema.OAuthMetadata{}

	mux := http.NewServeMux()
	// RFC 8414 path returns 404
	mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	// OIDC path returns metadata
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(metadata)
	})
	// Token endpoint
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "oidc-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	metadata.Issuer = server.URL
	metadata.TokenEndpoint = server.URL + "/token"
	metadata.GrantTypesSupported = []string{"client_credentials"}

	c, err := httpclient.New(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &oauth2.Config{ClientID: "test-client", ClientSecret: "test-secret", Endpoint: oauth2.Endpoint{AuthURL: server.URL}}
	creds, err := c.Login(context.Background(), cfg, httpclient.OptClientCredentials())
	if err != nil {
		t.Fatal(err)
	}
	if creds.AccessToken != "oidc-token" {
		t.Errorf("unexpected access token: %s", creds.AccessToken)
	}
}

// TestDiscovery_PathRelative tests that discovery finds metadata at a
// path-relative location (e.g., /realms/master/.well-known/...) when root returns 404.
func TestDiscovery_PathRelative(t *testing.T) {
	metadata := &schema.OAuthMetadata{}

	mux := http.NewServeMux()
	// Root paths return 404
	mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	// Path-relative discovery works (Keycloak-style)
	mux.HandleFunc("/realms/master/.well-known/oauth-authorization-server", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(metadata)
	})
	// Token endpoint
	mux.HandleFunc("/realms/master/protocol/openid-connect/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "keycloak-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	metadata.Issuer = server.URL + "/realms/master"
	metadata.TokenEndpoint = server.URL + "/realms/master/protocol/openid-connect/token"
	metadata.GrantTypesSupported = []string{"client_credentials"}

	c, err := httpclient.New(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	// Pass endpoint with path — discovery should find it at /realms/master/.well-known/...
	cfg := &oauth2.Config{ClientID: "test-client", ClientSecret: "test-secret", Endpoint: oauth2.Endpoint{AuthURL: server.URL + "/realms/master"}}
	creds, err := c.Login(context.Background(), cfg, httpclient.OptClientCredentials())
	if err != nil {
		t.Fatal(err)
	}
	if creds.AccessToken != "keycloak-token" {
		t.Errorf("unexpected access token: %s", creds.AccessToken)
	}
}

// TestDiscovery_NotFound tests that discovery returns a clear error when
// no well-known endpoint is available.
func TestDiscovery_NotFound(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()

	c, err := httpclient.New(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &oauth2.Config{ClientID: "test-client", ClientSecret: "test-secret", Endpoint: oauth2.Endpoint{AuthURL: server.URL}}
	_, err = c.Login(context.Background(), cfg, httpclient.OptClientCredentials())
	if err == nil {
		t.Fatal("expected error for server without OAuth support")
	}
	if !strings.Contains(err.Error(), "does not support OAuth discovery") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestDiscovery_ServerError tests that a non-404 error (e.g., 500) returns
// immediately without trying further candidates.
func TestDiscovery_ServerError(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer server.Close()

	c, err := httpclient.New(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &oauth2.Config{ClientID: "test-client", ClientSecret: "test-secret", Endpoint: oauth2.Endpoint{AuthURL: server.URL}}
	_, err = c.Login(context.Background(), cfg, httpclient.OptClientCredentials())
	if err == nil {
		t.Fatal("expected error for server returning 500")
	}
	if !strings.Contains(err.Error(), "OAuth discovery failed") {
		t.Errorf("unexpected error message: %v", err)
	}
	// Should have stopped after the first request (500 is not 404)
	if requestCount != 1 {
		t.Errorf("expected 1 request (early return on 500), got %d", requestCount)
	}
}
