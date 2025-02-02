package mistral

import (
	"bytes"
	"encoding/json"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type ToolCall struct {
	Id       string `json:"id,omitempty"`    // tool id
	Index    uint64 `json:"index,omitempty"` // tool index
	Function struct {
		Name      string `json:"name,omitempty"`      // tool name
		Arguments string `json:"arguments,omitempty"` // tool arguments
	}
}

type toolcall struct {
	meta ToolCall
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

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
// PUBLIC METHODS - TOOL CALL

func (t toolcall) Id() string {
	return t.meta.Id
}

// The tool name
func (t toolcall) Name() string {
	return t.meta.Function.Name
}

// Decode the calling parameters
func (t toolcall) Decode(v any) error {
	var buf bytes.Buffer
	buf.WriteString(t.meta.Function.Arguments)
	return json.NewDecoder(&buf).Decode(v)
}
