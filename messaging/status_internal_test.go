package messaging

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	// Packages
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// editor is a provider which can change, and delete, the messages it sent,
// which keeps what happened to them
type editor struct {
	mu     sync.Mutex
	events []string
	ids    int
}

func (*editor) Name() string                                                        { return "fake" }
func (*editor) Private(string) bool                                                 { return true }
func (*editor) Link(string) string                                                  { return "" }
func (*editor) Receive(context.Context, func(context.Context, Message) error) error { return nil }

func (e *editor) Send(_ context.Context, _, text string) ([]Sent, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.ids++
	e.events = append(e.events, "send "+strconv.Itoa(e.ids)+" "+text)
	return []Sent{{ID: strconv.Itoa(e.ids), Text: true}}, nil
}

func (e *editor) Edit(_ context.Context, _, id, text string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.events = append(e.events, "edit "+id+" "+text)
	return nil
}

func (e *editor) Delete(_ context.Context, _, id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.events = append(e.events, "delete "+id)
	return nil
}

func (e *editor) got() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.events...)
}

// plain is a provider which can't change its messages
type plain struct{ editor }

func (p *plain) Edit()   {}
func (p *plain) Delete() {}

func Test_Status(t *testing.T) {
	ctx := context.Background()
	newTestStatus := func(p Provider) *status {
		s := newStatus(ctx, p, "1")
		if s != nil {
			s.delay, s.interval = 100*time.Millisecond, 100*time.Millisecond
		}
		return s
	}

	t.Run("Quick", func(t *testing.T) {
		// A reply which is quick shows nothing
		e := &editor{}
		s := newTestStatus(e)
		s.show("Transcribing the recording")
		s.close()
		time.Sleep(150 * time.Millisecond)
		assert.Empty(t, e.got())
	})

	t.Run("Slow", func(t *testing.T) {
		// One which takes a while shows the latest line, once it's taken
		// long enough, and then each change, at most so often, the same
		// line once, and deletes it, once it's replied
		e := &editor{}
		s := newTestStatus(e)
		s.show("Reading the scan")
		s.show("  Transcribing   the recording ")
		assert.Empty(t, e.got(), "nothing, until it's taken a while")
		require.Eventually(t, func() bool { return len(e.got()) == 1 }, time.Second, 5*time.Millisecond)
		assert.Equal(t, []string{"send 1 ⏳ Transcribing the recording"}, e.got())
		s.show("Speaking the text aloud")
		s.show("Speaking the text aloud")
		assert.Len(t, e.got(), 1, "not changed again, until it's allowed to be")
		require.Eventually(t, func() bool { return len(e.got()) == 2 }, time.Second, 5*time.Millisecond)
		s.show("Speaking the text aloud")
		s.show("")
		s.close()
		s.show("After it's replied")
		require.Eventually(t, func() bool { return len(e.got()) == 3 }, time.Second, 5*time.Millisecond)
		time.Sleep(150 * time.Millisecond)
		assert.Equal(t, []string{"send 1 ⏳ Transcribing the recording", "edit 1 ⏳ Speaking the text aloud", "delete 1"}, e.got(), "nothing more, once it's replied")
	})

	t.Run("Done as it's sent", func(t *testing.T) {
		// A message which is being sent, as the reply is done, is deleted
		// once it's sent, rather than left in the chat
		e := &slowEditor{editor: editor{}, sending: make(chan struct{}), release: make(chan struct{})}
		s := newTestStatus(e)
		s.show("Transcribing the recording")
		<-e.sending
		s.close()
		close(e.release)
		require.Eventually(t, func() bool { return len(e.got()) == 2 }, time.Second, 5*time.Millisecond)
		assert.Equal(t, []string{"send 1 ⏳ Transcribing the recording", "delete 1"}, e.got())
	})

	t.Run("Can't edit", func(t *testing.T) {
		// A provider which can't change its messages shows nothing, as the
		// message would be left in the chat
		p := &plain{}
		var provider Provider = p
		_, ok := provider.(Editor)
		assert.False(t, ok)
		s := newTestStatus(p)
		assert.Nil(t, s)
		s.show("Reading")
		s.close()
		assert.Empty(t, p.got())
	})
}

// slowEditor is an editor whose first message is sent once it's released,
// saying when it's being sent
type slowEditor struct {
	editor
	sending chan struct{}
	release chan struct{}
}

func (e *slowEditor) Send(ctx context.Context, address, text string) ([]Sent, error) {
	close(e.sending)
	<-e.release
	return e.editor.Send(ctx, address, text)
}

func Test_RLockContext(t *testing.T) {
	// The lock which keeps the channels from being changed is waited for no
	// longer than the context lets it be, and is let go of, once it's taken,
	// when it's been given up on
	m := &Messaging{}
	m.sending.Lock()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	assert.ErrorIs(t, m.rlockContext(ctx), context.DeadlineExceeded)
	m.sending.Unlock()
	assert.Eventually(t, func() bool {
		if m.sending.TryLock() {
			m.sending.Unlock()
			return true
		}
		return false
	}, time.Second, 5*time.Millisecond, "the lock it took later is let go of")

	// And it's taken when it's free
	require.NoError(t, m.rlockContext(context.Background()))
	m.sending.RUnlock()
}
