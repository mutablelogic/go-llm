package schema

import (
	"encoding/json"

	// Packages
	yaml "gopkg.in/yaml.v3"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// JSONSchema is a JSON-encoded schema that supports unmarshalling from both
// JSON and YAML sources. When unmarshalling from YAML, the YAML node is first
// decoded to a native Go value and then marshalled to JSON bytes.
type JSONSchema json.RawMessage

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewJSONSchema creates a JSONSchema from raw JSON bytes.
func NewJSONSchema(data json.RawMessage) JSONSchema {
	return JSONSchema(data)
}

////////////////////////////////////////////////////////////////////////////////
// METHODS

// Bytes returns the underlying JSON bytes.
func (s JSONSchema) Bytes() []byte {
	return []byte(s)
}

////////////////////////////////////////////////////////////////////////////////
// JSON MARSHALLING

func (s JSONSchema) MarshalJSON() ([]byte, error) {
	if len(s) == 0 {
		return []byte("null"), nil
	}
	return []byte(s), nil
}

func (s *JSONSchema) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*s = nil
		return nil
	}
	*s = append((*s)[:0], data...)
	return nil
}

////////////////////////////////////////////////////////////////////////////////
// YAML UNMARSHALLING

func (s *JSONSchema) UnmarshalYAML(node *yaml.Node) error {
	var v any
	if err := node.Decode(&v); err != nil {
		return err
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	*s = data
	return nil
}
