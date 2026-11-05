// Package taggroup adds tags to the groups of an OpenAPI specification,
// which Redoc shows tags in, so modules which register their handlers on
// their own, in any order, can share a group
package taggroup

import (
	"slices"

	// Packages
	openapi "github.com/mutablelogic/go-server/pkg/openapi/schema"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// LLM is the group of the registry's, the toolkit's and the
	// broadcaster's tags
	LLM = "LLM"

	// ToolsAndAgents is the tag of the toolkit's tools and the registry's
	// agents, which models call
	ToolsAndAgents = "Tools & Agents"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC FUNCTIONS

// Add adds the tags to the group, which is created when there's none, so a
// tag which isn't in a group, which Redoc doesn't show, is. When the group
// has every tag, it's kept as it is. Otherwise its tags are those, in their
// order, and then those it had, so a module which adds every tag of the
// group, in order, orders it, whether it adds them before, or after, a
// module which adds one of them.
func Add(spec *openapi.Spec, group string, tags ...string) {
	i := slices.IndexFunc(spec.TagGroups, func(g openapi.TagGroup) bool { return g.Name == group })
	if i < 0 {
		spec.AddTagGroup(group, tags...)
		return
	}
	existing := spec.TagGroups[i].Tags
	if !slices.ContainsFunc(tags, func(tag string) bool { return !slices.Contains(existing, tag) }) {
		return
	}
	result := slices.Clone(tags)
	for _, tag := range existing {
		if !slices.Contains(result, tag) {
			result = append(result, tag)
		}
	}
	spec.TagGroups[i].Tags = result
}
