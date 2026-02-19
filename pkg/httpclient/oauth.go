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
	"path"
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

// LoginOpt is a functional option for the Login method.
type LoginOpt func(*loginOpts)

type loginOpts struct {
	listener       net.Listener
	authCallback   AuthURLCallback
	deviceCallback DeviceAuthCallback
	clientName     string
	clientCreds    bool
}

// OptInteractive selects the Authorization Code flow with PKCE.
// The listener is used for the OAuth callback server, and the callback
// is invoked with the authorization URL for the user to visit.
func OptInteractive(listener net.Listener, callback AuthURLCallback) LoginOpt {
	return func(o *loginOpts) {
		o.listener = listener
		o.authCallback = callback
	}
}

// OptDevice selects the Device Authorization flow (RFC 8628).
// The callback is invoked with the verification URI and user code.
func OptDevice(callback DeviceAuthCallback) LoginOpt {
	return func(o *loginOpts) {
		o.deviceCallback = callback
	}
}

// OptClientCredentials selects the Client Credentials flow (RFC 6749 Section 4.4).
// The oauth2.Config must have ClientSecret set.
func OptClientCredentials() LoginOpt {
	return func(o *loginOpts) {
		o.clientCreds = true
	}
}

// OptClientName sets the client name for dynamic client registration (RFC 7591).
// If the oauth2.Config has an empty ClientID, registration is attempted
// using this name.
func OptClientName(name string) LoginOpt {
	return func(o *loginOpts) {
		o.clientName = name
	}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// NewCallbackListener creates a TCP listener for OAuth callbacks and returns
// both the listener and the redirect URI to use. If addr is empty, a random
// available port on localhost is used. Only loopback addresses are allowed
// for security reasons.
func NewCallbackListener(addr string) (net.Listener, string, error) {
	if addr == "" {
		addr = "127.0.0.1:0"
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

// Login performs an OAuth 2.0 login flow. It discovers OAuth metadata using
// cfg.Endpoint.AuthURL as the base server URL, then replaces cfg.Endpoint
// with the discovered authorization and token URLs.
//
// Flow is selected by the provided options:
//
//   - OptInteractive: Authorization Code flow with PKCE (default for user-facing apps)
//   - OptDevice: Device Authorization flow (RFC 8628)
//   - OptClientCredentials: Client Credentials flow (machine-to-machine)
//
// If cfg.ClientID is empty and OptClientName is set, dynamic client registration
// is attempted. cfg.ClientSecret is passed through for confidential clients.
func (c *Client) Login(ctx context.Context, cfg *oauth2.Config, opts ...LoginOpt) (*schema.OAuthCredentials, error) {
	// Apply options
	var o loginOpts
	for _, opt := range opts {
		opt(&o)
	}

	// Use AuthURL as the base server URL for discovery
	endpoint := cfg.Endpoint.AuthURL
	if endpoint == "" {
		return nil, fmt.Errorf("cfg.Endpoint.AuthURL must be set to the server URL")
	}

	// Discover OAuth metadata and replace the endpoint
	metadata, err := c.discoverOAuth(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	cfg.Endpoint = metadata.Endpoint()

	// Dispatch to the appropriate flow, auto-registering if needed
	var token *oauth2.Token

	switch {
	case o.listener != nil && o.authCallback != nil:
		// Interactive: Authorization Code with PKCE
		cfg.RedirectURL = fmt.Sprintf("http://%s/callback", o.listener.Addr().String())
		if cfg.ClientID == "" {
			if err := c.autoRegister(ctx, metadata, cfg, o.clientName,
				[]string{cfg.RedirectURL},
				[]string{"authorization_code", "refresh_token"},
				[]string{"code"},
				"none",
			); err != nil {
				return nil, err
			}
		}
		token, err = c.interactiveFlow(ctx, cfg, metadata, o.listener, o.authCallback)

	case o.deviceCallback != nil:
		// Device Authorization flow
		if !metadata.SupportsDeviceFlow() {
			return nil, fmt.Errorf("%s does not support device authorization flow", endpoint)
		}
		if cfg.ClientID == "" {
			if err := c.autoRegister(ctx, metadata, cfg, o.clientName,
				nil,
				[]string{"urn:ietf:params:oauth:grant-type:device_code", "refresh_token"},
				nil,
				"none",
			); err != nil {
				return nil, err
			}
		}
		token, err = c.deviceFlow(ctx, cfg, o.deviceCallback)

	case o.clientCreds:
		// Client Credentials flow
		if cfg.ClientSecret == "" {
			return nil, fmt.Errorf("client secret is required for client credentials flow")
		}
		if !metadata.SupportsGrantType("client_credentials") {
			return nil, fmt.Errorf("%s does not support client_credentials grant", endpoint)
		}
		// No auto-registration for client_credentials: it requires a
		// pre-registered confidential client with a secret.
		if cfg.ClientID == "" {
			return nil, fmt.Errorf("client-id is required for client credentials flow")
		}
		token, err = c.clientCredentialsFlow(ctx, cfg, metadata)

	default:
		return nil, fmt.Errorf("no login flow specified: use OptInteractive, OptDevice, or OptClientCredentials")
	}

	if err != nil {
		return nil, err
	}

	return &schema.OAuthCredentials{Token: token, ClientID: cfg.ClientID, ClientSecret: cfg.ClientSecret, Endpoint: endpoint, TokenURL: metadata.TokenEndpoint}, nil
}

// interactiveFlow performs the Authorization Code exchange with PKCE.
func (c *Client) interactiveFlow(ctx context.Context, cfg *oauth2.Config, metadata *schema.OAuthMetadata, listener net.Listener, callback AuthURLCallback) (*oauth2.Token, error) {
	// Generate PKCE verifier and state
	verifier := oauth2.GenerateVerifier()
	state, err := generateState()
	if err != nil {
		return nil, fmt.Errorf("failed to generate state: %w", err)
	}

	// Build authorization URL with PKCE challenge method based on server support
	var challengeOpts []oauth2.AuthCodeOption
	switch {
	case metadata.SupportsS256():
		challengeOpts = []oauth2.AuthCodeOption{oauth2.S256ChallengeOption(verifier)}
	case metadata.SupportsPKCE():
		// Server supports PKCE but not S256 — fall back to plain
		// RFC 7636 requires both code_challenge and code_challenge_method parameters
		challengeOpts = []oauth2.AuthCodeOption{
			oauth2.SetAuthURLParam("code_challenge", verifier),
			oauth2.SetAuthURLParam("code_challenge_method", "plain"),
		}
	default:
		// Server didn't advertise PKCE support — use S256 anyway (widely supported,
		// required by OAuth 2.1, and many servers omit code_challenge_methods_supported)
		challengeOpts = []oauth2.AuthCodeOption{oauth2.S256ChallengeOption(verifier)}
	}

	authURL := cfg.AuthCodeURL(state, challengeOpts...)
	callback(authURL)

	// Wait for authorization code via callback server
	code, err := c.waitForAuthCallback(ctx, listener, state)
	if err != nil {
		return nil, err
	}

	// Exchange code for token
	token, err := cfg.Exchange(c.oauthContext(ctx), code, oauth2.VerifierOption(verifier))
	if err != nil {
		return nil, fmt.Errorf("token exchange failed: %w", err)
	}
	return token, nil
}

// deviceFlow performs the Device Authorization exchange.
func (c *Client) deviceFlow(ctx context.Context, cfg *oauth2.Config, callback DeviceAuthCallback) (*oauth2.Token, error) {
	// Request device code
	deviceResp, err := cfg.DeviceAuth(c.oauthContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("device code request failed: %w", err)
	}

	// Notify caller of verification details
	callback(deviceResp.VerificationURI, deviceResp.UserCode)

	// Poll for token
	token, err := cfg.DeviceAccessToken(c.oauthContext(ctx), deviceResp)
	if err != nil {
		return nil, fmt.Errorf("device token exchange failed: %w", err)
	}
	return token, nil
}

// clientCredentialsFlow performs the Client Credentials exchange.
func (c *Client) clientCredentialsFlow(ctx context.Context, cfg *oauth2.Config, metadata *schema.OAuthMetadata) (*oauth2.Token, error) {
	ccCfg := &clientcredentials.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		TokenURL:     metadata.TokenEndpoint,
		Scopes:       cfg.Scopes,
	}
	token, err := ccCfg.Token(c.oauthContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("client credentials exchange failed: %w", err)
	}
	return token, nil
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
		ClientID:     creds.ClientID,
		ClientSecret: creds.ClientSecret,
		Endpoint:     oauth2.Endpoint{TokenURL: creds.TokenURL},
	}

	// Use a token copy with an expired time to force the oauth2 library to refresh
	tok := *creds.Token
	tok.Expiry = time.Now().Add(-time.Minute)

	// Refresh the token
	newToken, err := cfg.TokenSource(c.oauthContext(ctx), &tok).Token()
	if err != nil {
		return nil, fmt.Errorf("token refresh failed: %w", err)
	}
	return &schema.OAuthCredentials{Token: newToken, ClientID: creds.ClientID, ClientSecret: creds.ClientSecret, Endpoint: creds.Endpoint, TokenURL: creds.TokenURL}, nil
}

// autoRegister performs dynamic client registration if no ClientID is set.
// It validates the client name and delegates to registerClient with flow-specific parameters.
func (c *Client) autoRegister(ctx context.Context, metadata *schema.OAuthMetadata, cfg *oauth2.Config, clientName string, redirectURIs, grantTypes, responseTypes []string, authMethod string) error {
	if clientName == "" {
		return fmt.Errorf("either client-id or client-name must be provided")
	}
	clientInfo, err := c.registerClient(ctx, metadata, clientName, redirectURIs, cfg.Scopes, grantTypes, responseTypes, authMethod)
	if err != nil {
		return fmt.Errorf("dynamic client registration failed (you may need to register manually and use --client-id): %w", err)
	}
	cfg.ClientID = clientInfo.ClientID
	cfg.ClientSecret = clientInfo.ClientSecret
	return nil
}

// registerClient performs dynamic client registration (RFC 7591).
// It registers a new OAuth client with the authorization server and returns the client info.
func (c *Client) registerClient(ctx context.Context, metadata *schema.OAuthMetadata, clientName string, redirectURIs []string, scopes []string, grantTypes []string, responseTypes []string, authMethod string) (*schema.OAuthClientInfo, error) {
	if !metadata.SupportsRegistration() {
		return nil, fmt.Errorf("%s does not support dynamic client registration", metadata.Issuer)
	}

	// Build registration request
	regReq := &schema.OAuthClientRegistration{
		ClientName:              clientName,
		RedirectURIs:            redirectURIs,
		GrantTypes:              grantTypes,
		ResponseTypes:           responseTypes,
		TokenEndpointAuthMethod: authMethod,
		Scope:                   strings.Join(scopes, " "),
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
			"status":  "ok",
			"message": "Authorization code received. You can close this window.",
		})
	})

	// Create server
	server := &http.Server{Handler: mux}

	// WaitGroup to ensure server goroutine completes
	var wg sync.WaitGroup
	wg.Go(func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			sendResult(authResult{err: fmt.Errorf("callback server failed: %w", err)})
		}
	})

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
	base := fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	suffixes := []string{schema.OAuthWellKnownPath, schema.OIDCWellKnownPath}
	candidates := make([]string, 0, len(suffixes)*4)
	for _, suffix := range suffixes {
		candidates = append(candidates, base+suffix) // root: /.well-known/...
	}

	// Add path-relative candidates walking up parent segments,
	// starting from the parent of the resource path.
	// For /realms/master/protocol/sse we try:
	//   /realms/master/protocol/.well-known/...
	//   /realms/master/.well-known/...
	//   /realms/.well-known/...
	basePath := path.Dir(strings.TrimRight(u.Path, "/"))
	for basePath != "" && basePath != "/" && basePath != "." {
		for _, suffix := range suffixes {
			candidates = append(candidates, base+basePath+suffix)
		}
		basePath = path.Dir(basePath)
	}

	// Iterate over candidates and return the first successful metadata response
	for _, candidateURL := range candidates {
		var metadata schema.OAuthMetadata
		if err := c.DoWithContext(ctx, nil, &metadata, client.OptReqEndpoint(candidateURL)); err != nil {
			// Certain HTTP status codes indicate the well-known path doesn't
			// exist at this location — skip and try the next candidate.
			// 401/403 are included because misconfigured auth middleware
			// sometimes guards non-existent paths.
			var httpErr httpresponse.Err
			if errors.As(err, &httpErr) {
				switch int(httpErr) {
				case http.StatusNotFound, http.StatusUnauthorized,
					http.StatusForbidden, http.StatusMethodNotAllowed:
					continue
				}
			}
			// Any other error (network, 500, etc.) is fatal
			return nil, fmt.Errorf("%s: OAuth discovery failed: %w", endpoint, err)
		}
		return &metadata, nil
	}

	// Return error: couldn't discover metadata from any candidate URL
	return nil, fmt.Errorf("%s does not support OAuth discovery", endpoint)
}
