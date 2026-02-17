package httpclient

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"net/url"

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

// InteractiveLogin performs an OAuth 2.0 Authorization Code flow with PKCE.
// It discovers the OAuth metadata from the endpoint, starts a local callback server,
// presents the authorization URL to the user via callback, waits for the callback, and exchanges
// the code for a token.
// If redirectURI is empty, a random local port is used.
func (c *Client) InteractiveLogin(ctx context.Context, endpoint, clientID string, scopes []string, redirectURI string, callback AuthURLCallback) (*oauth2.Token, error) {
	// Discover OAuth metadata
	metadata, err := c.DiscoverOAuth(ctx, endpoint)
	if err != nil {
		return nil, err
	}

	// Parse redirect URI to get the host:port for listener
	var listener net.Listener
	if redirectURI == "" {
		// Start local callback server on random port
		listener, err = net.Listen("tcp", "localhost:0")
		if err != nil {
			return nil, fmt.Errorf("failed to start callback server: %w", err)
		}
		redirectURI = fmt.Sprintf("http://%s/callback", listener.Addr().String())
	} else {
		// Parse the provided redirect URI and bind to that address
		u, err := url.Parse(redirectURI)
		if err != nil {
			return nil, fmt.Errorf("invalid redirect URI: %w", err)
		}
		listener, err = net.Listen("tcp", u.Host)
		if err != nil {
			return nil, fmt.Errorf("failed to start callback server on %s: %w", u.Host, err)
		}
	}
	defer listener.Close()

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

	// Channel to receive the authorization code or error
	type authResult struct {
		code string
		err  error
	}
	resultCh := make(chan authResult, 1)

	// Set up callback handler
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		// Verify state
		if r.URL.Query().Get("state") != state {
			resultCh <- authResult{err: fmt.Errorf("state mismatch")}
			_ = httpresponse.Error(w, httpresponse.ErrBadRequest.With("state mismatch"))
			return
		}

		// Check for error
		if errParam := r.URL.Query().Get("error"); errParam != "" {
			errDesc := r.URL.Query().Get("error_description")
			resultCh <- authResult{err: fmt.Errorf("authorization error: %s: %s", errParam, errDesc)}
			_ = httpresponse.Error(w, httpresponse.ErrBadRequest.With(errDesc))
			return
		}

		// Get authorization code
		code := r.URL.Query().Get("code")
		if code == "" {
			resultCh <- authResult{err: fmt.Errorf("no authorization code received")}
			_ = httpresponse.Error(w, httpresponse.ErrBadRequest.With("no authorization code received"))
			return
		}

		resultCh <- authResult{code: code}
		_ = httpresponse.JSON(w, http.StatusOK, 0, map[string]string{
			"status":  "success",
			"message": "Authentication successful! You can close this window.",
		})
	})

	// Start server in goroutine
	server := &http.Server{Handler: mux}
	go func() {
		server.Serve(listener)
	}()
	defer server.Shutdown(context.Background())

	// Wait for callback or context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-resultCh:
		if result.err != nil {
			return nil, result.err
		}

		// Exchange code for token (use our HTTP client)
		token, err := cfg.Exchange(c.oauthContext(ctx), result.code, oauth2.VerifierOption(verifier))
		if err != nil {
			return nil, fmt.Errorf("token exchange failed: %w", err)
		}
		return token, nil
	}
}

// DeviceLogin performs an OAuth 2.0 Device Authorization flow (RFC 8628).
// It requests a device code, provides the verification URL and code via callback,
// then polls until the user completes authorization.
func (c *Client) DeviceLogin(ctx context.Context, endpoint, clientID string, scopes []string, callback DeviceAuthCallback) (*oauth2.Token, error) {
	// Discover OAuth metadata
	metadata, err := c.DiscoverOAuth(ctx, endpoint)
	if err != nil {
		return nil, err
	}

	// Check if device flow is supported
	if !metadata.SupportsDeviceFlow() {
		return nil, fmt.Errorf("%s does not support device authorization flow", endpoint)
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
	return token, nil
}

// ClientCredentialsLogin performs an OAuth 2.0 Client Credentials flow (RFC 6749 Section 4.4).
// This is used for machine-to-machine authentication where no user is involved.
func (c *Client) ClientCredentialsLogin(ctx context.Context, endpoint, clientID, clientSecret string, scopes []string) (*oauth2.Token, error) {
	// Discover OAuth metadata
	metadata, err := c.DiscoverOAuth(ctx, endpoint)
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

	return token, nil
}

// RegisterClient performs dynamic client registration (RFC 7591).
// It registers a new OAuth client with the authorization server and returns the client info.
func (c *Client) RegisterClient(ctx context.Context, metadata *schema.OAuthMetadata, clientName string, redirectURIs []string) (*schema.OAuthClientInfo, error) {
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

// DiscoverOAuth fetches OAuth 2.0 Authorization Server Metadata from the
// well-known endpoint on the server.
func (c *Client) DiscoverOAuth(ctx context.Context, endpoint string) (*schema.OAuthMetadata, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	u.Path = schema.OAuthWellKnownPath
	var metadata schema.OAuthMetadata
	if err := c.DoWithContext(ctx, nil, &metadata, client.OptReqEndpoint(u.String())); err != nil {
		return nil, fmt.Errorf("%s does not support OAuth discovery at %s: %w", endpoint, u.String(), err)
	}
	return &metadata, nil
}
