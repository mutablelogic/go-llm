package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"

	// Packages
	goclient "github.com/mutablelogic/go-client"
	"github.com/mutablelogic/go-client/pkg/oauth"
	"golang.org/x/oauth2"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// OOBRedirectURI is the out-of-band redirect URI for native/CLI OAuth apps.
// When used, the authorization server displays the code in the browser rather
// than redirecting to a local callback server.
const OOBRedirectURI = "urn:ietf:wg:oauth:2.0:oob"

// Authorize authenticates the client against the MCP server using OAuth2.
//
// endpoint is the base URL of the MCP server used for discovery.
// clientID and clientSecret are the pre-registered app credentials.
// clientName is used for dynamic registration when clientID is empty.
// callbackPort is the TCP port for the loopback OAuth callback (0 = random).
// oob selects the out-of-band flow: the authorization URL is opened in the
// browser, the server displays the code, and the user pastes it into the
// terminal.
//
// On success the client's internal *http.Client is replaced with one that
// sends Authorization: Bearer <token> on every request; this client is then
// used by Connect to authenticate MCP transport requests.
func (c *Client) Authorize(ctx context.Context, endpoint, clientID, clientSecret, clientName string, callbackPort int, oob bool, openFn oauth.OpenFunc) error {
	gcOpts := []goclient.ClientOpt{goclient.OptEndpoint(endpoint)}
	if c.trace != nil {
		gcOpts = append(gcOpts, goclient.OptTrace(c.trace, true))
	}
	gc, err := goclient.New(gcOpts...)
	if err != nil {
		return fmt.Errorf("oauth client: %w", err)
	}
	flow := gc.OAuth(ctx)

	// Discover the OAuth2 authorization server metadata.
	// Primary: try discovery on the endpoint itself.
	// Fallback (MCP spec §Authentication): if the server sent a WWW-Authenticate
	// header containing resource_metadata (RFC 9728), fetch that document to find
	// the actual authorization server URL and discover there instead.
	metadata, err := flow.Discover(endpoint)
	if err != nil && c.wwwAuthenticate != "" {
		authServerURL, rmErr := c.resolveAuthServerFromHeader(ctx, c.wwwAuthenticate)
		if rmErr == nil {
			metadata, err = flow.Discover(authServerURL)
			if err != nil {
				// Server doesn't publish RFC 8414 metadata (e.g. GitHub).
				// Construct minimal metadata from the auth server URL itself:
				// many servers expose {base}/authorize and {base}/access_token
				// even without a discovery document.
				metadata = constructFallbackMetadata(authServerURL)
				err = nil
			}
		}
	}
	if err != nil {
		return fmt.Errorf("oauth discovery: %w", err)
	}

	var creds *oauth.OAuthCredentials

	// --- Client credentials (machine-to-machine) ---
	if clientID != "" && clientSecret != "" && metadata.SupportsGrantType(oauth.OAuthFlowClientCredentials) {
		creds, err = flow.AuthorizeWithCredentials(&oauth.OAuthCredentials{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Metadata:     metadata,
		})
		if err != nil {
			return err
		}
		normalizeTokenType(creds)
		c.applyToken(creds.Token)
		return nil
	}

	// --- Authorization code flow ---
	if err := metadata.SupportsFlow(oauth.OAuthFlowAuthorizationCode); err != nil {
		return fmt.Errorf("server does not support a usable flow (supports: %v): %w",
			metadata.GrantTypesSupported, err)
	}

	// --- OOB (out-of-band) flow: browser shows the code, user pastes it ---
	if oob {
		creds, err = flow.AuthorizeWithCode(&oauth.OAuthCredentials{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURI:  OOBRedirectURI,
			Metadata:     metadata,
		}, func(authURL string) (string, error) {
			if err := openFn(authURL); err != nil {
				return "", fmt.Errorf("open browser: %w", err)
			}
			fmt.Fprint(os.Stderr, "Paste the authorization code: ")
			scanner := bufio.NewScanner(os.Stdin)
			if !scanner.Scan() {
				return "", fmt.Errorf("no code entered")
			}
			return strings.TrimSpace(scanner.Text()), nil
		})
		if err != nil {
			return err
		}
		normalizeTokenType(creds)
		c.applyToken(creds.Token)
		return nil
	}

	// --- Loopback browser flow ---
	listenAddr := fmt.Sprintf("127.0.0.1:%d", callbackPort)
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("callback listener: %w", err)
	}
	defer listener.Close()
	redirectURI := "http://" + listener.Addr().String() + "/callback"

	// Obtain credentials — register dynamically if no client-id provided.
	var baseCreds *oauth.OAuthCredentials
	if clientID != "" {
		baseCreds = &oauth.OAuthCredentials{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Metadata:     metadata,
		}
	} else {
		baseCreds, err = flow.Register(metadata, clientName, redirectURI)
		if err != nil {
			return fmt.Errorf("dynamic client registration failed (this server may require a pre-registered client ID; pass --client-id): %w", err)
		}
	}

	creds, err = flow.AuthorizeWithBrowser(baseCreds, listener, openFn)
	if err != nil {
		return err
	}
	normalizeTokenType(creds)
	c.applyToken(creds.Token)
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE HELPERS

// reResourceMetadata extracts the resource_metadata URL from a WWW-Authenticate header.
var reResourceMetadata = regexp.MustCompile(`resource_metadata="([^"]+)"`)

// constructFallbackMetadata builds a minimal OAuthMetadata for auth servers
// that don't publish RFC 8414 / OIDC discovery documents (e.g. GitHub).
// It assumes the server exposes standard paths relative to its base URL:
//
//	{base}/authorize  — authorization endpoint
//	{base}/access_token — token endpoint
func constructFallbackMetadata(authServerURL string) *oauth.OAuthMetadata {
	authServerURL = strings.TrimRight(authServerURL, "/")
	return &oauth.OAuthMetadata{
		Issuer:                authServerURL,
		AuthorizationEndpoint: authServerURL + "/authorize",
		TokenEndpoint:         authServerURL + "/access_token",
		// Omit GrantTypesSupported so SupportsGrantType returns true for all flows.
	}
}

// resolveAuthServerFromHeader fetches the Protected Resource Metadata document
// (RFC 9728) referenced in a WWW-Authenticate header and returns the first
// authorization server URL listed in it.
func (c *Client) resolveAuthServerFromHeader(ctx context.Context, wwwAuth string) (string, error) {
	m := reResourceMetadata.FindStringSubmatch(wwwAuth)
	if m == nil {
		return "", fmt.Errorf("no resource_metadata in WWW-Authenticate: %s", wwwAuth)
	}
	rmURL := m[1]

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rmURL, nil)
	if err != nil {
		return "", fmt.Errorf("resource metadata request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("resource metadata fetch: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var doc struct {
		AuthorizationServers []string `json:"authorization_servers"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return "", fmt.Errorf("resource metadata parse: %w", err)
	}
	if len(doc.AuthorizationServers) == 0 {
		return "", fmt.Errorf("resource metadata lists no authorization servers")
	}
	return doc.AuthorizationServers[0], nil
}

// applyToken wraps token in an oauth2 transport and stores the resulting
// *http.Client so that Connect injects it into the MCP transport.
func (c *Client) applyToken(token *oauth2.Token) {
	if token == nil {
		return
	}
	// Layer the OAuth token transport on top of the existing transport
	// (which may already be the debug trace transport).
	c.httpClient.Transport = &oauth2.Transport{
		Source: oauth2.StaticTokenSource(token),
		Base:   c.httpClient.Transport,
	}
}

// normalizeTokenType ensures the token scheme is "Bearer" (canonical
// capitalisation per RFC 7235). Some servers return "bearer" (lowercase)
// which can cause strict validators to reject the Authorization header.
func normalizeTokenType(creds *oauth.OAuthCredentials) {
	if creds == nil || creds.Token == nil {
		return
	}
	if strings.EqualFold(creds.Token.TokenType, "bearer") {
		creds.Token.TokenType = "Bearer"
	}
}
