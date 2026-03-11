package toolkit

import (
	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type ListRequest struct {
	// Namespace restricts results to a single source.
	// Use "builtin", "user", or a connector name. Empty string returns all.
	Namespace string

	// Type filters — set to true to include that type in results.
	// When all three are false (zero value), all types are returned.
	Tools     bool
	Prompts   bool
	Resources bool

	// Pagination.
	Limit  *uint // nil means no limit
	Offset uint
}

type ListResponse struct {
	Tools     []llm.Tool
	Prompts   []llm.Prompt
	Resources []llm.Resource

	// Pagination metadata.
	Count  uint // total items matched (before pagination)
	Offset uint
	Limit  uint
}
