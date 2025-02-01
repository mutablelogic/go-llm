package tool

import (
	// Packages

	"encoding/json"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type CallMeta struct {
	Name  string         `json:"name"`
	Id    string         `json:"id,omitempty"`
	Input map[string]any `json:"input,omitempty"`
}

type call struct {
	meta CallMeta
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func NewCall(id, name string, input map[string]any) *call {
	return &call{
		meta: CallMeta{
			Name:  name,
			Id:    id,
			Input: input,
		},
	}
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (t call) String() string {
	data, err := json.MarshalIndent(t.meta, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(data)
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (t call) Name() string {
	return t.meta.Name
}

func (t call) Id() string {
	return t.meta.Id
}

func (t call) Decode(v any) error {
	if data, err := json.Marshal(t.meta.Input); err != nil {
		return err
	} else {
		return json.Unmarshal(data, v)
	}
}
