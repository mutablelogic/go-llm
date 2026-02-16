package tool

import (
	// Packages
	"fmt"

	opt "github.com/mutablelogic/go-llm/pkg/opt"
)

// WithToolkit sets a toolkit for generation options.
// The toolkit is stored under opt.ToolkitKey and can be retrieved
// with opts.Get(opt.ToolkitKey) and type-asserted to *Toolkit.
func WithToolkit(toolkit *Toolkit) opt.Opt {
	return opt.SetAny(opt.ToolkitKey, toolkit)
}

// WithTool adds an individual tool to the generation options.
// Individual tools are appended under opt.ToolKey and merged with
// toolkit tools by each provider.
func WithTool(t Tool) opt.Opt {
	return opt.ModifyAny(opt.ToolKey, func(existing any) (any, error) {
		if existing == nil {
			return []Tool{t}, nil
		}
		tools, ok := existing.([]Tool)
		if !ok {
			return nil, fmt.Errorf("WithTool: existing value for %q is %T, expected []Tool", opt.ToolKey, existing)
		}
		return append(tools, t), nil
	})
}
