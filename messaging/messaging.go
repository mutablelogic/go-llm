// Package messaging connects chats on messaging services, such as Telegram,
// to sessions, as channels, so the person can talk to their assistant there.
// A message sent in a channel's chat is sent to its session, and the
// session's replies are sent to each of its channels, wherever the message
// they reply to came from. A chat is connected with a link, which has a code,
// that the person opens, so only they can connect it, and a chat is a
// channel of one session, which a session can have more than one of.
package messaging

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"
	"uuid"

	// Packages
	otel "github.com/mutablelogic/go-client/pkg/otel"
	llm "github.com/mutablelogic/go-llm"
	broadcaster "github.com/mutablelogic/go-llm/broadcaster"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	schema "github.com/mutablelogic/go-llm/messaging/schema"
	registry "github.com/mutablelogic/go-llm/registry"
	attribute "go.opentelemetry.io/otel/attribute"
	trace "go.opentelemetry.io/otel/trace"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Messaging is the messaging providers, such as Telegram, and the channels,
// which are their chats, connected to sessions. It's safe to use
// concurrently, by one server, as a provider's messages are received by
// one.
type Messaging struct {
	store       Store
	providers   map[string]Provider
	broadcaster *broadcaster.Broadcaster
	tracer      trace.Tracer
	chat        ChatFunc
	session     SessionFunc
	open        OpenFunc
	now         func() time.Time

	// sending is held, shared, as messages are sent to channels, and to
	// change the channels, so a message isn't sent to a channel once it's
	// disconnected, or moved to another session
	sending sync.RWMutex

	// The channels, which are loaded from the store once, and the codes of
	// the links which connect a chat, which aren't kept when the server
	// restarts
	mu       sync.Mutex
	loaded   bool
	channels []schema.Channel
	codes    map[string]pairing

	// The messages for each session, which are sent to it one at a time,
	// in the order they were received
	qmu    sync.Mutex
	queues map[uuid.UUID]chan received
}

// pairing is a link's code, which connects a chat on the provider to the
// session, until it expires, or is used
type pairing struct {
	session  uuid.UUID
	provider string
	expires  time.Time
}

// Delivery is what was sent to one of a session's channels: the messages,
// in the order they were sent, and the error of anything which wasn't sent,
// or nil, when everything was.
type Delivery struct {
	Channel  schema.Channel
	Messages []Sent
	Err      error
}

// received is a message from a channel's chat
type received struct {
	channel schema.Channel
	message Message
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// Name is the namespace of the tools
	Name = "messaging"

	// CodeExpiry is how long a link which connects a chat can be used for
	CodeExpiry = 15 * time.Minute

	// codeLength is the number of characters of a link's code, of 64, so
	// it has 192 bits, which Telegram's start parameter, of at most 64,
	// takes
	codeLength = 32

	// codeAlphabet is the characters of a code
	codeAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"

	// queueLength is how many messages for a session wait to be sent to
	// it, beyond which they're refused
	queueLength = 16

	// queueIdle is how long a session's queue waits for a message before
	// it ends, until there's another
	queueIdle = time.Minute

	// typingInterval is how often a provider shows that a reply is being
	// written, as it shows it for a few seconds
	typingInterval = 4 * time.Second

	// retryMin and retryMax are how long a provider which can't receive
	// waits before it tries again, which doubles each time
	retryMin = 5 * time.Second
	retryMax = 5 * time.Minute
)

// What's said in a chat as it's connected, or isn't
const (
	textConnected = "Connected. Your messages here now go to your assistant, and its replies come here."
	textExpired   = "That link has expired, or was used already. Ask your assistant for a new one."
	textBusy      = "Too many messages are waiting for a reply. Try again in a moment."
	textDeleted   = "This chat's conversation was deleted, so it's disconnected. Ask your assistant for a new link to connect it again."
)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// New returns the messaging, whose channels are kept in the store, with the
// options, such as the providers, and the function which sends their
// messages to a session. Returns ErrBadParameter when there's no store, or
// an option isn't valid.
func New(store Store, opts ...Opt) (*Messaging, error) {
	var o opt
	if store == nil {
		return nil, llm.ErrBadParameter.With("missing messaging store")
	} else if err := o.apply(opts); err != nil {
		return nil, err
	}
	providers := make(map[string]Provider, len(o.providers))
	for _, provider := range o.providers {
		providers[provider.Name()] = provider
	}
	return &Messaging{
		store:       store,
		providers:   providers,
		broadcaster: o.broadcaster,
		tracer:      o.tracer,
		chat:        o.chat,
		session:     o.session,
		open:        o.open,
		now:         time.Now,
		codes:       make(map[string]pairing),
		queues:      make(map[uuid.UUID]chan received),
	}, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Providers returns the names of the providers, in order
func (m *Messaging) Providers() []string {
	names := make([]string, 0, len(m.providers))
	for name := range m.providers {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// Connect returns the link which connects a chat on the provider, or the
// only one, when it's empty, to the session, and when it expires. Returns
// ErrBadParameter when there's no session, or no provider is given, and
// there's more than one, and ErrNotFound when there's no provider with the
// name.
func (m *Messaging) Connect(ctx context.Context, session uuid.UUID, provider string) (_ string, _ time.Time, err error) {
	_, endSpan := otel.StartSpan(m.tracer, ctx, "messaging.Connect",
		attribute.String("arg.session", session.String()),
		attribute.String("arg.provider", provider),
	)
	defer func() { endSpan(err) }()

	if session == uuid.Nil() {
		return "", time.Time{}, llm.ErrBadParameter.With("missing session")
	}
	p, err := m.provider(provider)
	if err != nil {
		return "", time.Time{}, err
	}
	code, err := newCode()
	if err != nil {
		return "", time.Time{}, err
	}
	now := m.now()
	expires := now.Add(CodeExpiry)
	m.mu.Lock()
	for code, pairing := range m.codes {
		if !now.Before(pairing.expires) {
			delete(m.codes, code)
		}
	}
	m.codes[code] = pairing{session: session, provider: p.Name(), expires: expires}
	m.mu.Unlock()
	return p.Link(code), expires, nil
}

// List returns the channels of the session, or all of them, when it's the
// nil UUID, the first connected first
func (m *Messaging) List(ctx context.Context, session uuid.UUID) (_ []schema.Channel, err error) {
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "messaging.List",
		attribute.String("arg.session", session.String()),
	)
	defer func() { endSpan(err) }()

	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.load(ctx); err != nil {
		return nil, err
	}
	result := make([]schema.Channel, 0, len(m.channels))
	for _, channel := range m.channels {
		if session == uuid.Nil() || channel.Session == session {
			result = append(result, channel)
		}
	}
	return result, nil
}

// Disconnect disconnects the channel with the identifier, and returns it,
// or ErrNotFound when there's none
func (m *Messaging) Disconnect(ctx context.Context, id uuid.UUID) (_ schema.Channel, err error) {
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "messaging.Disconnect",
		attribute.String("arg.id", id.String()),
	)
	defer func() { endSpan(err) }()

	m.sending.Lock()
	defer m.sending.Unlock()
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.load(ctx); err != nil {
		return schema.Channel{}, err
	}
	i := slices.IndexFunc(m.channels, func(channel schema.Channel) bool { return channel.ID == id })
	if i < 0 {
		return schema.Channel{}, llm.ErrNotFound.Withf("channel %q", id)
	}
	channel := m.channels[i]
	if err := m.save(ctx, slices.Delete(slices.Clone(m.channels), i, i+1)); err != nil {
		return schema.Channel{}, err
	}
	return channel, nil
}

// DeleteSession disconnects the channels of the session, which was
// deleted, and forgets the links which would connect a chat to it, and
// returns how many channels there were
func (m *Messaging) DeleteSession(ctx context.Context, session uuid.UUID) (int, error) {
	m.sending.Lock()
	defer m.sending.Unlock()
	m.mu.Lock()
	defer m.mu.Unlock()
	for code, pairing := range m.codes {
		if pairing.session == session {
			delete(m.codes, code)
		}
	}
	if err := m.load(ctx); err != nil {
		return 0, err
	}
	kept := slices.DeleteFunc(slices.Clone(m.channels), func(channel schema.Channel) bool { return channel.Session == session })
	n := len(m.channels) - len(kept)
	if n == 0 {
		return 0, nil
	}
	return n, m.save(ctx, kept)
}

// Send sends the text, and the session's files, by their URIs, to the
// channels of the session with the identifiers, or all of them, when there
// are none, and returns what was sent to each, in order, or why it wasn't,
// so a channel which couldn't be sent to, or a file which couldn't be,
// doesn't stop the others. A provider which can't send files, or without
// WithFiles, is sent the text only. Returns ErrBadParameter when there's no
// text, nor files, and ErrNotFound when the session has no channels, or
// one of the identifiers isn't its.
func (m *Messaging) Send(ctx context.Context, session uuid.UUID, out Outgoing, ids ...uuid.UUID) (_ []Delivery, err error) {
	ctx, endSpan := otel.StartSpan(m.tracer, ctx, "messaging.Send",
		attribute.String("arg.session", session.String()),
		attribute.Int("arg.text", len(out.Text)),
		attribute.Int("arg.files", len(out.Files)),
	)
	defer func() { endSpan(err) }()

	if strings.TrimSpace(out.Text) == "" && len(out.Files) == 0 {
		return nil, llm.ErrBadParameter.With("missing text")
	}

	// The channels aren't changed until it's sent
	m.sending.RLock()
	defer m.sending.RUnlock()
	channels, err := m.List(ctx, session)
	if err != nil {
		return nil, err
	} else if session == uuid.Nil() || len(channels) == 0 {
		return nil, llm.ErrNotFound.With("no channels are connected to this conversation")
	}
	if len(ids) > 0 {
		chosen := make([]schema.Channel, 0, len(ids))
		for _, id := range ids {
			i := slices.IndexFunc(channels, func(channel schema.Channel) bool { return channel.ID == id })
			if i < 0 {
				return nil, llm.ErrNotFound.Withf("channel %q", id)
			}
			chosen = append(chosen, channels[i])
		}
		channels = chosen
	}
	deliveries := make([]Delivery, 0, len(channels))
	for _, channel := range channels {
		sent, err := m.send(ctx, channel, out)
		deliveries = append(deliveries, Delivery{Channel: channel, Messages: sent, Err: err})
	}
	return deliveries, nil
}

// Reply sends the session's reply, its text, and the files it made, such
// as an image, or speech, to each of its channels, and does nothing when it
// has none, or the reply has neither. It's called with each reply which is
// added to a session, wherever the message it replies to came from.
func (m *Messaging) Reply(ctx context.Context, session uuid.UUID, out Outgoing) error {
	if strings.TrimSpace(out.Text) == "" && len(out.Files) == 0 {
		return nil
	}
	if channels, err := m.List(ctx, session); err != nil {
		return err
	} else if len(channels) == 0 {
		return nil
	}
	deliveries, err := m.Send(ctx, session, out)
	for _, delivery := range deliveries {
		err = errors.Join(err, delivery.Err)
	}
	return err
}

// Run receives the messages sent to each provider, which are sent to their
// channel's session, until the context is cancelled, when it returns nil.
// A message which connects a chat connects it, and one from a chat which
// isn't connected is ignored. A provider which can't receive tries again,
// waiting longer each time. With a broadcaster, the channels of each
// session which is deleted are disconnected. What fails is logged to the
// logger, when there is one.
func (m *Messaging) Run(ctx context.Context, logger *slog.Logger) error {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	var wg sync.WaitGroup
	defer wg.Wait()
	for _, name := range m.Providers() {
		wg.Go(func() {
			m.receive(ctx, logger, m.providers[name])
		})
	}

	// Disconnect the channels of the sessions which are deleted
	var events chan eventschema.Event
	var done <-chan error
	if m.broadcaster != nil {
		events = make(chan eventschema.Event, 256)
		var err error
		if done, err = m.broadcaster.Watch(ctx, events); err != nil {
			logger.WarnContext(ctx, "channels of deleted sessions won't be disconnected", "error", err.Error())
			events, done = nil, nil
		}
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-done:
			events, done = nil, nil
		case event := <-events:
			if event.Schema != registry.EventSchema || event.Table != registry.SessionTable || event.Action != eventschema.EventDelete {
				continue
			}
			if session, err := uuid.Parse(event.Row); err == nil {
				if n, err := m.DeleteSession(ctx, session); err != nil {
					logger.ErrorContext(ctx, "channels of deleted session not disconnected", "session", session, "error", err.Error())
				} else if n > 0 {
					logger.InfoContext(ctx, "channels of deleted session disconnected", "session", session, "channels", n)
				}
			}
		}
	}
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// provider returns the provider with the name, or the only one, when it's
// empty
func (m *Messaging) provider(name string) (Provider, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		switch len(m.providers) {
		case 0:
			return nil, llm.ErrNotFound.With("no messaging providers are set up")
		case 1:
			for _, provider := range m.providers {
				return provider, nil
			}
		}
		return nil, llm.ErrBadParameter.Withf("missing provider, which is one of %s", strings.Join(m.Providers(), ", "))
	}
	if provider, ok := m.providers[name]; ok {
		return provider, nil
	}
	return nil, llm.ErrNotFound.Withf("messaging provider %q, which isn't one of %s", name, strings.Join(m.Providers(), ", "))
}

// receive receives the provider's messages, until the context is
// cancelled, trying again, when it can't, waiting longer each time, until
// it has received a message
func (m *Messaging) receive(ctx context.Context, logger *slog.Logger, provider Provider) {
	wait := retryMin
	for {
		err := provider.Receive(ctx, func(ctx context.Context, message Message) error {
			wait = retryMin
			err := m.received(ctx, logger, provider, message)
			if err != nil && ctx.Err() == nil {
				logger.ErrorContext(ctx, "message not handled", "provider", provider.Name(), "address", message.Address, "error", err.Error())
			}
			return err
		})
		if ctx.Err() != nil {
			return
		} else if err != nil {
			logger.ErrorContext(ctx, "messages not received", "provider", provider.Name(), "retry", wait.String(), "error", err.Error())
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		wait = min(wait*2, retryMax)
	}
}

// received connects the chat the message was sent in, when it has a code,
// or queues it for its channel's session, or ignores it, when its chat
// isn't connected
func (m *Messaging) received(ctx context.Context, logger *slog.Logger, provider Provider, message Message) error {
	if message.Code != "" {
		channel, err := m.pair(ctx, provider, message)
		switch {
		case errors.Is(err, errExpired):
			_, err := provider.Send(ctx, message.Address, textExpired)
			return err
		case err != nil:
			return err
		}
		logger.InfoContext(ctx, "chat connected", "provider", provider.Name(), "chat", channel.Title, "session", channel.Session)
		_, err = provider.Send(ctx, message.Address, textConnected)
		return err
	}
	channel, ok, err := m.find(ctx, provider.Name(), message.Address)
	if err != nil {
		return err
	} else if !ok {
		logger.InfoContext(ctx, "message from a chat which isn't connected ignored", "provider", provider.Name(), "address", message.Address, "chat", message.Title)
		return nil
	} else if strings.TrimSpace(message.Text) == "" || m.chat == nil {
		return nil
	}
	if !m.enqueue(ctx, logger, received{channel: channel, message: message}) {
		_, err := provider.Send(ctx, message.Address, textBusy)
		return err
	}
	return nil
}

// errExpired is the error of a code which isn't one, as it expired, or was
// used
var errExpired = errors.New("code expired")

// pair connects the chat the message was sent in to the session of its
// code, which is used, and returns its channel, replacing the channel it
// was, when it was connected to a session already. Returns errExpired when
// the code isn't one.
func (m *Messaging) pair(ctx context.Context, provider Provider, message Message) (schema.Channel, error) {
	m.mu.Lock()
	pairing, ok := m.codes[message.Code]
	delete(m.codes, message.Code)
	m.mu.Unlock()
	if !ok || pairing.provider != provider.Name() || !m.now().Before(pairing.expires) {
		return schema.Channel{}, errExpired
	}

	// The session must exist, as it may have been deleted since the link
	// was made, so a chat isn't moved to it from one which does
	if m.session != nil {
		if err := m.session(ctx, pairing.session); errors.Is(err, llm.ErrNotFound) {
			return schema.Channel{}, errExpired
		} else if err != nil {
			return schema.Channel{}, err
		}
	}
	m.sending.Lock()
	defer m.sending.Unlock()
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.load(ctx); err != nil {
		return schema.Channel{}, err
	}
	channel := schema.Channel{
		ID: uuid.New(),
		ChannelMeta: schema.ChannelMeta{
			Session:  pairing.session,
			Provider: provider.Name(),
			Address:  message.Address,
			Title:    message.Title,
		},
		Created: m.now().UTC(),
	}
	channels := slices.DeleteFunc(slices.Clone(m.channels), func(other schema.Channel) bool {
		return other.Provider == channel.Provider && other.Address == channel.Address
	})
	if err := m.save(ctx, append(channels, channel)); err != nil {
		return schema.Channel{}, err
	}
	return channel, nil
}

// find returns the channel of the chat on the provider, and whether there
// is one
func (m *Messaging) find(ctx context.Context, provider, address string) (schema.Channel, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.load(ctx); err != nil {
		return schema.Channel{}, false, err
	}
	for _, channel := range m.channels {
		if channel.Provider == provider && channel.Address == address {
			return channel, true, nil
		}
	}
	return schema.Channel{}, false, nil
}

// enqueue queues the message for its channel's session, whose queue sends
// them to it one at a time, and returns false, when too many are waiting
func (m *Messaging) enqueue(ctx context.Context, logger *slog.Logger, r received) bool {
	m.qmu.Lock()
	defer m.qmu.Unlock()
	session := r.channel.Session
	queue, ok := m.queues[session]
	if !ok {
		queue = make(chan received, queueLength)
		m.queues[session] = queue
		go m.work(ctx, logger, session, queue)
	}
	select {
	case queue <- r:
		return true
	default:
		return false
	}
}

// work sends the session's messages to it, one at a time, until none has
// been queued for a while, or the context is cancelled
func (m *Messaging) work(ctx context.Context, logger *slog.Logger, session uuid.UUID, queue chan received) {
	timer := time.NewTimer(queueIdle)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case r := <-queue:
			m.handle(ctx, logger, r)
			timer.Reset(queueIdle)
		case <-timer.C:
			// It ends, unless a message was queued since
			m.qmu.Lock()
			if len(queue) == 0 {
				delete(m.queues, session)
				m.qmu.Unlock()
				return
			}
			m.qmu.Unlock()
			timer.Reset(queueIdle)
		}
	}
}

// handle sends the message to its channel's session, showing that a reply
// is being written, as it's written, and says why, in the chat, when it
// can't be
func (m *Messaging) handle(ctx context.Context, logger *slog.Logger, r received) {
	provider := m.providers[r.channel.Provider]
	if provider == nil {
		return
	}
	stop := m.typing(ctx, provider, r.channel.Address)
	err := m.chat(ctx, r.channel, r.message)
	stop()
	if err == nil || ctx.Err() != nil {
		return
	}

	// The channel is disconnected, when its session was deleted, as the
	// event that it was may have been missed
	text := "Sorry, that couldn't be answered: " + err.Error()
	if m.session != nil && errors.Is(m.session(ctx, r.channel.Session), llm.ErrNotFound) {
		if _, err := m.DeleteSession(ctx, r.channel.Session); err != nil {
			logger.ErrorContext(ctx, "channels of deleted session not disconnected", "session", r.channel.Session, "error", err.Error())
		}
		logger.InfoContext(ctx, "channels of deleted session disconnected", "session", r.channel.Session)
		text = textDeleted
	} else {
		logger.ErrorContext(ctx, "message not answered", "provider", provider.Name(), "session", r.channel.Session, "error", err.Error())
	}
	if _, err := provider.Send(ctx, r.channel.Address, text); err != nil {
		logger.ErrorContext(ctx, "message not sent", "provider", provider.Name(), "session", r.channel.Session, "error", err.Error())
	}
}

// typing shows, in the chat, that a reply is being written, when the
// provider can, until the function it returns is called
func (m *Messaging) typing(ctx context.Context, provider Provider, address string) func() {
	typer, ok := provider.(Typer)
	if !ok {
		return func() {}
	}
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(typingInterval)
		defer ticker.Stop()
		for {
			_ = typer.Typing(ctx, address)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return func() {
		cancel()
		<-done
	}
}

// send sends the text, and the files, to the channel's chat, and returns
// the messages they were sent as, in order, and the errors of what wasn't
// sent. The text is the caption of the first file, when it's an image, and
// is otherwise sent before the files. A provider which can't send files is
// sent the text.
func (m *Messaging) send(ctx context.Context, channel schema.Channel, out Outgoing) ([]Sent, error) {
	provider := m.providers[channel.Provider]
	if provider == nil {
		return nil, llm.ErrNotFound.Withf("messaging provider %q, of channel %q, isn't set up", channel.Provider, channel.ID)
	}
	text := strings.TrimSpace(out.Text)
	sender, ok := provider.(FileSender)
	if !ok || m.open == nil || len(out.Files) == 0 {
		if text == "" {
			return nil, nil
		}
		return provider.Send(ctx, channel.Address, text)
	}
	var result []Sent
	var errs error
	for _, uri := range out.Files {
		file, content, err := m.open(ctx, channel.Session, uri)
		if err != nil {
			errs = errors.Join(errs, err)
			continue
		}
		content.Close()
		caption := ""
		if text != "" {
			if strings.HasPrefix(file.Type, "image/") {
				caption = text
			} else {
				sent, err := provider.Send(ctx, channel.Address, text)
				result = append(result, sent...)
				if err != nil {
					return result, errors.Join(errs, err)
				}
			}
			text = ""
		}

		// The provider opens it, and can open it again
		open := func() (io.ReadCloser, error) {
			_, content, err := m.open(ctx, channel.Session, uri)
			return content, err
		}
		sent, err := sender.SendFile(ctx, channel.Address, file, open, caption)
		result = append(result, sent...)
		errs = errors.Join(errs, err)
	}

	// The text, when none of the files could be opened
	if text != "" {
		sent, err := provider.Send(ctx, channel.Address, text)
		result = append(result, sent...)
		errs = errors.Join(errs, err)
	}
	return result, errs
}

// load loads the channels from the store, once. The caller holds the lock.
func (m *Messaging) load(ctx context.Context) error {
	if m.loaded {
		return nil
	}
	channels, err := m.store.Load(ctx)
	if err != nil {
		return err
	}
	m.channels, m.loaded = channels, true
	return nil
}

// save saves the channels to the store, and keeps them, once they're saved.
// The caller holds the lock.
func (m *Messaging) save(ctx context.Context, channels []schema.Channel) error {
	if err := m.store.Save(ctx, channels); err != nil {
		return err
	}
	m.channels = channels
	return nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// newCode returns a random code, for a link which connects a chat
func newCode() (string, error) {
	data := make([]byte, codeLength)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	for i, b := range data {
		data[i] = codeAlphabet[int(b)%len(codeAlphabet)]
	}
	return string(data), nil
}
