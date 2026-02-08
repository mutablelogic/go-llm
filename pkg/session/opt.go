package session

import (
	// Packages
	opt "github.com/mutablelogic/go-llm/pkg/opt"
)

///////////////////////////////////////////////////////////////////////////////
// OPTIONS

// WithLimit sets the maximum number of sessions returned by List.
func WithLimit(limit uint) opt.Opt {
	return opt.SetUint(opt.LimitKey, limit)
}
