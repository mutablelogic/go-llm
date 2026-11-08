package cmd

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	// Packages
	broadcaster "github.com/mutablelogic/go-llm/broadcaster/httpclient"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	httpclient "github.com/mutablelogic/go-llm/mcp/registry/httpclient"
	schema "github.com/mutablelogic/go-llm/mcp/schema"
	server "github.com/mutablelogic/go-server"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

var (
	// WaitTimeout is how long a connector which is created is waited for,
	// as its server is connected, and signed in to, when it needs to be
	WaitTimeout = 10 * time.Minute

	// PollInterval is how often the connector is asked about, as well as
	// when there's an event of it, as events which happen before they're
	// watched aren't seen
	PollInterval = 2 * time.Second

	// LoopbackPort is the port of the loopback the authorization server
	// redirects to, when the server's own callback can't be, which is the
	// same each time, so the client registered with it, with its redirect,
	// is used again, or any port, when it's taken
	LoopbackPort = 8977
)

const (
	// loopbackPath is the path of the loopback's callback
	loopbackPath = "/callback"

	// loopbackShutdown is how long the loopback waits for its responses to
	// be written, as it stops
	loopbackShutdown = 3 * time.Second
)

// loopbackPage is the page a person is shown, once they're redirected to
// the loopback
var loopbackPage = template.Must(template.New("loopback").Parse(`<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>{{ .Title }}</title>
<style>body{font-family:system-ui,sans-serif;max-width:32rem;margin:4rem auto;padding:0 1rem;line-height:1.5}</style></head>
<body><h1>{{ .Title }}</h1><p>{{ .Message }}</p></body>
</html>
`))

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// waitConnected waits for the connector, as its server is connected, and
// returns it, once it is. When it needs to be signed in to, signing in is
// started, at the server's own callback, when the browser can be redirected
// to it, or at a loopback, which passes its code on to the server, and the
// URL a person signs in at is opened, in their browser, and printed. It
// returns the error of why it couldn't be connected, when it's another.
func waitConnected(ctx server.Cmd, parent context.Context, client *httpclient.Client, name string) (*schema.Connector, error) {
	endpoint, opts, err := ctx.ClientEndpoint()
	if err != nil {
		return nil, err
	}
	events, err := broadcaster.New(endpoint, opts...)
	if err != nil {
		return nil, err
	}
	waitCtx, cancel := context.WithTimeout(parent, WaitTimeout)
	defer cancel()

	// The connector's events, which say when it's connected, or needs to be
	// signed in to, are watched, so it's asked about at once
	changed := make(chan struct{}, 1)
	go func() {
		_ = events.WatchEvents(waitCtx, func(event eventschema.Event) error {
			if event.Schema == schema.Name && event.Table == schema.ConnectorTable && event.Row == name {
				select {
				case changed <- struct{}{}:
				default:
				}
			}
			return nil
		})
	}()

	// It's asked about, at once, then as there's an event of it, or every
	// interval, until it's connected, or there's an error
	var loopback *loopback
	defer func() {
		if loopback != nil {
			loopback.Close()
		}
	}()
	signingIn := false
	ticker := time.NewTicker(PollInterval)
	defer ticker.Stop()
	for {
		connector, err := client.GetConnector(waitCtx, name)
		switch {
		case err != nil && waitCtx.Err() == nil:
			return nil, err
		case err != nil:
			// Waited for long enough, or interrupted
		case connector.Connected:
			return connector, nil
		case connector.Auth == schema.AuthRequired && !signingIn:
			redirect := callback(endpoint)
			if redirect == "" {
				if loopback, err = newLoopback(client); err != nil {
					return nil, err
				}
				redirect = loopback.URL()
			}
			authorize, err := client.Authorize(waitCtx, name, redirect)
			if err != nil {
				return nil, fmt.Errorf("connector %q needs to be signed in to, but can't be: %w", name, err)
			}
			if loopback != nil {
				// Only the callback of this sign-in, with its state, is
				// passed on, so another request can't end it
				u, err := url.Parse(authorize.URL)
				if err != nil {
					return nil, err
				}
				loopback.expect(u.Query().Get("state"))
			}
			signingIn = true
			fmt.Fprintf(os.Stderr, "Sign in to %s, at:\n\n  %s\n\n", name, authorize.URL)
			if err := openBrowser(authorize.URL); err != nil {
				fmt.Fprintln(os.Stderr, "Open the URL in a browser, as one couldn't be opened:", err)
			}
		case connector.Auth != schema.AuthRequired && connector.Error != "":
			return nil, fmt.Errorf("connector %q was created, but can't be connected, and is tried again: %s", name, connector.Error)
		}

		// Wait for an event of it, or the interval, or the loopback's
		// callback, which says why, when it isn't signed in to
		var failed <-chan error
		if loopback != nil {
			failed = loopback.failed
		}
		select {
		case <-waitCtx.Done():
			if errors.Is(waitCtx.Err(), context.DeadlineExceeded) && parent.Err() == nil {
				return nil, fmt.Errorf("connector %q was created, but wasn't connected within %v", name, WaitTimeout)
			}
			return nil, parent.Err()
		case err := <-failed:
			return nil, fmt.Errorf("connector %q wasn't signed in to: %w", name, err)
		case <-changed:
		case <-ticker.C:
		}
	}
}

// callback returns the server's own callback, of its endpoint, which the
// browser is redirected to, when it can be: when it's https, or a loopback
// address, as authorization servers don't redirect to other http URLs, or
// the browser can't reach it, or else empty
func callback(endpoint string) string {
	u, err := url.Parse(endpoint)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return ""
	} else if u.Scheme == "http" && !isLoopback(u.Hostname()) {
		return ""
	}
	return strings.TrimSuffix(endpoint, "/") + "/mcp/callback"
}

// isLoopback returns true when the host is a loopback address, or
// localhost
func isLoopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// openBrowser opens the URL in the person's browser
func openBrowser(rawURL string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", rawURL)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL)
	default:
		cmd = exec.Command("xdg-open", rawURL)
	}
	return cmd.Start()
}

///////////////////////////////////////////////////////////////////////////////
// LOOPBACK

// loopback is the callback, on the person's own computer, which an
// authorization server redirects their browser to, once they've signed in,
// which passes its code on to the server's callback, as the browser can't
// be redirected to it
type loopback struct {
	listener net.Listener
	server   *http.Server
	failed   chan error

	// The state of the sign-in which is waited for, so a callback of
	// another, such as a stale tab's, or a request which isn't one, is
	// refused, without being passed on, or ending it
	mu    sync.Mutex
	state string
}

// newLoopback listens at LoopbackPort, or any port, when it's taken, of
// 127.0.0.1, and passes what it's redirected to with on to the server's
// callback
func newLoopback(client *httpclient.Client) (*loopback, error) {
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(LoopbackPort)))
	if err != nil {
		if listener, err = net.Listen("tcp", "127.0.0.1:0"); err != nil {
			return nil, err
		}
	}
	l := &loopback{listener: listener, failed: make(chan error, 1)}
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+loopbackPath, func(w http.ResponseWriter, r *http.Request) {
		// A callback which isn't of the sign-in which is waited for is
		// refused, here, and doesn't end it
		if !l.expected(r.URL.Query().Get("state")) {
			writeLoopbackPage(w, http.StatusBadRequest, "Not signed in", "This isn't the sign-in the command line is waiting for. Sign in from the URL it printed.")
			return
		}

		// Its page is written before the command is told it wasn't signed
		// in, so it's shown, before the loopback stops
		connector, err := client.Callback(r.Context(), r.URL.Query())
		if err != nil {
			writeLoopbackPage(w, http.StatusBadRequest, "Not signed in", err.Error())
			select {
			case l.failed <- err:
			default:
			}
			return
		}
		writeLoopbackPage(w, http.StatusOK, "Signed in to "+connector.Name, "You can close this window, and go back to the command line.")
	})
	l.server = &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = l.server.Serve(listener) }()
	return l, nil
}

// URL returns the URL of the loopback's callback
func (l *loopback) URL() string {
	return "http://" + l.listener.Addr().String() + loopbackPath
}

// expect sets the state of the sign-in which is waited for
func (l *loopback) expect(state string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.state = state
}

// expected returns true when the state is that of the sign-in which is
// waited for
func (l *loopback) expected(state string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.state != "" && subtle.ConstantTimeCompare([]byte(state), []byte(l.state)) == 1
}

// Close stops listening, once the responses which are being written are,
// within a few seconds, so the browser is shown its page
func (l *loopback) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), loopbackShutdown)
	defer cancel()
	if err := l.server.Shutdown(ctx); err != nil {
		_ = l.server.Close()
	}
}

// writeLoopbackPage writes the page a person is shown, once they're
// redirected to the loopback
func writeLoopbackPage(w http.ResponseWriter, status int, title, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = loopbackPage.Execute(w, struct{ Title, Message string }{title, message})
}
