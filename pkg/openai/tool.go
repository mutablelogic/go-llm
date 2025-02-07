package openai

import (
	"encoding/json"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type ToolCall struct {
	Id       string `json:"id,omitempty"`    // tool id
	Type     string `json:"type,omitempty"`  // tool type (function)
	Index    uint64 `json:"index,omitempty"` // tool index
	Function struct {
		Name      string `json:"name,omitempty"`      // tool name
		Arguments string `json:"arguments,omitempty"` // tool arguments
	} `json:"function"`
}

type toolcall struct {
	meta ToolCall
}

type ToolCalls []toolcall

type ToolResults struct {
	Id string `json:"tool_call_id,omitempty"`
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (t *toolcall) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &t.meta)
}

func (t toolcall) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.meta)
}

func (t toolcall) String() string {
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(data)
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// The tool name
func (t toolcall) Name() string {
	return t.meta.Function.Name
}

// The tool identifier
func (t toolcall) Id() string {
	return t.meta.Id
}

// Decode the calling parameters
func (t toolcall) Decode(v any) error {
	return json.Unmarshal([]byte(t.meta.Function.Arguments), v)
}
