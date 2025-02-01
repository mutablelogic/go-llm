package tool

import (
	// Packages
	"encoding/json"

	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type ResultMeta struct {
	Call  llm.ToolCall `json:"call"`
	Value any          `json:"result"`
}

type result struct {
	meta ResultMeta
}

var _ llm.ToolResult = (*result)(nil)

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func NewResult(call llm.ToolCall, value any) llm.ToolResult {
	return &result{
		meta: ResultMeta{
			Call:  call,
			Value: value,
		},
	}
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (r result) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.meta)
}

func (r result) String() string {
	data, err := json.MarshalIndent(r.meta, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(data)
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// The call associated with the result
func (r result) Call() llm.ToolCall {
	return r.meta.Call
}

// The result, which can be encoded into json
func (r result) Value() any {
	return r.meta.Value
}
