package client

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// OAuthMetadata represents the OAuth 2.0 Authorization Server Metadata
// returned from the well-known discovery endpoint (RFC 8414).
type OAuthMetadata struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	RegistrationEndpoint              string   `json:"registration_endpoint,omitempty"`
	RevocationEndpoint                string   `json:"revocation_endpoint,omitempty"`
	ResponseTypesSupported            []string `json:"response_types_supported,omitempty"`
	ResponseModesSupported            []string `json:"response_modes_supported,omitempty"`
	GrantTypesSupported               []string `json:"grant_types_supported,omitempty"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported,omitempty"`
	CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported,omitempty"`
	ScopesSupported                   []string `json:"scopes_supported,omitempty"`
}

// OAuthRegistration represents the response from dynamic client registration (RFC 7591).
type OAuthRegistration struct {
	ClientID                string   `json:"client_id"`
	ClientSecret            string   `json:"client_secret,omitempty"`
	ClientIDIssuedAt        int64    `json:"client_id_issued_at,omitempty"`
	ClientSecretExpiresAt   int64    `json:"client_secret_expires_at,omitempty"`
	RedirectURIs            []string `json:"redirect_uris,omitempty"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method,omitempty"`
	GrantTypes              []string `json:"grant_types,omitempty"`
	ResponseTypes           []string `json:"response_types,omitempty"`
	ClientName              string   `json:"client_name,omitempty"`
}

// OAuthToken represents the response from the token endpoint.
type OAuthToken struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

// PKCEChallenge holds a PKCE code verifier and its S256 challenge.
type PKCEChallenge struct {
	Verifier  string
	Challenge string
	Method    string // "S256"
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// DiscoverOAuth fetches the OAuth 2.0 Authorization Server Metadata for the
// given MCP server URL. It looks for the well-known endpoint relative to the
// server's origin (per MCP spec / RFC 8414).
func DiscoverOAuth(ctx context.Context, serverURL string) (*OAuthMetadata, error) {
	// Parse the server URL to get the origin
	u, err := url.Parse(serverURL)
	if err != nil {
		return nil, fmt.Errorf("invalid server URL: %w", err)
	}

	// Build the well-known URL: origin + /.well-known/oauth-authorization-server
	wellKnown := fmt.Sprintf("%s://%s/.well-known/oauth-authorization-server", u.Scheme, u.Host)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, wellKnown, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OAuth discovery failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OAuth discovery returned %s", resp.Status)
	}

	var meta OAuthMetadata
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return nil, fmt.Errorf("OAuth discovery: invalid response: %w", err)
	}

	// Validate required fields
	if meta.AuthorizationEndpoint == "" {
		return nil, fmt.Errorf("OAuth discovery: missing authorization_endpoint")
	}
	if meta.TokenEndpoint == "" {
		return nil, fmt.Errorf("OAuth discovery: missing token_endpoint")
	}

	return &meta, nil
}

// Register performs OAuth 2.0 Dynamic Client Registration (RFC 7591) using the
// registration endpoint from the metadata. Returns the registered client details.
func (m *OAuthMetadata) Register(ctx context.Context, clientName string, redirectURIs []string) (*OAuthRegistration, error) {
	if m.RegistrationEndpoint == "" {
		return nil, fmt.Errorf("server does not support dynamic client registration")
	}

	// Build registration request
	body := map[string]any{
		"client_name":                clientName,
		"redirect_uris":              redirectURIs,
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
		"token_endpoint_auth_method": "none",
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.RegistrationEndpoint, strings.NewReader(string(data)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("client registration failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("client registration returned %s", resp.Status)
	}

	var reg OAuthRegistration
	if err := json.NewDecoder(resp.Body).Decode(&reg); err != nil {
		return nil, fmt.Errorf("client registration: invalid response: %w", err)
	}
	if reg.ClientID == "" {
		return nil, fmt.Errorf("client registration: missing client_id")
	}

	return &reg, nil
}

// AuthorizationURL builds the authorization URL for the OAuth authorization code flow
// with PKCE. The caller should open this URL in a browser.
func (m *OAuthMetadata) AuthorizationURL(clientID, redirectURI string, pkce *PKCEChallenge, scopes ...string) string {
	params := url.Values{
		"response_type":         {"code"},
		"client_id":             {clientID},
		"redirect_uri":          {redirectURI},
		"code_challenge":        {pkce.Challenge},
		"code_challenge_method": {pkce.Method},
	}
	if len(scopes) > 0 {
		params.Set("scope", strings.Join(scopes, " "))
	}
	return m.AuthorizationEndpoint + "?" + params.Encode()
}

// ExchangeCode exchanges an authorization code for tokens using the token endpoint.
func (m *OAuthMetadata) ExchangeCode(ctx context.Context, clientID, code, redirectURI string, pkce *PKCEChallenge) (*OAuthToken, error) {
	data := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {clientID},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"code_verifier": {pkce.Verifier},
	}

	return m.tokenRequest(ctx, data)
}

// RefreshToken exchanges a refresh token for a new access token.
func (m *OAuthMetadata) RefreshToken(ctx context.Context, clientID, refreshToken string) (*OAuthToken, error) {
	data := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {clientID},
		"refresh_token": {refreshToken},
	}

	return m.tokenRequest(ctx, data)
}

// SupportsS256 returns true if the server supports S256 PKCE challenges.
func (m *OAuthMetadata) SupportsS256() bool {
	for _, method := range m.CodeChallengeMethodsSupported {
		if method == "S256" {
			return true
		}
	}
	return false
}

// SupportsRegistration returns true if the server supports dynamic client registration.
func (m *OAuthMetadata) SupportsRegistration() bool {
	return m.RegistrationEndpoint != ""
}

// NewPKCEChallenge generates a new PKCE code verifier and S256 challenge.
func NewPKCEChallenge() (*PKCEChallenge, error) {
	// Generate 32 random bytes for the verifier
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("failed to generate PKCE verifier: %w", err)
	}

	verifier := base64.RawURLEncoding.EncodeToString(b)

	// S256: BASE64URL(SHA256(verifier))
	hash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(hash[:])

	return &PKCEChallenge{
		Verifier:  verifier,
		Challenge: challenge,
		Method:    "S256",
	}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func (m *OAuthMetadata) tokenRequest(ctx context.Context, data url.Values) (*OAuthToken, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.TokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Try to read error response
		var errResp map[string]any
		json.NewDecoder(resp.Body).Decode(&errResp)
		if desc, ok := errResp["error_description"]; ok {
			return nil, fmt.Errorf("token request failed: %s: %v", resp.Status, desc)
		}
		if e, ok := errResp["error"]; ok {
			return nil, fmt.Errorf("token request failed: %s: %v", resp.Status, e)
		}
		return nil, fmt.Errorf("token request failed: %s", resp.Status)
	}

	var token OAuthToken
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, fmt.Errorf("token response: invalid JSON: %w", err)
	}
	if token.AccessToken == "" {
		return nil, fmt.Errorf("token response: missing access_token")
	}

	return &token, nil
}
