package tool

import (
	// Packages
	opt "github.com/mutablelogic/go-llm/pkg/opt"
)

// WithToolkit sets a toolkit for generation options.
// The toolkit is stored under opt.ToolkitKey and can be retrieved
// with opts.Get(opt.ToolkitKey) and type-asserted to *Toolkit.
func WithToolkit(toolkit *Toolkit) opt.Opt {
	return opt.SetAny(opt.ToolkitKey, toolkit)
}
