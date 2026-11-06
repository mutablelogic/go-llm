// Package agents has example agents, as YAML, which the llm server creates
// when it starts, or which llm agent-create creates, such as:
//
//	llm agent-create agents/translate.yaml
package agents

import (
	"embed"
	"fmt"
	"io/fs"
	"slices"

	// Packages
	schema "github.com/mutablelogic/go-llm/registry/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

//go:embed *.yaml
var files embed.FS

///////////////////////////////////////////////////////////////////////////////
// PUBLIC FUNCTIONS

// All returns the example agents, in order of their files' names, which are
// built in, so they can't be changed, or deleted, or an error when one isn't
// valid
func All() ([]schema.AgentMeta, error) {
	names, err := fs.Glob(files, "*.yaml")
	if err != nil {
		return nil, err
	}
	slices.Sort(names)
	result := make([]schema.AgentMeta, 0, len(names))
	for _, name := range names {
		data, err := files.ReadFile(name)
		if err != nil {
			return nil, err
		}
		meta, err := schema.ParseAgentMeta(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		meta.Builtin = types.Ptr(true)
		result = append(result, meta)
	}
	return result, nil
}
