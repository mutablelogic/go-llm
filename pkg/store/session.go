package store

import (
	"strings"
	"time"

	// Packages
	uuid "github.com/google/uuid"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS - SESSION UTILITIES

// validateLabels checks that all label keys are valid identifiers.
func validateLabels(labels map[string]string) error {
	for k := range labels {
		if !types.IsIdentifier(k) {
			return llm.ErrBadParameter.Withf("invalid label key: %q", k)
		}
	}
	return nil
}

// matchLabels returns true if the session's labels contain all of the
// required filter labels. An empty filter matches everything.
// Each filter entry is a "key:value" string.
func matchLabels(sessionLabels map[string]string, filter []string) bool {
	for _, f := range filter {
		k, v, ok := strings.Cut(f, ":")
		if !ok {
			continue
		}
		if sessionLabels[k] != v {
			return false
		}
	}
	return true
}

// newSession validates meta and returns a new Session with a unique ID,
// empty conversation, and timestamps set to now.
func newSession(meta schema.SessionMeta) (*schema.Session, error) {
	if meta.Model == "" {
		return nil, llm.ErrBadParameter.With("model name is required")
	}
	if err := validateLabels(meta.Labels); err != nil {
		return nil, err
	}

	now := time.Now()
	return &schema.Session{
		ID:          uuid.New().String(),
		SessionMeta: meta,
		Messages:    make(schema.Conversation, 0),
		Created:     now,
		Modified:    now,
	}, nil
}

// mergeSessionMeta applies non-zero fields from meta onto s and updates the
// modified timestamp. Returns an error only if label validation fails.
func mergeSessionMeta(s *schema.Session, meta schema.SessionMeta) error {
	if meta.Name != "" {
		s.Name = meta.Name
	}
	if meta.Model != "" {
		s.Model = meta.Model
	}
	if meta.Provider != "" {
		s.Provider = meta.Provider
	}
	if meta.SystemPrompt != "" {
		s.SystemPrompt = meta.SystemPrompt
	}
	if meta.Format != nil {
		s.Format = meta.Format
	}
	if meta.Thinking != nil {
		s.Thinking = meta.Thinking
	}
	if meta.ThinkingBudget > 0 {
		s.ThinkingBudget = meta.ThinkingBudget
	}
	if len(meta.Labels) > 0 {
		if err := validateLabels(meta.Labels); err != nil {
			return err
		}
		if s.Labels == nil {
			s.Labels = make(map[string]string)
		}
		for k, v := range meta.Labels {
			if v == "" {
				delete(s.Labels, k)
			} else {
				s.Labels[k] = v
			}
		}
	}
	s.Modified = time.Now()
	return nil
}

// paginate returns a slice of items bounded by offset and limit, along with the
// total count of items before pagination.
func paginate[T any](items []T, offset uint, limit *uint) ([]T, uint) {
	total := uint(len(items))
	start := min(offset, total)
	end := start + types.Value(limit)
	if limit == nil || end > total {
		end = total
	}
	return items[start:end], total
}
