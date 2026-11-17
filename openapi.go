package llm

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// TagGroup is the OpenAPI tag group of the server's tags, such as the
	// registry's, the toolkit's, and the broadcaster's, which Redoc shows
	// them in, as it doesn't show a tag outside a group
	TagGroup = "LLM"

	// ToolsAndAgentsTag is the OpenAPI tag of the toolkit's tools, and the
	// registry's agents, which models call
	ToolsAndAgentsTag = "Tools & Agents"
)
