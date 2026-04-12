package manager

import (
	"context"
	"log/slog"
	"sync"
	"time"

	// Packages
	uuid "github.com/google/uuid"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	pg "github.com/mutablelogic/go-pg"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type SessionFeedCallback func([]*schema.Message)

type SessionFeed struct {
	pg.Conn
	mu                 sync.Mutex
	delay              time.Duration
	next               time.Time
	last               uint64
	nextSubscriptionID uint64
	subscribers        map[uuid.UUID]map[uint64]SessionFeedCallback
}

type messageLastID uint64

type messageLastIDSelector struct {
	Sessions []uuid.UUID
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func NewSessionFeed(ctx context.Context, conn pg.Conn, delay time.Duration) (*SessionFeed, error) {
	feed := &SessionFeed{
		Conn:  conn,
		delay: delay,
	}

	// Implement a delay for how often the feed will check for new messages.
	if delay > 0 {
		feed.next = time.Now().Add(delay)
	}

	// Initialize the last ID to the current maximum so the feed only returns new messages.
	last, err := feed.lastID(ctx, nil)
	if err != nil {
		return nil, err
	} else {
		feed.last = last
	}

	// Return success
	return feed, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (s *SessionFeed) Subscribe(ctx context.Context, session uuid.UUID, callback SessionFeedCallback) error {
	if ctx == nil {
		return schema.ErrBadParameter.With("context is required")
	}
	if session == uuid.Nil {
		return schema.ErrBadParameter.With("session is required")
	}
	if callback == nil {
		return schema.ErrBadParameter.With("callback is required")
	}

	s.mu.Lock()

	s.nextSubscriptionID++
	id := s.nextSubscriptionID
	if s.subscribers == nil {
		s.subscribers = make(map[uuid.UUID]map[uint64]SessionFeedCallback)
	}
	if s.subscribers[session] == nil {
		s.subscribers[session] = make(map[uint64]SessionFeedCallback)
	}
	s.subscribers[session][id] = callback
	s.mu.Unlock()

	go func() {
		<-ctx.Done()
		s.mu.Lock()
		defer s.mu.Unlock()
		s.unsubscribeLocked(session, id)
	}()

	return nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func (s *SessionFeed) update(ctx context.Context) error {
	now := time.Now()

	s.mu.Lock()
	if !s.next.IsZero() && now.Before(s.next) {
		s.mu.Unlock()
		return nil
	}
	if s.delay > 0 {
		s.next = now.Add(s.delay)
	}
	last := s.last
	sessions := s.subscribedSessionsLocked()
	s.mu.Unlock()

	// Fix an upper bound so pagination is stable while new messages continue arriving.
	until, err := s.lastID(ctx, nil)
	if err != nil {
		return err
	}
	if until <= last {
		return nil
	}
	if len(sessions) == 0 {
		s.mu.Lock()
		s.last = until
		s.mu.Unlock()
		return nil
	}

	// Paginate through each subscribed session until we reach the upper bound.
	total := uint(0)
	deliveries := make(map[uuid.UUID][]*schema.Message, len(sessions))
	limit := schema.MessageListMax
	for _, session := range sessions {
		offset := uint64(0)
		for {
			result, err := s.listSessionMessages(ctx, schema.MessageListRequest{
				OffsetLimit: pg.OffsetLimit{Offset: offset, Limit: &limit},
				Sessions:    []uuid.UUID{session},
				Last:        last,
				Until:       until,
			})
			if err != nil {
				return err
			}
			total += uint(len(result.Body))
			if len(result.Body) > 0 {
				deliveries[session] = append(deliveries[session], result.Body...)
			}
			if len(result.Body) == 0 || uint64(len(result.Body)) < limit {
				break
			}
			offset += uint64(len(result.Body))
		}
	}

	callbacks := make(map[uuid.UUID][]SessionFeedCallback, len(deliveries))
	s.mu.Lock()
	s.last = until
	for session := range deliveries {
		callbacks[session] = s.callbacksLocked(session)
	}
	s.mu.Unlock()

	for session, messages := range deliveries {
		for _, callback := range callbacks[session] {
			callback(messages)
		}
	}

	slog.Default().InfoContext(ctx, "updating session feed", "messages", total, "last", until)
	return nil
}

func (s *SessionFeed) listSessionMessages(ctx context.Context, req schema.MessageListRequest) (*schema.MessageList, error) {
	result := schema.MessageList{MessageListRequest: req}
	if err := s.Conn.List(ctx, &result, req); err != nil {
		return nil, pg.NormalizeError(err)
	}
	result.OffsetLimit.Clamp(uint64(result.Count))
	return &result, nil
}

func (s *SessionFeed) lastID(ctx context.Context, sessions []uuid.UUID) (uint64, error) {
	var result messageLastID
	if err := s.Conn.Get(ctx, &result, messageLastIDSelector{Sessions: sessions}); err != nil {
		return 0, pg.NormalizeError(err)
	}
	return uint64(result), nil
}

func (m *messageLastID) Scan(row pg.Row) error {
	return row.Scan((*uint64)(m))
}

func (req messageLastIDSelector) Select(bind *pg.Bind, op pg.Op) (string, error) {
	if len(req.Sessions) == 0 {
		bind.Set("where", "")
	} else {
		bind.Set("where", `WHERE message.session = ANY(`+bind.Set("sessions", req.Sessions)+`)`)
	}

	switch op {
	case pg.Get:
		return bind.Query("message.last_id"), nil
	default:
		return "", schema.ErrNotImplemented.Withf("unsupported messageLastIDSelector operation %q", op)
	}
}

func (s *SessionFeed) subscribedSessionsLocked() []uuid.UUID {
	result := make([]uuid.UUID, 0, len(s.subscribers))
	for session, callbacks := range s.subscribers {
		if len(callbacks) > 0 {
			result = append(result, session)
		}
	}
	return result
}

func (s *SessionFeed) callbacksLocked(session uuid.UUID) []SessionFeedCallback {
	callbacks := s.subscribers[session]
	result := make([]SessionFeedCallback, 0, len(callbacks))
	for _, callback := range callbacks {
		result = append(result, callback)
	}
	return result
}

func (s *SessionFeed) unsubscribeLocked(session uuid.UUID, id uint64) bool {
	callbacks, exists := s.subscribers[session]
	if !exists {
		return false
	}
	if _, exists := callbacks[id]; !exists {
		return false
	}
	delete(callbacks, id)
	if len(callbacks) == 0 {
		delete(s.subscribers, session)
	}
	return true
}

func (s *SessionFeed) unsubscribeSession(session uuid.UUID) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.unsubscribeSessionLocked(session)
}

func (s *SessionFeed) unsubscribeSessionLocked(session uuid.UUID) int {
	callbacks, exists := s.subscribers[session]
	if !exists {
		return 0
	}
	count := len(callbacks)
	delete(s.subscribers, session)
	return count
}
