package manager

import (
	"context"
	"time"

	// Packages
	uuid "github.com/google/uuid"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	pg "github.com/mutablelogic/go-pg"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

const sessionFeedMessageListMax uint64 = 1000

type SessionFeed struct {
	pg.Conn
}

type sessionFeedMessage struct {
	ID        uint64
	Session   uuid.UUID
	Message   schema.Message
	CreatedAt time.Time
}

type sessionFeedMessageListRequest struct {
	pg.OffsetLimit
	Sessions []uuid.UUID
	AfterID  uint64
}

type sessionFeedMessageList struct {
	sessionFeedMessageListRequest
	Count uint64
	Body  []sessionFeedMessage
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func NewSessionFeed(conn pg.Conn) *SessionFeed {
	return &SessionFeed{Conn: conn}
}

func (s *SessionFeed) latestMessages(ctx context.Context, sessions []uuid.UUID, afterID uint64) (*sessionFeedMessageList, error) {
	result := sessionFeedMessageList{
		sessionFeedMessageListRequest: sessionFeedMessageListRequest{
			OffsetLimit: pg.OffsetLimit{Limit: types.Ptr(sessionFeedMessageListMax)},
			Sessions:    sessions,
			AfterID:     afterID,
		},
	}
	if len(sessions) == 0 {
		return &result, nil
	}
	if err := s.Conn.List(ctx, &result, result.sessionFeedMessageListRequest); err != nil {
		return nil, pg.NormalizeError(err)
	}
	result.OffsetLimit = result.sessionFeedMessageListRequest.OffsetLimit
	result.OffsetLimit.Clamp(result.Count)
	return &result, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func (s *SessionFeed) update(ctx context.Context) error {
	_ = ctx
	return nil
}

func (m *sessionFeedMessage) Scan(row pg.Row) error {
	var result string
	if err := row.Scan(&m.ID, &m.Session, &m.Message.Role, &m.Message.Content, &m.Message.Tokens, &result, &m.Message.Meta, &m.CreatedAt); err != nil {
		return err
	}
	m.Message.Result = parseSessionFeedResult(result)
	if m.Message.Meta == nil {
		m.Message.Meta = make(map[string]any)
	}
	if m.Message.Content == nil {
		m.Message.Content = []schema.ContentBlock{}
	}
	return nil
}

func (list *sessionFeedMessageList) Scan(row pg.Row) error {
	var message sessionFeedMessage
	if err := message.Scan(row); err != nil {
		return err
	}
	list.Body = append(list.Body, message)
	return nil
}

func (list *sessionFeedMessageList) ScanCount(row pg.Row) error {
	if err := row.Scan(&list.Count); err != nil {
		return err
	}
	return nil
}

func (req sessionFeedMessageListRequest) Select(bind *pg.Bind, op pg.Op) (string, error) {
	if len(req.Sessions) == 0 {
		return "", schema.ErrBadParameter.With("session feed sessions are required")
	}

	bind.Set("sessions", req.Sessions)
	bind.Set("after_id", req.AfterID)
	req.OffsetLimit.Bind(bind, sessionFeedMessageListMax)

	switch op {
	case pg.List:
		return bind.Query("message.session_feed"), nil
	default:
		return "", schema.ErrNotImplemented.Withf("unsupported sessionFeedMessageListRequest operation %q", op)
	}
}

func parseSessionFeedResult(result string) schema.ResultType {
	switch result {
	case "", schema.ResultStop.String():
		return schema.ResultStop
	case schema.ResultMaxTokens.String():
		return schema.ResultMaxTokens
	case schema.ResultBlocked.String():
		return schema.ResultBlocked
	case schema.ResultToolCall.String():
		return schema.ResultToolCall
	case schema.ResultError.String():
		return schema.ResultError
	case schema.ResultOther.String():
		return schema.ResultOther
	case schema.ResultMaxIterations.String():
		return schema.ResultMaxIterations
	default:
		return schema.ResultOther
	}
}
