package httpclient

import (
	"fmt"

	// Packages
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	types "github.com/mutablelogic/go-server/pkg/types"
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

// WithLabel adds a label filter (key:value) for listing sessions.
// The key must be a valid identifier. Multiple calls accumulate filters.
func WithLabel(key, value string) opt.Opt {
	if !types.IsIdentifier(key) {
		return opt.Error(fmt.Errorf("invalid label key: %q", key))
	}
	return opt.AddString(opt.LabelKey, key+":"+value)
}

// WithName filters results by name.
func WithName(name string) opt.Opt {
	return opt.SetString(opt.NameKey, name)
}
