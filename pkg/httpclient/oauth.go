package httpclient

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	// Packages
	client "github.com/mutablelogic/go-client"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	oauth2 "golang.org/x/oauth2"
	clientcredentials "golang.org/x/oauth2/clientcredentials"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// AuthURLCallback is called with the authorization URL for interactive login.
// The callback should present this URL to the user (e.g., open browser, display).
type AuthURLCallback func(authURL string)

// DeviceAuthCallback is called with the device authorization details.
// The callback should present the verification URI and user code to the user.
type DeviceAuthCallback func(verificationURI, userCode string)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// NewCallbackListener creates a TCP listener for OAuth callbacks and returns
// both the listener and the redirect URI to use. If addr is empty, a random
// available port on localhost is used. Only loopback addresses are allowed
// for security reasons.
func NewCallbackListener(addr string) (net.Listener, string, error) {
	if addr == "" {
		addr = "localhost:0"
	}

	// Parse and validate the address
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, "", fmt.Errorf("invalid callback address %q: %w", addr, err)
	}

	// Validate loopback only
	if !isLoopback(host) {
		return nil, "", fmt.Errorf("callback address must be loopback (localhost/127.0.0.1/::1), got %q", host)
	}

	// Validate port is present (can be "0" for random)
	if port == "" {
		return nil, "", fmt.Errorf("callback address %q missing port", addr)
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, "", fmt.Errorf("failed to start callback server on %s: %w", addr, err)
	}
	redirectURI := fmt.Sprintf("http://%s/callback", listener.Addr().String())
	return listener, redirectURI, nil
}

// InteractiveLogin performs an OAuth 2.0 Authorization Code flow with PKCE.
// It discovers the OAuth metadata from the endpoint, uses the provided listener
// for callbacks, presents the authorization URL to the user via callback,
// waits for the callback, and exchanges the code for a token.
// If clientID is empty and clientName is non-empty, it attempts dynamic client
// registration. The caller is responsible for closing the listener after this returns.
func (c *Client) InteractiveLogin(ctx context.Context, endpoint, clientID, clientName string, scopes []string, listener net.Listener, callback AuthURLCallback) (*schema.OAuthCredentials, error) {
	// Discover OAuth metadata
	metadata, err := c.discoverOAuth(ctx, endpoint)
	if err != nil {
		return nil, err
	}

	// Derive redirect URI from listener
	redirectURI := fmt.Sprintf("http://%s/callback", listener.Addr().String())

	// Auto-register if no client ID provided
	if clientID == "" {
		if clientName == "" {
			return nil, fmt.Errorf("either client-id or client-name must be provided")
		}
		clientInfo, err := c.registerClient(ctx, metadata, clientName, []string{redirectURI})
		if err != nil {
			return nil, fmt.Errorf("dynamic client registration failed (you may need to register manually and use --client-id): %w", err)
		}
		clientID = clientInfo.ClientID
	}

	// Create OAuth2 config
	cfg := &oauth2.Config{
		ClientID:    clientID,
		Scopes:      scopes,
		Endpoint:    metadata.Endpoint(),
		RedirectURL: redirectURI,
	}

	// Generate PKCE verifier and state
	verifier := oauth2.GenerateVerifier()
	state, err := generateState()
	if err != nil {
		return nil, fmt.Errorf("failed to generate state: %w", err)
	}

	// Build authorization URL
	authURL := cfg.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier))

	// Notify caller of the URL
	callback(authURL)

	// Wait for authorization code via callback server
	code, err := c.waitForAuthCallback(ctx, listener, state)
	if err != nil {
		return nil, err
	}

	// Exchange code for token (use our HTTP client)
	token, err := cfg.Exchange(c.oauthContext(ctx), code, oauth2.VerifierOption(verifier))
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}
	return &schema.OAuthCredentials{Token: token, ClientID: clientID, Endpoint: endpoint, TokenURL: metadata.TokenEndpoint}, nil
}

// RefreshToken exchanges a refresh token for a new access token.
// If force is false and the token is still valid (with a 30-second buffer),
// the existing credentials are returned as-is. The provided token must
// contain a valid refresh token.
func (c *Client) RefreshToken(ctx context.Context, creds *schema.OAuthCredentials, force bool) (*schema.OAuthCredentials, error) {
	if creds.RefreshToken == "" {
		return nil, fmt.Errorf("token does not contain a refresh token")
	} else if creds.TokenURL == "" {
		return nil, fmt.Errorf("credentials missing token URL")
	}

	// If not forcing, return existing credentials if the token is still valid
	if !force && !creds.Expiry.IsZero() && time.Until(creds.Expiry) > 30*time.Second {
		return creds, nil
	}

	// Create OAuth2 config using stored token URL (no discovery needed)
	cfg := &oauth2.Config{
		ClientID: creds.ClientID,
		Endpoint: oauth2.Endpoint{TokenURL: creds.TokenURL},
	}

	// Use a token copy with an expired time to force the oauth2 library to refresh
	tok := *creds.Token
	tok.Expiry = time.Now().Add(-time.Minute)

	// Refresh the token
	newToken, err := cfg.TokenSource(c.oauthContext(ctx), &tok).Token()
	if err != nil {
		return nil, fmt.Errorf("token refresh failed: %w", err)
	}
	return &schema.OAuthCredentials{Token: newToken, ClientID: creds.ClientID, Endpoint: creds.Endpoint, TokenURL: creds.TokenURL}, nil
}

// DeviceLogin performs an OAuth 2.0 Device Authorization flow (RFC 8628).
// It requests a device code, provides the verification URL and code via callback,
// then polls until the user completes authorization.
// If clientID is empty and clientName is non-empty, it attempts dynamic client
// registration.
func (c *Client) DeviceLogin(ctx context.Context, endpoint, clientID, clientName string, scopes []string, callback DeviceAuthCallback) (*schema.OAuthCredentials, error) {
	// Discover OAuth metadata
	metadata, err := c.discoverOAuth(ctx, endpoint)
	if err != nil {
		return nil, err
	}

	// Check if device flow is supported
	if !metadata.SupportsDeviceFlow() {
		return nil, fmt.Errorf("%s does not support device authorization flow", endpoint)
	}

	// Auto-register if no client ID provided
	if clientID == "" {
		if clientName == "" {
			return nil, fmt.Errorf("either client-id or client-name must be provided")
		}
		clientInfo, err := c.registerClient(ctx, metadata, clientName, nil)
		if err != nil {
			return nil, fmt.Errorf("dynamic client registration failed (you may need to register manually and use --client-id): %w", err)
		}
		clientID = clientInfo.ClientID
	}

	// Create OAuth2 config
	cfg := &oauth2.Config{
		ClientID: clientID,
		Scopes:   scopes,
		Endpoint: metadata.Endpoint(),
	}

	// Request device code (use our HTTP client)
	deviceResp, err := cfg.DeviceAuth(c.oauthContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("device code request failed: %w", err)
	}

	// Notify caller of verification details
	callback(deviceResp.VerificationURI, deviceResp.UserCode)

	// Poll for token (oauth2 handles polling internally, use our HTTP client)
	token, err := cfg.DeviceAccessToken(c.oauthContext(ctx), deviceResp)
	if err != nil {
		return nil, fmt.Errorf("device token exchange failed: %w", err)
	}
	return &schema.OAuthCredentials{Token: token, ClientID: clientID, Endpoint: endpoint, TokenURL: metadata.TokenEndpoint}, nil
}

// ClientCredentialsLogin performs an OAuth 2.0 Client Credentials flow (RFC 6749 Section 4.4).
// This is used for machine-to-machine authentication where no user is involved.
func (c *Client) ClientCredentialsLogin(ctx context.Context, endpoint, clientID, clientSecret string, scopes []string) (*schema.OAuthCredentials, error) {
	// Discover OAuth metadata
	metadata, err := c.discoverOAuth(ctx, endpoint)
	if err != nil {
		return nil, err
	}

	// Check if client_credentials grant is supported
	if !metadata.SupportsGrantType("client_credentials") {
		return nil, fmt.Errorf("%s does not support client_credentials grant", endpoint)
	}

	// Create client credentials config
	cfg := &clientcredentials.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		TokenURL:     metadata.TokenEndpoint,
		Scopes:       scopes,
	}

	// Get token (use our HTTP client)
	token, err := cfg.Token(c.oauthContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("client credentials exchange failed: %w", err)
	}

	return &schema.OAuthCredentials{Token: token, ClientID: clientID, Endpoint: endpoint, TokenURL: metadata.TokenEndpoint}, nil
}

// registerClient performs dynamic client registration (RFC 7591).
// It registers a new OAuth client with the authorization server and returns the client info.
func (c *Client) registerClient(ctx context.Context, metadata *schema.OAuthMetadata, clientName string, redirectURIs []string) (*schema.OAuthClientInfo, error) {
	if !metadata.SupportsRegistration() {
		return nil, fmt.Errorf("%s does not support dynamic client registration", metadata.Issuer)
	}

	// Build registration request
	regReq := &schema.OAuthClientRegistration{
		ClientName:              clientName,
		RedirectURIs:            redirectURIs,
		GrantTypes:              []string{"authorization_code", "refresh_token"},
		ResponseTypes:           []string{"code"},
		TokenEndpointAuthMethod: "none", // Public client (no secret)
	}

	// Create JSON request payload
	payload, err := client.NewJSONRequest(regReq)
	if err != nil {
		return nil, err
	}

	// Send registration request
	var clientInfo schema.OAuthClientInfo
	if err := c.DoWithContext(ctx, payload, &clientInfo, client.OptReqEndpoint(metadata.RegistrationEndpoint)); err != nil {
		return nil, err
	}

	return &clientInfo, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// oauthContext returns a context with our HTTP client injected for oauth2 library use.
func (c *Client) oauthContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, oauth2.HTTPClient, c.Client.Client)
}

// generateState creates a random state string for CSRF protection.
func generateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// isLoopback returns true if the host is a loopback address.
func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// authResult holds the result from the OAuth callback handler.
type authResult struct {
	code string
	err  error
}

// waitForAuthCallback starts an HTTP server on the given listener, waits for
// an OAuth callback with the expected state, and returns the authorization code.
// It properly shuts down the server and waits for all goroutines to complete.
func (c *Client) waitForAuthCallback(ctx context.Context, listener net.Listener, expectedState string) (string, error) {
	resultCh := make(chan authResult, 1)
	var once sync.Once

	// sendResult sends a result to the channel exactly once, preventing
	// duplicate callbacks from blocking handler goroutines.
	sendResult := func(r authResult) {
		once.Do(func() {
			resultCh <- r
		})
	}

	// Set up callback handler
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		// Verify state
		if r.URL.Query().Get("state") != expectedState {
			sendResult(authResult{err: fmt.Errorf("state mismatch")})
			_ = httpresponse.Error(w, httpresponse.ErrBadRequest.With("state mismatch"))
			return
		}

		// Check for error from authorization server
		if errParam := r.URL.Query().Get("error"); errParam != "" {
			errDesc := r.URL.Query().Get("error_description")
			sendResult(authResult{err: fmt.Errorf("authorization error: %s: %s", errParam, errDesc)})
			_ = httpresponse.Error(w, httpresponse.ErrBadRequest.With(errDesc))
			return
		}

		// Get authorization code
		code := r.URL.Query().Get("code")
		if code == "" {
			sendResult(authResult{err: fmt.Errorf("no authorization code received")})
			_ = httpresponse.Error(w, httpresponse.ErrBadRequest.With("no authorization code received"))
			return
		}

		sendResult(authResult{code: code})
		_ = httpresponse.JSON(w, http.StatusOK, 0, map[string]string{
			"status":  "success",
			"message": "Authentication successful! You can close this window.",
		})
	})

	// Create server
	server := &http.Server{Handler: mux}

	// WaitGroup to ensure server goroutine completes
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			sendResult(authResult{err: fmt.Errorf("callback server failed: %w", err)})
		}
	}()

	// Wait for callback or context cancellation
	var result authResult
	select {
	case <-ctx.Done():
		result = authResult{err: ctx.Err()}
	case result = <-resultCh:
	}

	// Shutdown server and wait for goroutine to complete
	_ = server.Shutdown(context.Background())
	wg.Wait()

	if result.err != nil {
		return "", result.err
	}
	return result.code, nil
}

// discoverOAuth fetches OAuth 2.0 Authorization Server Metadata from the
// well-known endpoint on the server. It tries RFC 8414 root paths first,
// then falls back to path-relative discovery (e.g., Keycloak realms).
func (c *Client) discoverOAuth(ctx context.Context, endpoint string) (*schema.OAuthMetadata, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	} else {
		u.RawQuery = ""
		u.Fragment = ""
	}

	// Build candidate URLs: root-based (RFC 8414) first, then path-relative (Keycloak)
	suffixes := []string{schema.OAuthWellKnownPath, schema.OIDCWellKnownPath}
	candidates := make([]string, 0, len(suffixes)*2)
	for _, suffix := range suffixes {
		candidates = append(candidates, suffix) // root: /.well-known/...
	}

	// Set base path for path-relative discovery (e.g., /realms/master/.well-known/...)
	basePath := strings.TrimRight(u.Path, "/")
	if basePath != "" {
		for _, suffix := range suffixes {
			candidates = append(candidates, basePath+suffix) // relative: /realms/master/.well-known/...
		}
	}

	// Iterate over candidates and return the first successful metadata response
	for _, path := range candidates {
		u.Path = path
		var metadata schema.OAuthMetadata
		if err := c.DoWithContext(ctx, nil, &metadata, client.OptReqEndpoint(u.String())); err != nil {
			// 404 means this path doesn't exist, try the next candidate
			var httpErr httpresponse.Err
			if errors.As(err, &httpErr) && int(httpErr) == http.StatusNotFound {
				continue
			}
			// Any other error (network, 500, etc.) is fatal
			return nil, fmt.Errorf("%s: OAuth discovery failed: %w", endpoint, err)
		}
		return &metadata, nil
	}

	// Return error: couldn't discover metadata from any candidate URL
	return nil, fmt.Errorf("%s does not support OAuth discovery", endpoint)
}
