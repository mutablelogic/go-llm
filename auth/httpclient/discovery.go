package httpclient

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	// Packages
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Metadata is how a token for a resource is got: the resource's metadata,
// when it has it, and that of its authorization servers
type Metadata struct {
	Resource             ResourceMetadata `json:"resource,omitzero"`
	AuthorizationServers []ServerMetadata `json:"authorization_servers,omitempty"`
}

// ResourceMetadata is a protected resource's metadata, which says which
// authorization servers issue tokens for it
// https://datatracker.ietf.org/doc/html/rfc9728
type ResourceMetadata struct {
	Resource               string   `json:"resource,omitempty"`
	AuthorizationServers   []string `json:"authorization_servers,omitempty"`
	ScopesSupported        []string `json:"scopes_supported,omitempty"`
	BearerMethodsSupported []string `json:"bearer_methods_supported,omitempty"`
	ResourceName           string   `json:"resource_name,omitempty"`
}

// ServerMetadata is an authorization server's metadata, which says where a
// person authorizes a client, a token is got, and a client is registered
// https://datatracker.ietf.org/doc/html/rfc8414
type ServerMetadata struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint,omitempty"`
	TokenEndpoint                     string   `json:"token_endpoint,omitempty"`
	RegistrationEndpoint              string   `json:"registration_endpoint,omitempty"`
	RevocationEndpoint                string   `json:"revocation_endpoint,omitempty"`
	ScopesSupported                   []string `json:"scopes_supported,omitempty"`
	ResponseTypesSupported            []string `json:"response_types_supported,omitempty"`
	GrantTypesSupported               []string `json:"grant_types_supported,omitempty"`
	CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported,omitempty"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported,omitempty"`
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// The well-known paths of a resource's metadata, and of an authorization
// server's, as OAuth, and OpenID Connect, have them
const (
	ResourceMetadataPath = ".well-known/oauth-protected-resource"
	OAuthMetadataPath    = ".well-known/oauth-authorization-server"
	OIDCMetadataPath     = ".well-known/openid-configuration"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Discover returns how a token for the resource at the URL, or the client's
// endpoint, when it's empty, is got: its metadata, at its well-known path,
// with its path, or without, and that of its authorization servers, or of
// its origin, as an authorization server, when it says none. Returns
// ErrNotFound when no authorization server's metadata is found.
func (c *Client) Discover(ctx context.Context, resource string) (*Metadata, error) {
	if resource = strings.TrimSpace(resource); resource == "" {
		resource = c.endpoint
	}
	result := new(Metadata)
	for _, endpoint := range wellKnown(resource, ResourceMetadataPath) {
		var metadata ResourceMetadata
		if err := c.DoWithContext(ctx, nil, &metadata, client.OptReqEndpoint(endpoint)); err == nil {
			result.Resource = metadata
			break
		}
	}
	issuers := result.Resource.AuthorizationServers
	if len(issuers) == 0 {
		issuers = []string{origin(resource)}
	}
	return c.servers(ctx, result, issuers)
}

// DiscoverWithError returns how a token is got, as the error, which is an
// AuthError, says, from the URL of the resource's metadata it has, when it
// has one, or otherwise as Discover does, for the client's endpoint.
// Returns the error, when it isn't an AuthError.
func (c *Client) DiscoverWithError(ctx context.Context, err error) (*Metadata, error) {
	authErr := AsAuthError(err)
	if authErr == nil {
		return nil, err
	}
	endpoint := strings.TrimSpace(authErr.Get("resource_metadata"))
	if endpoint == "" {
		return c.Discover(ctx, "")
	}
	result := new(Metadata)
	if err := c.DoWithContext(ctx, nil, &result.Resource, client.OptReqEndpoint(endpoint)); err != nil {
		return nil, llm.ErrServiceUnavailable.Withf("resource metadata %q: %v", endpoint, err)
	}
	issuers := result.Resource.AuthorizationServers
	if len(issuers) == 0 {
		issuers = []string{origin(c.endpoint)}
	}
	return c.servers(ctx, result, issuers)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// servers adds the metadata of each of the authorization servers, by their
// issuers, which have it, and returns the result, or ErrNotFound, when none
// have it
func (c *Client) servers(ctx context.Context, result *Metadata, issuers []string) (*Metadata, error) {
	var errs []error
	for _, issuer := range issuers {
		server, err := c.server(ctx, issuer)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		result.AuthorizationServers = append(result.AuthorizationServers, *server)
	}
	if len(result.AuthorizationServers) == 0 {
		return nil, llm.ErrNotFound.Withf("no authorization server's metadata: %v", errors.Join(errs...))
	}
	return result, nil
}

// server returns the authorization server's metadata, by its issuer, at
// OAuth's well-known path, or OpenID Connect's, which says where a token is
// got, or ErrNotFound, when it hasn't any, with why each of its well-known
// URLs which isn't missing, such as one which failed, or isn't metadata,
// wasn't, so it's known why
func (c *Client) server(ctx context.Context, issuer string) (*ServerMetadata, error) {
	var errs []error
	for _, path := range []string{OAuthMetadataPath, OIDCMetadataPath} {
		for _, endpoint := range wellKnown(issuer, path) {
			var metadata ServerMetadata
			if err := c.DoWithContext(ctx, nil, &metadata, client.OptReqEndpoint(endpoint)); err != nil {
				if !missing(err) {
					errs = append(errs, fmt.Errorf("%s: %w", endpoint, err))
				}
				continue
			} else if metadata.TokenEndpoint == "" {
				errs = append(errs, fmt.Errorf("%s: has no token_endpoint", endpoint))
				continue
			}
			if metadata.Issuer == "" {
				metadata.Issuer = issuer
			}
			return &metadata, nil
		}
	}
	if len(errs) > 0 {
		return nil, llm.ErrNotFound.Withf("authorization server %q has no metadata: %v", issuer, errors.Join(errs...))
	}
	return nil, llm.ErrNotFound.Withf("authorization server %q has no metadata", issuer)
}

// missing returns true when the error is that what was asked for isn't
// there, which a well-known URL which isn't the one a server has is, so
// it's tried at another, rather than said to have failed
func missing(err error) bool {
	var httpErr httpresponse.Err
	return errors.As(err, &httpErr) && (httpErr == httpresponse.ErrNotFound || httpErr == httpresponse.Err(http.StatusMethodNotAllowed))
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// wellKnown returns the URLs of the well-known path of the URL: inserted
// between its host and its path, as RFC 8414 and RFC 9728 have it, then
// after its path, as OpenID Connect has it, and at its root, without
// those which are the same
func wellKnown(raw, path string) []string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil
	}
	base := &url.URL{Scheme: u.Scheme, Host: u.Host}
	suffix := strings.Trim(u.EscapedPath(), "/")
	candidates := []string{}
	if suffix != "" {
		candidates = append(candidates,
			base.JoinPath(path, suffix).String(),
			base.JoinPath(suffix, path).String(),
		)
	}
	candidates = append(candidates, base.JoinPath(path).String())
	seen := map[string]bool{}
	result := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if !seen[candidate] {
			seen[candidate] = true
			result = append(result, candidate)
		}
	}
	return result
}

// origin returns the URL's scheme, and host, or nothing, when it isn't one
func origin(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return (&url.URL{Scheme: u.Scheme, Host: u.Host}).String()
}
