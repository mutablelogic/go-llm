package schema

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	// Packages
	uuid "github.com/google/uuid"
	pg "github.com/mutablelogic/go-pg"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// MemoryMeta represents the writable/updatable fields for a memory entry.
type MemoryMeta struct {
	Value *string `json:"value,omitempty" help:"Text value stored for the memory key" optional:""`
}

// MemoryInsert represents the fields required to create or replace a memory entry.
type MemoryInsert struct {
	Session    uuid.UUID `json:"session" help:"Session owning the memory entry"`
	Key        string    `json:"key" help:"Text key for the memory entry"`
	MemoryMeta `embed:""`
}

// Memory represents a stored memory entry.
type Memory struct {
	MemoryInsert `embed:""`
	CreatedAt    time.Time  `json:"created_at,omitempty" help:"Creation timestamp" readonly:""`
	ModifiedAt   *time.Time `json:"modified_at,omitempty" help:"Last modification timestamp" optional:"" readonly:""`
}

// MemorySelector selects a single memory entry by session and key.
type MemorySelector struct {
	Session uuid.UUID `json:"session"`
	Key     string    `json:"key"`
}

// MemoryListRequest represents a request to list or search memory entries.
type MemoryListRequest struct {
	pg.OffsetLimit
	Session *uuid.UUID `json:"session,omitzero" help:"Restrict results to a single session" optional:""`
	Q       string     `json:"q,omitempty" help:"Web-style text query matched against memory keys and values using PostgreSQL websearch syntax; leave empty or use * to list all memories for the session" optional:""`
	Start   *time.Time `json:"start,omitempty" help:"Return memories on or after this timestamp" optional:""`
	End     *time.Time `json:"end,omitempty" help:"Return memories on or before this timestamp" optional:""`
}

// MemoryList represents a paginated list of memory entries.
type MemoryList struct {
	MemoryListRequest `embed:""`
	Count             uint      `json:"count" readonly:""`
	Body              []*Memory `json:"body,omitzero"`
}

const MemoryListMax uint64 = 100

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - READER

func (m *Memory) Scan(row pg.Row) error {
	var value string
	if err := row.Scan(&m.Session, &m.Key, &value, &m.CreatedAt, &m.ModifiedAt); err != nil {
		return err
	}
	m.Value = &value
	return nil
}

func (list *MemoryList) Scan(row pg.Row) error {
	var memory Memory
	if err := memory.Scan(row); err != nil {
		return err
	}
	list.Body = append(list.Body, &memory)
	return nil
}

func (list *MemoryList) ScanCount(row pg.Row) error {
	return row.Scan(&list.Count)
}

func (req MemoryListRequest) Query() url.Values {
	values := url.Values{}
	if req.Offset > 0 {
		values.Set("offset", strconv.FormatUint(req.Offset, 10))
	}
	if req.Limit != nil {
		values.Set("limit", strconv.FormatUint(types.Value(req.Limit), 10))
	}
	if req.Session != nil && *req.Session != uuid.Nil {
		values.Set("session", req.Session.String())
	}
	if q := strings.TrimSpace(req.Q); q != "" {
		values.Set("q", q)
	}
	if req.Start != nil {
		values.Set("start", req.Start.Format(time.RFC3339))
	}
	if req.End != nil {
		values.Set("end", req.End.Format(time.RFC3339))
	}
	return values
}

func (sel MemorySelector) Select(bind *pg.Bind, op pg.Op) (string, error) {
	if sel.Session == uuid.Nil {
		return "", fmt.Errorf("memory session is required")
	}
	key := strings.TrimSpace(sel.Key)
	if !types.IsIdentifier(key) {
		return "", fmt.Errorf("memory key %q is not a valid identifier", sel.Key)
	}
	bind.Set("session", sel.Session)
	bind.Set("key", key)

	switch op {
	case pg.Get:
		return bind.Query("memory.select"), nil
	default:
		return "", fmt.Errorf("MemorySelector: unsupported operation %q", op)
	}
}

func (req MemoryListRequest) Select(bind *pg.Bind, op pg.Op) (string, error) {
	bind.Del("where")
	if req.Session != nil {
		if *req.Session == uuid.Nil {
			return "", fmt.Errorf("memory session cannot be nil")
		}
		bind.Append("where", `memory."session" = `+bind.Set("session", *req.Session))
	}
	if q := strings.TrimSpace(req.Q); q != "" && q != "*" {
		bind.Append("where", `to_tsvector('simple', COALESCE(memory."key", '') || ' ' || COALESCE(memory."value", '')) @@ websearch_to_tsquery('simple', `+bind.Set("q", q)+`)`)
	}
	if req.Start != nil {
		bind.Append("where", `memory.created_at >= `+bind.Set("start", *req.Start))
	}
	if req.End != nil {
		bind.Append("where", `memory.created_at <= `+bind.Set("end", *req.End))
	}

	where := bind.Join("where", " AND ")
	if where == "" {
		bind.Set("where", "")
	} else {
		bind.Set("where", "WHERE "+where)
	}
	bind.Set("orderby", `ORDER BY memory.created_at DESC, memory."session" ASC, memory."key" ASC`)
	req.OffsetLimit.Bind(bind, MemoryListMax)

	switch op {
	case pg.List:
		return bind.Query("memory.list"), nil
	default:
		return "", fmt.Errorf("MemoryListRequest: unsupported operation %q", op)
	}
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - WRITER

func (m MemoryInsert) Insert(bind *pg.Bind) (string, error) {
	if m.Session == uuid.Nil {
		return "", fmt.Errorf("memory session is required")
	} else {
		bind.Set("session", m.Session)
	}

	if key := strings.TrimSpace(m.Key); !types.IsIdentifier(key) {
		return "", fmt.Errorf("memory key %q is not a valid identifier", m.Key)
	} else {
		bind.Set("key", key)
	}
	if value := strings.TrimSpace(types.Value(m.Value)); value == "" {
		return "", fmt.Errorf("memory value is required")
	} else {
		bind.Set("value", value)
	}

	return bind.Query("memory.insert"), nil
}

func (m MemoryInsert) Update(_ *pg.Bind) error {
	return fmt.Errorf("MemoryInsert: update: not supported")
}
