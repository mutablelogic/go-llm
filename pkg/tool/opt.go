package tool

import (
	"encoding/json"
	"fmt"

	// Packages
	opt "github.com/mutablelogic/go-llm/pkg/opt"
)

///////////////////////////////////////////////////////////////////////////////
// OPTIONS

// toolDefinition is the JSON structure for a tool
type toolDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	InputSchema any    `json:"input_schema,omitempty"`
}

// WithToolkit adds all tools from a toolkit to the request options.
// Each tool is serialized and added individually.
func WithToolkit(toolkit *Toolkit) opt.Opt {
	if toolkit == nil {
		return opt.Error(fmt.Errorf("toolkit is required"))
	}

	var opts []opt.Opt
	for _, t := range toolkit.Tools() {
		schema, err := t.Schema()
		if err != nil {
			return opt.Error(fmt.Errorf("failed to get schema for tool %q: %w", t.Name(), err))
		}
		tool := toolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: schema,
		}
		data, err := json.Marshal(tool)
		if err != nil {
			return opt.Error(fmt.Errorf("failed to serialize tool %q: %w", t.Name(), err))
		}
		opts = append(opts, opt.AddString("tools", string(data)))
	}

	return opt.WithOpts(opts...)
}
