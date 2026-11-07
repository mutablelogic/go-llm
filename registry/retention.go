package registry

import (
	"context"
	"errors"
	"log/slog"
	"time"

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
)

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// deleteOldSessions deletes the sessions which haven't changed for longer
// than WithSessionRetention keeps them, unless one is replying, or the
// function it was given keeps it, or it was saved since it was listed, and
// returns how many were deleted. They're listed once, all of them, as a
// store may read every session to list any, and then deleted, each as it
// was listed, so one which is saved since isn't. A session which can't be
// deleted, or checked, is logged, and kept, until the next time.
func (r *Registry) deleteOldSessions(ctx context.Context, logger *slog.Logger) int {
	if r.sessionRetention <= 0 || r.sessionStore == nil {
		return 0
	}
	before := time.Now().Add(-r.sessionRetention)
	list, err := r.sessionStore.List(ctx, schema.SessionListRequest{})
	if err != nil {
		logger.ErrorContext(ctx, "listing sessions to delete", "error", err.Error())
		return 0
	}

	// Each which hasn't changed since before then, unless it's kept, or
	// replying, or has changed since it was listed
	deleted := 0
	for _, session := range list.Body {
		if ctx.Err() != nil {
			break
		} else if session.Modified.IsZero() || !session.Modified.Before(before) {
			continue
		}
		if r.sessionKeep != nil {
			keep, err := r.sessionKeep(ctx, session.ID)
			if err != nil {
				logger.ErrorContext(ctx, "checking a session to delete", "session", session.ID.String(), "error", err.Error())
				continue
			} else if keep {
				continue
			}
		}
		if _, err := r.deleteSession(ctx, session.ID, types.Ptr(session.Serial)); errors.Is(err, llm.ErrConflict) || errors.Is(err, llm.ErrNotFound) {
			continue
		} else if err != nil {
			logger.ErrorContext(ctx, "deleting a session", "session", session.ID.String(), "error", err.Error())
			continue
		}
		deleted++
	}
	if deleted > 0 {
		logger.InfoContext(ctx, "deleted sessions", "count", deleted, "retention", r.sessionRetention.String())
	}
	return deleted
}
