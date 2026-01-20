package anthropic

import (
	opt "github.com/mutablelogic/go-llm/pkg/opt"
)

////////////////////////////////////////////////////////////////////////////////
// ANTHROPIC OPTIONS

func WithAfterId(id string) opt.Opt {
	return opt.WithString("after_id", id)
}

func WithBeforeId(id string) opt.Opt {
	return opt.WithString("before_id", id)
}

func WithLimit(limit uint) opt.Opt {
	return opt.WithUint("limit", limit)
}
