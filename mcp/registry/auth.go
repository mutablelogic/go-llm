package registry

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	authclient "github.com/mutablelogic/go-llm/auth/httpclient"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	schema "github.com/mutablelogic/go-llm/mcp/schema"
	oauth2 "golang.org/x/oauth2"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// pending is a connector's authorization, which is waited for: its
// connector, as it was, its credential, its PKCE verifier, the URI it's
// redirected to, and when it's no longer waited for
type pending struct {
	connector  schema.Connector
	credential schema.Credential
	verifier   string
	redirect   string
	expires    time.Time
}

// authorizations are the authorizations which are waited for, by their
// states
type authorizations struct {
	mu      sync.Mutex
	pending map[string]pending
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// AuthorizeTimeout is how long an authorization is waited for, once it's
// started, until its callback
var AuthorizeTimeout = 10 * time.Minute

// CallbackPath is the path of the callback, beneath the API's prefix, which
// an authorization server redirects to, with its code
const CallbackPath = "/mcp/callback"

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Authorize starts the authorization of the connector's server, as it needs
// to be authorized, with the callback, which the authorization server
// redirects to, with its code, and returns the URL a person authorizes it
// at. Its authorization server is discovered, as its server said, and a
// client is registered with it, unless one with the callback is already,
// which the connector's credential has. Returns ErrNotFound when there's
// no connector with the name, ErrBadParameter when the callback isn't an
// http, or https, URL, ErrNotImplemented when the authorization server
// doesn't register clients, or support PKCE's S256, and
// ErrServiceUnavailable when it can't be discovered.
func (r *Registry) Authorize(ctx context.Context, name, callback string) (string, error) {
	connector, err := r.get(ctx, name)
	if err != nil {
		return "", err
	} else if u, err := url.Parse(callback); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", llm.ErrBadParameter.Withf("callback %q isn't an http, or https, URL", callback)
	}

	// Discover its authorization server, as its server said, when it did
	c, err := r.client(connector)
	if err != nil {
		return "", err
	}
	discoverCtx, cancel := context.WithTimeout(ctx, r.connectTimeout)
	defer cancel()
	var metadata *authclient.Metadata
	if challenge := c.Challenge(); challenge != nil {
		metadata, err = c.DiscoverWithError(discoverCtx, challenge)
	} else {
		metadata, err = c.Discover(discoverCtx, "")
	}
	if err != nil {
		return "", llm.ErrServiceUnavailable.Withf("connector %q: %v", name, err)
	}
	server := metadata.AuthorizationServers[0]
	if !server.SupportsS256() {
		return "", llm.ErrNotImplemented.Withf("authorization server %q doesn't support PKCE's S256", server.Issuer)
	}
	resource := metadata.Resource.Resource
	if resource == "" {
		resource = connector.URL
	}
	scopes := metadata.Resource.ScopesSupported
	if challenge := c.Challenge(); challenge != nil && challenge.Get("scope") != "" {
		scopes = strings.Fields(challenge.Get("scope"))
	}

	// Its client, which is the connector's, when it has one, as its
	// authorization server doesn't register clients, or is registered, with
	// the callback, unless it is already, with the authorization server
	credential := schema.Credential{URL: connector.URL, Issuer: server.Issuer, AuthorizationEndpoint: server.AuthorizationEndpoint, TokenEndpoint: server.TokenEndpoint, Resource: resource, Scopes: scopes}
	if connector.ClientIDCredential != "" {
		// Its identifier, and secret, aren't kept, but the name of their
		// credential is, and they're got each time they're used
		if _, _, err := r.clientCredentials(ctx, connector); err != nil {
			return "", err
		}
		credential.ClientCredential, credential.RedirectURIs = connector.ClientIDCredential, []string{callback}
		if existing, err := r.credentials.GetCredential(ctx, name); err == nil {
			credential.Token = existing.Token
		}
	} else if existing, err := r.credentials.GetCredential(ctx, name); err == nil && existing.Issuer == server.Issuer && existing.ClientID != "" && slices.Contains(existing.RedirectURIs, callback) {
		credential.ClientID, credential.ClientSecret, credential.RedirectURIs, credential.Token = existing.ClientID, existing.ClientSecret, existing.RedirectURIs, existing.Token
	} else {
		redirects := []string{callback}
		if err == nil && existing.Issuer == server.Issuer {
			// It's registered again, with the callback, as well as those it
			// had, as it can be authorized from more than one place
			redirects = append(slices.DeleteFunc(slices.Clone(existing.RedirectURIs), func(uri string) bool { return uri == callback }), callback)
		}
		registration, err := c.Register(discoverCtx, server, r.clientName(), redirects, scopes...)
		if err != nil {
			return "", err
		}
		credential.ClientID, credential.ClientSecret, credential.RedirectURIs = registration.ClientID, registration.ClientSecret, registration.RedirectURIs
	}

	// Its PKCE verifier, and state, which its callback is matched with
	state, err := randomState()
	if err != nil {
		return "", err
	}
	verifier := oauth2.GenerateVerifier()
	r.authorizations.add(state, pending{connector: connector, credential: credential, verifier: verifier, redirect: callback, expires: time.Now().Add(AuthorizeTimeout)})
	config, err := r.config(ctx, connector, credential, callback)
	if err != nil {
		return "", err
	}
	return config.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier), oauth2.SetAuthURLParam("resource", resource)), nil
}

// Callback completes the authorization the state is of, with the code, which
// the authorization server redirected with, or the error it said, by
// exchanging the code for a token, which is stored, with the connector's
// credential, and connecting its server again, with it. Returns the
// connector, or ErrNotFound, when the state isn't one which is waited for,
// or ErrBadParameter, when the authorization server said why it wasn't
// authorized, or the code couldn't be exchanged.
func (r *Registry) Callback(ctx context.Context, state, code, reason string) (*schema.Connector, error) {
	p, exists := r.authorizations.take(state)
	if !exists {
		return nil, llm.ErrNotFound.With("no authorization is waited for, with the state")
	} else if reason != "" {
		return nil, llm.ErrBadParameter.Withf("connector %q wasn't authorized: %s", p.connector.Name, reason)
	} else if code == "" {
		return nil, llm.ErrBadParameter.With("missing code")
	}

	// Exchange the code for a token, as the client it was authorized for
	credential := p.credential
	config, err := r.config(ctx, p.connector, credential, p.redirect)
	if err != nil {
		return nil, err
	}
	exchangeCtx, cancel := context.WithTimeout(ctx, r.connectTimeout)
	defer cancel()
	token, err := config.Exchange(exchangeCtx, code, oauth2.VerifierOption(p.verifier), oauth2.SetAuthURLParam("resource", credential.Resource))
	if err != nil {
		return nil, llm.ErrBadParameter.Withf("connector %q: the code couldn't be exchanged: %v", p.connector.Name, err)
	}
	credential.Token = token

	// Keep it, unless the connector was deleted, or its server, or client,
	// changed, as it was signed in to, so its token isn't another's
	r.credentialsMu.Lock()
	connector, err := r.get(ctx, p.connector.Name)
	if err == nil {
		r.mu.Lock()
		removed, deleted := r.removed[connector.Name]
		r.mu.Unlock()
		switch {
		case deleted && removed >= p.connector.Serial:
			err = llm.ErrNotFound.Withf("connector %q was deleted, as it was signed in to", connector.Name)
		case connector.URL != p.connector.URL || connector.ClientIDCredential != p.connector.ClientIDCredential:
			err = llm.ErrConflict.Withf("connector %q was changed, as it was signed in to", connector.Name)
		default:
			err = r.credentials.PutCredential(ctx, connector.Name, credential)
		}
	}
	r.credentialsMu.Unlock()
	if err != nil {
		return nil, err
	}

	// Connect its server again, with the token
	r.reconnect(connector)
	return r.status(connector), nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// reconnect disconnects the connector's client, so it's connected again, by
// Run, which is told it changed, with a token, when it has one
func (r *Registry) reconnect(connector schema.Connector) {
	r.mu.Lock()
	e, exists := r.clients[connector.Name]
	if exists {
		delete(r.clients, connector.Name)
	}
	r.mu.Unlock()
	if exists {
		_ = r.disconnect(connector.Name, e)
	}
	r.publish(eventschema.EventUpdate, connector)
}

// tokenSource returns the source of the token of the connector's server,
// which is refreshed, as it expires, and stored, once it is, or nil, when
// it has none
func (r *Registry) tokenSource(connector schema.Connector) oauth2.TokenSource {
	ctx := context.Background()
	credential, err := r.credentials.GetCredential(ctx, connector.Name)
	if err != nil || credential.Token == nil {
		return nil
	} else if credential.URL != connector.URL {
		// It's another server's, whose token isn't sent to this one
		return nil
	} else if credential.ClientCredential != connector.ClientIDCredential {
		// Or another client's
		return nil
	}
	config, err := r.config(ctx, connector, *credential, "")
	if err != nil {
		r.log().Warn("connector's token can't be refreshed", "connector", connector.Name, "error", err.Error())
		return nil
	}

	// It's refreshed within the connect timeout, so a token endpoint which
	// doesn't reply doesn't hold up the connector's requests
	refreshCtx := context.WithValue(ctx, oauth2.HTTPClient, &http.Client{Timeout: r.connectTimeout})
	return &storedTokens{registry: r, name: connector.Name, last: credential.Token, source: config.TokenSource(refreshCtx, credential.Token)}
}

// config returns the configuration of the authorization code grant, of the
// connector's credential, with the redirect URI, whose client's secret is
// the connector's, as its credentials are expanded, when it has a client of
// its own, or the one its authorization server registered, when it did
func (r *Registry) config(ctx context.Context, connector schema.Connector, credential schema.Credential, redirect string) (*oauth2.Config, error) {
	id, secret := credential.ClientID, credential.ClientSecret
	if credential.ClientCredential != "" {
		if credential.ClientCredential != connector.ClientIDCredential {
			return nil, llm.ErrConflict.Withf("connector %q has another client", connector.Name)
		}
		var err error
		if id, secret, err = r.clientCredentials(ctx, connector); err != nil {
			return nil, err
		}
	}
	server := authclient.ServerMetadata{Issuer: credential.Issuer, AuthorizationEndpoint: credential.AuthorizationEndpoint, TokenEndpoint: credential.TokenEndpoint}
	return server.Config(authclient.Registration{ClientID: id, ClientSecret: secret}, redirect, credential.Scopes...), nil
}

// clientName returns the name a client is registered with, which is the
// name the registry's servers are told
func (r *Registry) clientName() string {
	if r.name != "" {
		return r.name
	}
	return schema.DefaultName
}

// add waits for the authorization, with the state, and forgets those which
// are no longer waited for
func (a *authorizations) add(state string, p pending) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.pending == nil {
		a.pending = make(map[string]pending)
	}
	now := time.Now()
	for s, other := range a.pending {
		if now.After(other.expires) {
			delete(a.pending, s)
		}
	}
	a.pending[state] = p
}

// take returns the authorization with the state, which is waited for, once
func (a *authorizations) take(state string) (pending, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	p, exists := a.pending[state]
	delete(a.pending, state)
	if !exists || time.Now().After(p.expires) {
		return pending{}, false
	}
	return p, true
}

///////////////////////////////////////////////////////////////////////////////
// TOKENS

// storedTokens is the source of a connector's token, which stores it, once
// it's refreshed
type storedTokens struct {
	registry *Registry
	name     string
	source   oauth2.TokenSource

	mu   sync.Mutex
	last *oauth2.Token
}

// Token returns the token, which is refreshed, as it expires, and stored,
// once it is
func (s *storedTokens) Token() (*oauth2.Token, error) {
	// One at once, so a token which is refreshed is stored before another
	// is compared with it
	s.mu.Lock()
	defer s.mu.Unlock()
	token, err := s.source.Token()
	if err != nil {
		return nil, err
	} else if sameToken(s.last, token) {
		return token, nil
	}

	// It's stored, and only once it is, is it the last, so one which
	// couldn't be is stored again, as it's next returned
	ctx := context.Background()
	s.registry.credentialsMu.Lock()
	defer s.registry.credentialsMu.Unlock()
	credential, err := s.registry.credentials.GetCredential(ctx, s.name)
	switch {
	case errors.Is(err, llm.ErrNotFound):
		// The connector's credential was deleted, so it isn't stored
		s.last = token
	case err != nil:
		s.registry.log().Warn("connector's token not stored", "connector", s.name, "error", err.Error())
	default:
		credential.Token = token
		if err := s.registry.credentials.PutCredential(ctx, s.name, *credential); err != nil {
			s.registry.log().Warn("connector's token not stored", "connector", s.name, "error", err.Error())
		} else {
			s.last = token
		}
	}
	return token, nil
}

// sameToken returns true when the tokens are the same, as they're stored:
// their access, and refresh, tokens, and when they expire
func sameToken(a, b *oauth2.Token) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.AccessToken == b.AccessToken && a.RefreshToken == b.RefreshToken && a.TokenType == b.TokenType && a.Expiry.Equal(b.Expiry)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// randomState returns a random state, which an authorization's callback is
// matched with
func randomState() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
