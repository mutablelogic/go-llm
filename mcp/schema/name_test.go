package schema_test

import (
	"testing"

	// Packages
	schema "github.com/mutablelogic/go-llm/mcp/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
	assert "github.com/stretchr/testify/assert"
)

func Test_ConnectorName(t *testing.T) {
	for in, want := range map[string]string{
		"github-mcp-server":     "github-mcp-server",
		"GitHub":                "github",
		"Notion MCP":            "notion-mcp",
		"Atlassian Rovo  (MCP)": "atlassian-rovo-mcp",
		"mcp.example":           "mcp-example",
		"123 Go":                "go",
		"snake_case_server":     "snake_case_server",
		"Café":                  "caf",
		"":                      schema.DefaultConnectorName,
		"!!!":                   schema.DefaultConnectorName,
		"a very long server name which goes on and on and on, and on": "a-very-long-server-name-which-goes-on-and-on-and",
	} {
		got := schema.ConnectorName(in)
		assert.Equal(t, want, got, in)
		assert.True(t, types.IsIdentifier(got), got)
		assert.True(t, types.IsIdentifier(schema.NumberedName(got, 99)), got)
	}
}

func Test_HostName(t *testing.T) {
	assert.Equal(t, "seolinkmap", schema.HostName("https://seolinkmap.com/mcp"))
	assert.Equal(t, "mcp.notion", schema.HostName("https://mcp.notion.com/mcp"))
	assert.Equal(t, "example", schema.HostName("https://www.example.com"))
	assert.Equal(t, "localhost", schema.HostName("http://localhost:8080/mcp"))
	assert.Equal(t, "", schema.HostName("::"))
}

func Test_NumberedName(t *testing.T) {
	assert.Equal(t, "github", schema.NumberedName("github", 1))
	assert.Equal(t, "github-2", schema.NumberedName("github", 2))
}
