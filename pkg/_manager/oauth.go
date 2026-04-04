package manager

import (
	"context"
	"log/slog"
	"net/http"
	"sync"

	// Packages
	client "github.com/mutablelogic/go-client"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	oauth2 "golang.org/x/oauth2"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// OAuthClientOpt returns a client.ClientOpt that installs an oauth2 transport
// which refreshes the access token automatically when it expires.
//
// After each successful refresh the new token is persisted back to the
// credential store so that subsequent restarts start with a valid token.
func OAuthClientOpt(ctx context.Context, url string, cred *schema.OAuthCredentials, store schema.CredentialStore) client.ClientOpt {
	config := &oauth2.Config{
		ClientID:     cred.ClientID,
		ClientSecret: cred.ClientSecret,
		Endpoint:     oauth2.Endpoint{TokenURL: cred.TokenURL},
	}
	// config.TokenSource wraps the token in a ReuseTokenSource that refreshes
	// automatically when the access token has expired.
	base := config.TokenSource(ctx, cred.Token)
	ts := &persistingTokenSource{
		ctx:     ctx,
		base:    base,
		current: cred.Token,
		cred:    *cred,
		url:     url,
		store:   store,
	}
	return client.OptTransport(func(parent http.RoundTripper) http.RoundTripper {
		return &oauth2.Transport{Source: ts, Base: parent}
	})
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE TYPES

// persistingTokenSource wraps an oauth2.TokenSource and writes the refreshed
// token back to the credential store whenever a new access token is obtained.
type persistingTokenSource struct {
	mu      sync.Mutex
	ctx     context.Context
	base    oauth2.TokenSource
	current *oauth2.Token
	cred    schema.OAuthCredentials // full creds, for re-persisting with ClientID etc.
	url     string
	store   schema.CredentialStore
}

func (p *persistingTokenSource) Token() (*oauth2.Token, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	tok, err := p.base.Token()
	if err != nil {
		return nil, err
	}

	// Persist the new token if the access token changed.
	if tok.AccessToken != p.current.AccessToken {
		p.current = tok
		p.cred.Token = tok // update the embedded token, keep everything else
		if p.store != nil {
			if credErr := p.store.SetCredential(p.ctx, p.url, p.cred); credErr != nil {
				slog.Warn("failed to persist refreshed oauth token", "url", p.url, "err", credErr)
			} else {
				slog.Debug("oauth token refreshed and persisted", "url", p.url)
			}
		}
	}
	return tok, nil
}
