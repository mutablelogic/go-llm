package schema_test

import (
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/mcp/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_Matcher(t *testing.T) {
	// A name is included when it matches an include pattern, or there are
	// none, and no exclude pattern, as a provider's models are
	meta := func(include, exclude []string) schema.ConnectorMeta {
		return schema.ConnectorMeta{URL: "https://mcp.example.com", Include: include, Exclude: exclude}
	}
	for _, test := range []struct {
		meta  schema.ConnectorMeta
		name  string
		match bool
	}{
		{meta(nil, nil), "search", true},
		{meta([]string{"^search"}, nil), "search_issues", true},
		{meta([]string{"^search"}, nil), "create_issue", false},
		{meta(nil, []string{"signup"}), "seolinkmap_signup", false},
		{meta(nil, []string{"signup"}), "content_search", true},
		{meta([]string{"^content_", "^get_"}, []string{"_full$"}), "content_search", true},
		{meta([]string{"^content_", "^get_"}, []string{"_full$"}), "content_get_full", false},
		{meta([]string{"^content_", "^get_"}, []string{"_full$"}), "get_website_info", true},
	} {
		match, err := test.meta.Matcher()
		require.NoError(t, err)
		assert.Equal(t, test.match, match(test.name), "%v %v %s", test.meta.Include, test.meta.Exclude, test.name)
	}

	// And a pattern must be a regular expression
	for _, m := range []schema.ConnectorMeta{meta([]string{"("}, nil), meta(nil, []string{"["})} {
		_, err := m.Matcher()
		assert.ErrorIs(t, err, llm.ErrBadParameter)
		assert.ErrorIs(t, m.Validate(), llm.ErrBadParameter)
	}
	assert.NoError(t, meta([]string{"^a"}, []string{"b$"}).Validate())
}
