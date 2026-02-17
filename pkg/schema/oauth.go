package schema

import "golang.org/x/oauth2"

///////////////////////////////////////////////////////////////////////////////
// TYPES

// OAuthMetadata represents OAuth 2.0 Authorization Server Metadata (RFC 8414).
type OAuthMetadata struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	DeviceAuthorizationEndpoint       string   `json:"device_authorization_endpoint,omitempty"`
	RegistrationEndpoint              string   `json:"registration_endpoint,omitempty"`
	JwksURI                           string   `json:"jwks_uri,omitempty"`
	ResponseTypesSupported            []string `json:"response_types_supported,omitempty"`
	ResponseModesSupported            []string `json:"response_modes_supported,omitempty"`
	GrantTypesSupported               []string `json:"grant_types_supported,omitempty"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported,omitempty"`
	ScopesSupported                   []string `json:"scopes_supported,omitempty"`
	CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported,omitempty"`
	RevocationEndpoint                string   `json:"revocation_endpoint,omitempty"`
	IntrospectionEndpoint             string   `json:"introspection_endpoint,omitempty"`
}

// OAuthClientRegistration represents a dynamic client registration request (RFC 7591).
type OAuthClientRegistration struct {
	ClientName              string   `json:"client_name,omitempty"`
	RedirectURIs            []string `json:"redirect_uris,omitempty"`
	GrantTypes              []string `json:"grant_types,omitempty"`
	ResponseTypes           []string `json:"response_types,omitempty"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method,omitempty"`
	Scope                   string   `json:"scope,omitempty"`
}

// OAuthClientInfo represents the response from dynamic client registration.
type OAuthClientInfo struct {
	ClientID                string   `json:"client_id"`
	ClientSecret            string   `json:"client_secret,omitempty"`
	ClientIDIssuedAt        int64    `json:"client_id_issued_at,omitempty"`
	ClientSecretExpiresAt   int64    `json:"client_secret_expires_at,omitempty"`
	ClientName              string   `json:"client_name,omitempty"`
	RedirectURIs            []string `json:"redirect_uris,omitempty"`
	GrantTypes              []string `json:"grant_types,omitempty"`
	ResponseTypes           []string `json:"response_types,omitempty"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method,omitempty"`
}

// OAuthCredentials bundles an OAuth token with the metadata needed to
// refresh or reuse it later without re-discovering or re-registering.
type OAuthCredentials struct {
	*oauth2.Token

	// ClientID is the OAuth client ID used to obtain this token.
	ClientID string `json:"client_id"`

	// Endpoint is the MCP/OAuth server endpoint (used for discovery).
	Endpoint string `json:"endpoint"`

	// TokenURL is the OAuth token endpoint URL (used for refresh without re-discovery).
	TokenURL string `json:"token_url"`
}

///////////////////////////////////////////////////////////////////////////////
// CONSTANTS

const (
	// OAuthWellKnownPath is the standard OAuth 2.0 Authorization Server Metadata endpoint (RFC 8414).
	OAuthWellKnownPath = "/.well-known/oauth-authorization-server"

	// OIDCWellKnownPath is the OpenID Connect Discovery endpoint (RFC 5785).
	OIDCWellKnownPath = "/.well-known/openid-configuration"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// SupportsPKCE returns true if the server supports PKCE.
func (m *OAuthMetadata) SupportsPKCE() bool {
	for _, method := range m.CodeChallengeMethodsSupported {
		if method == "S256" || method == "plain" {
			return true
		}
	}
	return false
}

// SupportsS256 returns true if the server supports S256 challenge method.
func (m *OAuthMetadata) SupportsS256() bool {
	for _, method := range m.CodeChallengeMethodsSupported {
		if method == "S256" {
			return true
		}
	}
	return false
}

// SupportsGrantType returns true if the server supports the given grant type.
// Per RFC 8414, grant_types_supported is optional - when omitted, we return true
// to avoid blocking flows that might be supported.
func (m *OAuthMetadata) SupportsGrantType(grantType string) bool {
	// If not specified, don't block - the grant type might still be supported
	if len(m.GrantTypesSupported) == 0 {
		return true
	}
	for _, gt := range m.GrantTypesSupported {
		if gt == grantType {
			return true
		}
	}
	return false
}

// Endpoint returns an oauth2.Endpoint from the metadata.
func (m *OAuthMetadata) Endpoint() oauth2.Endpoint {
	return oauth2.Endpoint{
		AuthURL:       m.AuthorizationEndpoint,
		DeviceAuthURL: m.DeviceAuthorizationEndpoint,
		TokenURL:      m.TokenEndpoint,
	}
}

// SupportsDeviceFlow returns true if the server supports the device authorization grant.
// The presence of device_authorization_endpoint is the primary indicator.
func (m *OAuthMetadata) SupportsDeviceFlow() bool {
	return m.DeviceAuthorizationEndpoint != ""
}

// SupportsRegistration returns true if the server supports dynamic client registration.
func (m *OAuthMetadata) SupportsRegistration() bool {
	return m.RegistrationEndpoint != ""
}
