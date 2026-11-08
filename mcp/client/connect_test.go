package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	// Packages
	jsonrpc "github.com/modelcontextprotocol/go-sdk/jsonrpc"
	client "github.com/mutablelogic/go-client"
	llm "github.com/mutablelogic/go-llm"
	test "github.com/mutablelogic/go-llm/mcp/test"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// setKeepAlive sets how often a server is pinged, until the test ends
func setKeepAlive(t *testing.T, value time.Duration) {
	t.Helper()
	before := keepAlive
	keepAlive = value
	t.Cleanup(func() { keepAlive = before })
}

func Test_Replied(t *testing.T) {
	// A ping which a server replies to, even with an error, or with what
	// can't be decoded, says it's there, but one which isn't replied to
	// doesn't
	var typeErr json.UnmarshalTypeError
	assert.True(t, replied(nil))
	assert.True(t, replied(fmt.Errorf("calling ping: %w", &typeErr)))
	assert.True(t, replied(fmt.Errorf("calling ping: %w", &jsonrpc.Error{Code: -32601, Message: "method not found"})))
	assert.False(t, replied(errors.New("connection refused")))
	assert.False(t, replied(context.DeadlineExceeded))
}

func Test_Connect(t *testing.T) {
	ctx := t.Context()

	for name, sse := range map[string]bool{"Streamable": false, "SSE": true} {
		t.Run(name, func(t *testing.T) {
			// A server is connected to, with its transport, and go-client's
			// options, and the session is initialized, and then closed
			var opts []test.Opt
			if sse {
				opts = append(opts, test.WithSSE())
			}
			server := test.NewServer(t, opts...)
			c, err := New(server.URL, WithName("test", "1.0"), WithClientOpts(client.OptHeader("X-Test", "yes")))
			require.NoError(t, err)
			assert.False(t, c.Connected())
			require.NoError(t, c.Connect(ctx))
			assert.True(t, c.Connected())
			info, err := c.Status()
			require.NoError(t, err)
			assert.Equal(t, test.Name, info.Name)
			assert.Equal(t, test.Version, info.Version)
			for _, header := range server.Headers() {
				assert.Equal(t, "yes", header.Get("X-Test"), "every request has go-client's headers")
			}

			// Connecting again keeps the session
			session := c.session
			require.NoError(t, c.Connect(ctx))
			assert.Same(t, session, c.session)

			// And it's disconnected, once, and can be connected again
			require.NoError(t, c.Disconnect())
			assert.False(t, c.Connected())
			info, err = c.Status()
			assert.Nil(t, info)
			assert.NoError(t, err)
			require.NoError(t, c.Disconnect())
			require.NoError(t, c.Connect(ctx))
			assert.True(t, c.Connected())
			require.NoError(t, c.Disconnect())
		})
	}

	t.Run("Unavailable", func(t *testing.T) {
		// A server which isn't one, or can't be reached, is unavailable
		notFound := httptest.NewServer(http.NotFoundHandler())
		t.Cleanup(notFound.Close)
		c, err := New(notFound.URL)
		require.NoError(t, err)
		assert.ErrorIs(t, c.Connect(ctx), llm.ErrServiceUnavailable)
		assert.False(t, c.Connected())
		_, err = c.Status()
		assert.ErrorIs(t, err, llm.ErrServiceUnavailable, "the status has why")

		// Unless the caller gave up
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		c, err = New(notFound.URL)
		require.NoError(t, err)
		assert.ErrorIs(t, c.Connect(cancelled), context.Canceled)
		_, err = c.Status()
		assert.NoError(t, err)
	})

	t.Run("Context", func(t *testing.T) {
		// A session lasts after the context it was connected with is
		// cancelled, with either transport, as SSE's stream would otherwise
		// end with it
		for name, opts := range map[string][]test.Opt{"Streamable": nil, "SSE": {test.WithSSE()}} {
			t.Run(name, func(t *testing.T) {
				server := test.NewServer(t, opts...)
				c, err := New(server.URL)
				require.NoError(t, err)
				connectCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				require.NoError(t, c.Connect(connectCtx))
				cancel()
				time.Sleep(100 * time.Millisecond)
				assert.True(t, c.Connected())
				require.NoError(t, c.Disconnect())
			})
		}
	})

	t.Run("KeepAlive", func(t *testing.T) {
		// A server which replies to pings, even with what can't be decoded,
		// such as [], stays connected
		setKeepAlive(t, 10*time.Millisecond)
		for name, opts := range map[string][]test.Opt{"Pong": nil, "Array": {test.WithPingResult("[]")}} {
			t.Run(name, func(t *testing.T) {
				server := test.NewServer(t, opts...)
				c, err := New(server.URL)
				require.NoError(t, err)
				require.NoError(t, c.Connect(ctx))
				time.Sleep(20 * keepAlive)
				assert.True(t, c.Connected())
				require.NoError(t, c.Disconnect())
			})
		}
	})

	t.Run("Closed", func(t *testing.T) {
		// A session which the server closes is forgotten
		server := test.NewServer(t, test.WithSSE())
		c, err := New(server.URL)
		require.NoError(t, err)
		require.NoError(t, c.Connect(ctx))
		server.CloseClientConnections()
		server.Close()
		require.Eventually(t, func() bool { return !c.Connected() }, 5*time.Second, 10*time.Millisecond)
		_, err = c.Status()
		assert.Error(t, err, "the status has why it ended")
	})
}

func Test_ConnectUnlocked(t *testing.T) {
	// A server which doesn't reply, as it's connected, doesn't keep the
	// client's status from being asked for, nor another Connect from giving
	// up, as its own context ends, and it's given up on, as it's
	// disconnected
	release := make(chan struct{})
	hung := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(func() {
		close(release)
		hung.Close()
	})
	c, err := New(hung.URL)
	require.NoError(t, err)
	first := make(chan error, 1)
	go func() { first <- c.Connect(context.Background()) }()
	time.Sleep(50 * time.Millisecond)

	start := time.Now()
	assert.False(t, c.Connected())
	_, err = c.Status()
	assert.NoError(t, err)
	assert.Less(t, time.Since(start), 50*time.Millisecond, "the status isn't kept waiting")

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	assert.ErrorIs(t, c.Connect(ctx), context.DeadlineExceeded, "another Connect gives up, as its context ends")

	require.NoError(t, c.Disconnect())
	select {
	case err := <-first:
		assert.Error(t, err, "the connection is given up on, as it's disconnected")
	case <-time.After(2 * time.Second):
		t.Fatal("the connection wasn't given up on")
	}
	assert.False(t, c.Connected())
}
