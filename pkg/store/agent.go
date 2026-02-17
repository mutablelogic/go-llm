package store

import (
	"encoding/json"
	"sort"
	"time"

	// Packages
	uuid "github.com/google/uuid"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS - AGENT UTILITIES

// validateAgentName checks that name is a valid identifier.
func validateAgentName(name string) error {
	if !types.IsIdentifier(name) {
		return llm.ErrBadParameter.Withf("agent name: must be a valid identifier, got %q", name)
	}
	return nil
}

// filterAgents filters, sorts, and deduplicates agents according to the
// list request. When Name is set, all matching versions are returned
// (optionally narrowed to a specific Version). When Name is empty, only
// the latest version of each agent is returned. Results are sorted by
// creation time, most recent first.
func filterAgents(all []*schema.Agent, req schema.ListAgentRequest) []*schema.Agent {
	// Filter by name and version
	var candidates []*schema.Agent
	for _, a := range all {
		if req.Name != "" && a.Name != req.Name {
			continue
		}
		if req.Version != nil && a.Version != *req.Version {
			continue
		}
		candidates = append(candidates, a)
	}

	// Sort by creation time, most recent first
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Created.After(candidates[j].Created)
	})

	// When no name filter, keep only the latest version per name
	if req.Name == "" {
		seen := make(map[string]bool)
		result := candidates[:0]
		for _, a := range candidates {
			if !seen[a.Name] {
				seen[a.Name] = true
				result = append(result, a)
			}
		}
		return result
	}
	return candidates
}

// newAgentVersion validates an update request and returns a new agent
// version with merged metadata, a fresh UUID, and version+1. Returns
// (existing, nil) when nothing has changed (no-op), or (nil, error) on
// validation failure.
func newAgentVersion(existing *schema.Agent, meta schema.AgentMeta) (*schema.Agent, error) {
	// Reject name changes
	if meta.Name != "" && meta.Name != existing.Name {
		return nil, llm.ErrBadParameter.With("agent name cannot be changed via update")
	}

	// Merge non-zero fields from meta onto existing
	merged := existing.AgentMeta
	mergeAgentMeta(&merged, meta)

	// No-op if nothing has changed
	if agentMetaEqual(existing.AgentMeta, merged) {
		return existing, nil
	}

	return &schema.Agent{
		ID:        uuid.New().String(),
		Created:   time.Now(),
		Version:   existing.Version + 1,
		AgentMeta: merged,
	}, nil
}

// mergeAgentMeta applies non-zero fields from src onto dst.
func mergeAgentMeta(dst *schema.AgentMeta, src schema.AgentMeta) {
	// AgentMeta fields
	if src.Name != "" {
		dst.Name = src.Name
	}
	if src.Title != "" {
		dst.Title = src.Title
	}
	if src.Description != "" {
		dst.Description = src.Description
	}
	if src.Template != "" {
		dst.Template = src.Template
	}
	if len(src.Input) > 0 {
		dst.Input = src.Input
	}
	if src.Tools != nil {
		dst.Tools = src.Tools
	}

	// GeneratorMeta fields
	if src.Provider != "" {
		dst.Provider = src.Provider
	}
	if src.Model != "" {
		dst.Model = src.Model
	}
	if src.SystemPrompt != "" {
		dst.SystemPrompt = src.SystemPrompt
	}
	if len(src.Format) > 0 {
		dst.Format = src.Format
	}
	if src.Thinking != nil {
		dst.Thinking = src.Thinking
	}
	if src.ThinkingBudget != 0 {
		dst.ThinkingBudget = src.ThinkingBudget
	}
}

// agentMetaEqual returns true if two AgentMeta values are identical.
func agentMetaEqual(a, b schema.AgentMeta) bool {
	if a.Name != b.Name || a.Title != b.Title || a.Description != b.Description || a.Template != b.Template {
		return false
	}
	if a.Provider != b.Provider || a.Model != b.Model || a.SystemPrompt != b.SystemPrompt {
		return false
	}
	if a.ThinkingBudget != b.ThinkingBudget {
		return false
	}
	// Compare *bool Thinking
	switch {
	case a.Thinking == nil && b.Thinking == nil:
	case a.Thinking == nil || b.Thinking == nil:
		return false
	case *a.Thinking != *b.Thinking:
		return false
	}
	// Compare JSONSchema fields as JSON bytes
	if !jsonEqual(a.Format, b.Format) || !jsonEqual(a.Input, b.Input) {
		return false
	}
	// Compare Tools slices
	if len(a.Tools) != len(b.Tools) {
		return false
	}
	for i := range a.Tools {
		if a.Tools[i] != b.Tools[i] {
			return false
		}
	}
	return true
}

// jsonEqual compares two JSONSchema values by normalising through JSON
// round-trip so that semantically equivalent schemas with different
// whitespace compare equal.
func jsonEqual(a, b schema.JSONSchema) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	var va, vb any
	if err := json.Unmarshal(a, &va); err != nil {
		return string(a) == string(b)
	}
	if err := json.Unmarshal(b, &vb); err != nil {
		return string(a) == string(b)
	}
	na, _ := json.Marshal(va)
	nb, _ := json.Marshal(vb)
	return string(na) == string(nb)
}
