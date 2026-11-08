package httpclient

import (
	"context"
	"net/http"
	"slices"
	"strings"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	oauth2 "golang.org/x/oauth2"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Registration is a client, as an authorization server registered it: its
// identifier, its secret, when it has one, and the URIs it's redirected to
type Registration struct {
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret,omitempty"`
	RedirectURIs []string `json:"redirect_uris,omitempty"`
}

// registrationRequest is a client's metadata, as it's registered, which is
// a public client, of the authorization code grant, and its refresh
// https://datatracker.ietf.org/doc/html/rfc7591
type registrationRequest struct {
	ClientName              string   `json:"client_name,omitempty"`
	RedirectURIs            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	Scope                   string   `json:"scope,omitempty"`
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Register registers a client, with the name, and the URIs it's redirected
// to, and the scopes, with the authorization server, which has a
// registration endpoint, and returns it, without the resource's
// credentials, as it's another's. Returns ErrNotImplemented when the server
// doesn't register clients, so one's identifier is needed.
func (c *Client) Register(ctx context.Context, server ServerMetadata, name string, redirectURIs []string, scopes ...string) (*Registration, error) {
	if server.RegistrationEndpoint == "" {
		return nil, llm.ErrNotImplemented.Withf("authorization server %q doesn't register clients", server.Issuer)
	} else if len(redirectURIs) == 0 {
		return nil, llm.ErrBadParameter.With("missing redirect URIs")
	}
	req, err := client.NewJSONRequestEx(http.MethodPost, registrationRequest{
		ClientName:              name,
		RedirectURIs:            redirectURIs,
		GrantTypes:              []string{"authorization_code", "refresh_token"},
		ResponseTypes:           []string{"code"},
		TokenEndpointAuthMethod: "none",
		Scope:                   strings.Join(scopes, " "),
	}, "application/json")
	if err != nil {
		return nil, err
	}
	var result Registration
	if err := c.metadata.DoWithContext(ctx, req, &result, client.OptReqEndpoint(server.RegistrationEndpoint)); err != nil {
		return nil, llm.ErrServiceUnavailable.Withf("register with %q: %v", server.Issuer, err)
	} else if result.ClientID == "" {
		return nil, llm.ErrServiceUnavailable.Withf("register with %q: no client_id", server.Issuer)
	}
	if len(result.RedirectURIs) == 0 {
		result.RedirectURIs = slices.Clone(redirectURIs)
	}
	return &result, nil
}

// SupportsS256 returns true when the authorization server says it supports
// PKCE's S256, which a client needs, as MCP has it
func (server ServerMetadata) SupportsS256() bool {
	return slices.Contains(server.CodeChallengeMethodsSupported, "S256")
}

// Config returns the configuration of the authorization code grant, with
// the server, for the client, its redirect URI, and the scopes, which its
// code is exchanged, and its token refreshed, with
func (server ServerMetadata) Config(registration Registration, redirectURI string, scopes ...string) *oauth2.Config {
	style := oauth2.AuthStyleAutoDetect
	if registration.ClientSecret == "" {
		style = oauth2.AuthStyleInParams
	}
	return &oauth2.Config{
		ClientID:     registration.ClientID,
		ClientSecret: registration.ClientSecret,
		Endpoint:     oauth2.Endpoint{AuthURL: server.AuthorizationEndpoint, TokenURL: server.TokenEndpoint, AuthStyle: style},
		RedirectURL:  redirectURI,
		Scopes:       scopes,
	}
}
