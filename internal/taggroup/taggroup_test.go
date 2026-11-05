package taggroup_test

import (
	"testing"

	// Packages
	taggroup "github.com/mutablelogic/go-llm/internal/taggroup"
	openapi "github.com/mutablelogic/go-server/pkg/openapi/schema"
	assert "github.com/stretchr/testify/assert"
)

func Test_Add(t *testing.T) {
	order := []string{"Providers", "Models", "Sessions", "Inference", taggroup.ToolsAndAgents, "Events"}
	for name, adds := range map[string][][]string{
		"GroupFirst": {order, {taggroup.ToolsAndAgents}},
		"TagFirst":   {{taggroup.ToolsAndAgents}, order},
	} {
		// The group is ordered by the module which adds every tag, whether
		// it adds them before, or after, one which adds one of them
		spec := &openapi.Spec{}
		spec.AddTagGroup("Other", "OpenAPI")
		for _, tags := range adds {
			taggroup.Add(spec, taggroup.LLM, tags...)
		}
		assert.Equal(t, []openapi.TagGroup{
			{Name: "Other", Tags: []string{"OpenAPI"}},
			{Name: taggroup.LLM, Tags: order},
		}, spec.TagGroups, name)
	}

	// A tag which the group doesn't have is added, before those it has
	spec := &openapi.Spec{}
	taggroup.Add(spec, taggroup.LLM, "Providers")
	taggroup.Add(spec, taggroup.LLM, "Agents")
	assert.Equal(t, []string{"Agents", "Providers"}, spec.TagGroups[0].Tags)
}
