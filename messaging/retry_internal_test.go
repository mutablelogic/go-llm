package messaging

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	assert "github.com/stretchr/testify/assert"
)

// rateLimited is an error which says how long to wait
type rateLimited time.Duration

func (e rateLimited) Error() string             { return "rate limited" }
func (e rateLimited) RetryAfter() time.Duration { return time.Duration(e) }

func Test_Messaging_RetryDelay(t *testing.T) {
	// The wait, when the error doesn't say, or says less
	delay, ok := retryDelay(5*time.Second, nil)
	assert.True(t, ok)
	assert.Equal(t, 5*time.Second, delay)
	delay, ok = retryDelay(5*time.Second, errors.New("network"))
	assert.True(t, ok)
	assert.Equal(t, 5*time.Second, delay)
	delay, ok = retryDelay(5*time.Second, rateLimited(time.Second))
	assert.True(t, ok)
	assert.Equal(t, 5*time.Second, delay)

	// As long as the error says, when it's longer, wrapped or not
	delay, ok = retryDelay(5*time.Second, rateLimited(32*time.Second))
	assert.True(t, ok)
	assert.Equal(t, 32*time.Second, delay)
	delay, ok = retryDelay(5*time.Second, errors.Join(errors.New("poll"), rateLimited(time.Minute)))
	assert.True(t, ok)
	assert.Equal(t, time.Minute, delay)
	assert.Equal(t, time.Minute, RetryAfter(errors.Join(errors.New("poll"), rateLimited(time.Minute))))
	assert.Zero(t, RetryAfter(errors.New("network")))

	// Not again, when it's forbidden
	_, ok = retryDelay(5*time.Second, llm.ErrForbidden.With("Unauthorized"))
	assert.False(t, ok)
}

// connector is a provider which connects, or doesn't, and counts how often
// it receives
type connector struct {
	err      error
	received chan struct{}
}

func (*connector) Name() string                                         { return "fake" }
func (*connector) Link(context.Context, string) (string, error)         { return "", nil }
func (c *connector) Connect(context.Context) (string, error)            { return "@bot", c.err }
func (*connector) Send(context.Context, string, string) ([]Sent, error) { return nil, nil }
func (c *connector) Receive(ctx context.Context, _ func(context.Context, Message) error) error {
	c.received <- struct{}{}
	<-ctx.Done()
	return nil
}

func Test_Messaging_ConnectLogged(t *testing.T) {
	// Connected, which is logged, before it receives
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	m := &Messaging{}
	ctx, cancel := context.WithCancel(context.Background())
	c := &connector{received: make(chan struct{}, 1)}
	done := make(chan struct{})
	go func() { m.receive(ctx, logger, c); close(done) }()
	select {
	case <-c.received:
	case <-time.After(time.Second):
		t.Fatal("not received")
	}
	cancel()
	<-done
	assert.Contains(t, buf.String(), `msg="messaging connected" provider=fake as=@bot`)

	// Not connected, as it's forbidden, which is logged, and it doesn't
	// receive, nor try again
	buf.Reset()
	c = &connector{err: llm.ErrForbidden.With("Unauthorized"), received: make(chan struct{}, 1)}
	m.receive(context.Background(), logger, c)
	assert.Contains(t, buf.String(), `msg="messaging not connected, and won't be, until the server restarts" provider=fake`)
	assert.Contains(t, buf.String(), "Unauthorized")
	assert.Empty(t, c.received)
}
