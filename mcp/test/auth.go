package test

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// auth is a mock authorization server, of a mock MCP server, which needs to
// be authorized
type auth struct {
	mu        sync.Mutex
	n         int
	clients   map[string][]string // Each client's redirect URIs
	secrets   map[string]string   // The secrets of the clients which have them
	codes     map[string]grant    // The codes which are approved, and not exchanged
	tokens    map[string]bool     // The access tokens which are valid
	refreshes map[string]bool     // The refresh tokens which are valid
}

// grant is a code which was approved, for a client, its redirect URI, and
// its PKCE challenge, and resource
type grant struct {
	client, redirect, challenge, resource string
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func newAuth() *auth {
	return &auth{clients: map[string][]string{}, secrets: map[string]string{}, codes: map[string]grant{}, tokens: map[string]bool{}, refreshes: map[string]bool{}}
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Approve approves the authorization URL, as a person would, and returns
// the URL it redirects to, with a code, and its state, or an error, when
// it isn't a valid request, such as one without S256
func (s *Server) Approve(authorizeURL string) (string, error) {
	if s.auth == nil {
		return "", fmt.Errorf("the server doesn't need to be authorized")
	}
	u, err := url.Parse(authorizeURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	a := s.auth
	a.mu.Lock()
	defer a.mu.Unlock()
	client, redirect := q.Get("client_id"), q.Get("redirect_uri")
	if q.Get("response_type") != "code" {
		return "", fmt.Errorf("response_type isn't code")
	} else if !slices.Contains(a.clients[client], redirect) {
		return "", fmt.Errorf("redirect_uri %q isn't the client's", redirect)
	} else if q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" {
		return "", fmt.Errorf("no S256 code challenge")
	} else if q.Get("resource") != s.URL {
		return "", fmt.Errorf("resource %q isn't the server's", q.Get("resource"))
	}
	a.n++
	code := fmt.Sprintf("code-%d", a.n)
	a.codes[code] = grant{client: client, redirect: redirect, challenge: q.Get("code_challenge"), resource: q.Get("resource")}
	to, err := url.Parse(redirect)
	if err != nil {
		return "", err
	}
	values := to.Query()
	values.Set("code", code)
	values.Set("state", q.Get("state"))
	to.RawQuery = values.Encode()
	return to.String(), nil
}

// Client adds a client, with its secret, and redirect URIs, as one which
// is registered beforehand, rather than dynamically, would be
func (s *Server) Client(id, secret string, redirects ...string) {
	if s.auth == nil {
		return
	}
	s.auth.mu.Lock()
	defer s.auth.mu.Unlock()
	s.auth.clients[id] = redirects
	if secret != "" {
		s.auth.secrets[id] = secret
	}
}

// Revoke makes every token the server issued invalid, so it needs to be
// authorized again
func (s *Server) Revoke() {
	if s.auth == nil {
		return
	}
	s.auth.mu.Lock()
	defer s.auth.mu.Unlock()
	clear(s.auth.tokens)
	clear(s.auth.refreshes)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// serve serves the authorization server's requests, and returns false, or
// an MCP request, which isn't authorized, when it returns false, too, or
// returns true, when it's authorized, so it's served
func (a *auth) serve(base string, w http.ResponseWriter, r *http.Request) bool {
	switch r.URL.Path {
	case "/.well-known/oauth-protected-resource":
		writeJSON(w, http.StatusOK, map[string]any{"resource": base, "authorization_servers": []string{base}})
	case "/.well-known/oauth-authorization-server":
		writeJSON(w, http.StatusOK, map[string]any{
			"issuer":                           base,
			"authorization_endpoint":           base + "/authorize",
			"token_endpoint":                   base + "/token",
			"registration_endpoint":            base + "/register",
			"code_challenge_methods_supported": []string{"S256"},
			"grant_types_supported":            []string{"authorization_code", "refresh_token"},
		})
	case "/register":
		var req struct {
			RedirectURIs []string `json:"redirect_uris"`
		}
		if r.Method != http.MethodPost || json.NewDecoder(r.Body).Decode(&req) != nil || len(req.RedirectURIs) == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_client_metadata"})
			return false
		}
		a.mu.Lock()
		a.n++
		id := fmt.Sprintf("client-%d", a.n)
		a.clients[id] = req.RedirectURIs
		a.mu.Unlock()
		writeJSON(w, http.StatusCreated, map[string]any{"client_id": id, "redirect_uris": req.RedirectURIs, "token_endpoint_auth_method": "none"})
	case "/token":
		a.token(w, r)
	default:
		header := r.Header.Get("Authorization")
		a.mu.Lock()
		valid := strings.HasPrefix(header, "Bearer ") && a.tokens[strings.TrimPrefix(header, "Bearer ")]
		a.mu.Unlock()
		if valid {
			return true
		}
		w.Header().Set("WWW-Authenticate", `Bearer realm="test", resource_metadata="`+base+`/.well-known/oauth-protected-resource"`)
		w.WriteHeader(http.StatusUnauthorized)
	}
	return false
}

// token exchanges a code, with its PKCE verifier, or a refresh token, for
// an access token, and a refresh token
func (a *auth) token(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_request"})
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	switch r.PostForm.Get("grant_type") {
	case "authorization_code":
		// The client is authenticated with HTTP Basic, or in the form
		client, secret, basic := r.BasicAuth()
		if !basic {
			client, secret = r.PostForm.Get("client_id"), r.PostForm.Get("client_secret")
		} else if id, err := url.QueryUnescape(client); err == nil {
			client = id
			if s, err := url.QueryUnescape(secret); err == nil {
				secret = s
			}
		}
		g, exists := a.codes[r.PostForm.Get("code")]
		delete(a.codes, r.PostForm.Get("code"))
		sum := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
		switch {
		case !exists, g.client != client, g.redirect != r.PostForm.Get("redirect_uri"):
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_grant"})
			return
		case a.secrets[g.client] != secret:
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "invalid_client"})
			return
		case base64.RawURLEncoding.EncodeToString(sum[:]) != g.challenge:
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_grant", "error_description": "code_verifier doesn't match"})
			return
		}
	case "refresh_token":
		if !a.refreshes[r.PostForm.Get("refresh_token")] {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_grant"})
			return
		}
		delete(a.refreshes, r.PostForm.Get("refresh_token"))
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "unsupported_grant_type"})
		return
	}
	a.n++
	access, refresh := fmt.Sprintf("access-%d", a.n), fmt.Sprintf("refresh-%d", a.n)
	a.tokens[access], a.refreshes[refresh] = true, true
	writeJSON(w, http.StatusOK, map[string]any{"access_token": access, "token_type": "Bearer", "expires_in": 3600, "refresh_token": refresh})
}

// writeJSON writes the value, as JSON, with the status
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
