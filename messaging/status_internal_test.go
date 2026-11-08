package messaging

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	// Packages
	assert "github.com/stretchr/testify/assert"
)

// editor is a provider which can change, and delete, the messages it sent,
// which keeps what happened to them
type editor struct {
	mu     sync.Mutex
	events []string
	ids    int
}

func (*editor) Name() string                                                        { return "fake" }
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
		time.Sleep(150 * time.Millisecond)
		assert.Equal(t, []string{"send 1 ⏳ Transcribing the recording"}, e.got())
		s.show("Speaking the text aloud")
		s.show("Speaking the text aloud")
		assert.Len(t, e.got(), 1, "not changed again, until it's allowed to be")
		time.Sleep(150 * time.Millisecond)
		s.show("Speaking the text aloud")
		s.show("")
		time.Sleep(150 * time.Millisecond)
		s.close()
		s.show("After it's replied")
		time.Sleep(150 * time.Millisecond)
		assert.Equal(t, []string{"send 1 ⏳ Transcribing the recording", "edit 1 ⏳ Speaking the text aloud", "delete 1"}, e.got())
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
