package mistral

import (
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
	} `json:"function"`
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
