package httpclient

import (
	// Packages
	opt "github.com/mutablelogic/go-llm/pkg/opt"
)

///////////////////////////////////////////////////////////////////////////////
// OPTIONS

// WithLimit sets the maximum number of results to return.
// If limit is nil, any existing limit is removed.
func WithLimit(limit *uint) opt.Opt {
	if limit == nil {
		return opt.SetAny(opt.LimitKey, nil)
	}
	return opt.SetUint(opt.LimitKey, *limit)
}

// WithOffset sets the pagination offset.
// If offset is 0, any existing offset is removed.
func WithOffset(offset uint) opt.Opt {
	if offset == 0 {
		return opt.SetAny(opt.OffsetKey, nil)
	}
	return opt.SetUint(opt.OffsetKey, offset)
}

// WithProvider filters results by provider name.
func WithProvider(provider string) opt.Opt {
	return opt.SetString(opt.ProviderKey, provider)
}
