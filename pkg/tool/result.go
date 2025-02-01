package tool

import (
	// Packages
	"encoding/json"

	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type result struct {
	call  llm.ToolCall
	value any
}

var _ llm.ToolResult = (*result)(nil)

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (r result) String() string {
	type j struct {
		Call   llm.ToolCall `json:"call"`
		Result any          `json:"result"`
	}
	data, err := json.MarshalIndent(j{r.call, r.value}, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(data)
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// The call associated with the result
func (r result) Call() llm.ToolCall {
	return r.call
}

// The result, which can be encoded into json
func (r result) Result() any {
	return r.value
}
