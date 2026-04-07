package schema

import (
	_ "embed"

	// Packages
	llmschema "github.com/mutablelogic/go-llm/kernel/schema"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

//go:embed objects.sql
var Objects string

//go:embed queries.sql
var Queries string

const (
	DefaultMemorySchema = "memory"
	DefaultLLMSchema    = llmschema.DefaultSchema
	DefaultAuthSchema   = llmschema.DefaultAuthSchema
)
