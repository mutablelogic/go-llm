package registry

import (
	"context"
	"errors"
	"log/slog"
	"time"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/registry/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// RetentionInterval is how often the sessions which haven't changed for
	// longer than they're kept are deleted, as the registry runs
	RetentionInterval = time.Hour

	// retentionPage is how many sessions are listed at once, to find those
	// which are deleted
	retentionPage = 100
)

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// deleteOldSessions deletes the sessions which haven't changed for longer
// than WithSessionRetention keeps them, unless one is replying, or the
// function it was given keeps it, and returns how many were deleted. A
// session which can't be deleted, or checked, is logged, and kept, until
// the next time. They're listed, and then deleted, so the sessions which are
// listed don't change as they're deleted.
func (r *Registry) deleteOldSessions(ctx context.Context, logger *slog.Logger) int {
	if r.sessionRetention <= 0 || r.sessionStore == nil {
		return 0
	}
	before := time.Now().Add(-r.sessionRetention)

	// The sessions which haven't changed since before then, which are listed
	// most recently changed first, so once one has, the rest have too
	var old []uuid.UUID
	for offset := uint(0); ; offset += retentionPage {
		list, err := r.sessionStore.List(ctx, schema.SessionListRequest{Offset: offset, Limit: types.Ptr(uint(retentionPage))})
		if err != nil {
			logger.ErrorContext(ctx, "listing sessions to delete", "error", err.Error())
			return 0
		}
		for _, session := range list.Body {
			if !session.Modified.IsZero() && session.Modified.Before(before) {
				old = append(old, session.ID)
			}
		}
		if len(list.Body) < retentionPage {
			break
		}
	}

	// Each, unless it's kept, or replying, or has changed since
	deleted := 0
	for _, id := range old {
		if ctx.Err() != nil {
			break
		}
		if r.sessionKeep != nil {
			keep, err := r.sessionKeep(ctx, id)
			if err != nil {
				logger.ErrorContext(ctx, "checking a session to delete", "session", id.String(), "error", err.Error())
				continue
			} else if keep {
				continue
			}
		}
		if session, err := r.sessionStore.Get(ctx, id); err != nil || !session.Modified.Before(before) {
			continue
		}
		if _, err := r.DeleteSession(ctx, id); errors.Is(err, llm.ErrConflict) || errors.Is(err, llm.ErrNotFound) {
			continue
		} else if err != nil {
			logger.ErrorContext(ctx, "deleting a session", "session", id.String(), "error", err.Error())
			continue
		}
		deleted++
	}
	if deleted > 0 {
		logger.InfoContext(ctx, "deleted sessions", "count", deleted, "retention", r.sessionRetention.String())
	}
	return deleted
}
