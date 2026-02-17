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

///////////////////////////////////////////////////////////////////////////////
// TESTS

func TestDiscoverOAuth(t *testing.T) {
	mock := newMockOAuthServer(t)
	defer mock.Server.Close()

	c, err := httpclient.New(mock.Server.URL)
	if err != nil {
		t.Fatal(err)
	}

	metadata, err := c.DiscoverOAuth(context.Background(), mock.Server.URL)
	if err != nil {
		t.Fatal(err)
	}

	if metadata.Issuer != mock.Server.URL {
		t.Errorf("expected issuer %s, got %s", mock.Server.URL, metadata.Issuer)
	}
	if metadata.AuthorizationEndpoint != mock.Server.URL+"/authorize" {
		t.Errorf("unexpected authorization endpoint: %s", metadata.AuthorizationEndpoint)
	}
	if !metadata.SupportsDeviceFlow() {
		t.Error("expected device flow to be supported")
	}
	if !metadata.SupportsRegistration() {
		t.Error("expected registration to be supported")
	}
	if !metadata.SupportsPKCE() {
		t.Error("expected PKCE to be supported")
	}
}

func TestDiscoverOAuth_NotFound(t *testing.T) {
	// Server without OAuth support
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()

	c, err := httpclient.New(server.URL)
	if err != nil {
		t.Fatal(err)
	}

	_, err = c.DiscoverOAuth(context.Background(), server.URL)
	if err == nil {
		t.Fatal("expected error for server without OAuth support")
	}
	if !strings.Contains(err.Error(), "does not support OAuth discovery") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRegisterClient(t *testing.T) {
	mock := newMockOAuthServer(t)
	defer mock.Server.Close()

	c, err := httpclient.New(mock.Server.URL)
	if err != nil {
		t.Fatal(err)
	}

	metadata, err := c.DiscoverOAuth(context.Background(), mock.Server.URL)
	if err != nil {
		t.Fatal(err)
	}

	clientInfo, err := c.RegisterClient(context.Background(), metadata, "test-client", []string{"http://localhost:8080/callback"})
	if err != nil {
		t.Fatal(err)
	}

	if clientInfo.ClientID != "registered-client-id" {
		t.Errorf("unexpected client ID: %s", clientInfo.ClientID)
	}
	if clientInfo.ClientName != "test-client" {
		t.Errorf("unexpected client name: %s", clientInfo.ClientName)
	}
}

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

	token, err := c.InteractiveLogin(ctx, mock.Server.URL, "test-client", []string{"openid"}, listener, func(url string) {
		authURL = url
	})
	if err != nil {
		t.Fatal(err)
	}

	if token.AccessToken != "test-access-token" {
		t.Errorf("unexpected access token: %s", token.AccessToken)
	}
	if token.RefreshToken != "test-refresh-token" {
		t.Errorf("unexpected refresh token: %s", token.RefreshToken)
	}

	_ = redirectURI // Used in registration
}

func TestClientCredentialsLogin(t *testing.T) {
	mock := newMockOAuthServer(t)
	defer mock.Server.Close()

	c, err := httpclient.New(mock.Server.URL)
	if err != nil {
		t.Fatal(err)
	}

	token, err := c.ClientCredentialsLogin(context.Background(), mock.Server.URL, "test-client", "test-secret", []string{"api"})
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
	token, err := c.DeviceLogin(ctx, mock.Server.URL, "test-client", []string{"openid"}, func(uri, code string) {
		verificationURI = uri
		userCode = code
	})
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
