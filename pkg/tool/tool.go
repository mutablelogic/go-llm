package tool

import (
	"fmt"
	"reflect"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type tool struct {
	llm.Tool `json:"-"`
	ToolMeta
}

var _ llm.Tool = (*tool)(nil)

type ToolMeta struct {
	Name        string `json:"name"`
	Description string `json:"description"`

	// Variation on how schema is output
	Parameters  *ToolParameters `json:"parameters,omitempty"`
	InputSchema *ToolParameters `json:"input_schema,omitempty"`
}

type ToolParameters struct {
	Type       string                   `json:"type,omitempty"`
	Required   []string                 `json:"required,omitempty"`
	Properties map[string]ToolParameter `json:"properties,omitempty"`
}

type ToolParameter struct {
	Name        string   `json:"-"`
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Enum        []string `json:"enum,omitempty"`
	required    bool
	index       []int // Field index into prototype for setting a field
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (t tool) Name() string {
	return t.ToolMeta.Name
}

func (t tool) Description() string {
	return t.ToolMeta.Description
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// Return tool parameters from a struct
func paramsFor(params any) ([]ToolParameter, error) {
	if params == nil {
		return []ToolParameter{}, nil
	}
	rt := reflect.TypeOf(params)
	if rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}
	if rt.Kind() != reflect.Struct {
		return nil, llm.ErrBadParameter.With("params must be a struct")
	}

	// Iterate over fields
	fields := reflect.VisibleFields(rt)
	result := make([]ToolParameter, 0, len(fields))
	for _, field := range fields {
		fmt.Println(field.Name, "=>", field.Index)
		// Ignore unexported fields
		name := field.Tag.Get("json")
		if name == "-" {
			continue
		}

		// Determine parameter
		if param, err := paramFor(field); err != nil {
			return nil, err
		} else {
			result = append(result, param)
		}
	}

	// Return success
	return result, nil
}

// Return tool parameters from a struct field
func paramFor(field reflect.StructField) (ToolParameter, error) {
	// Name
	name := field.Tag.Get("name")
	if name == "" {
		name = field.Name
	}

	// Type
	typ, err := paramType(field)
	if err != nil {
		return ToolParameter{}, err
	}

	// Required
	_, required := field.Tag.Lookup("required")

	// Enum
	enum := []string{}
	if enum_ := field.Tag.Get("enum"); enum_ != "" {
		enum = strings.Split(enum_, ",")
	}

	// Return success
	return ToolParameter{
		Name:        field.Name,
		Type:        typ,
		Description: field.Tag.Get("help"),
		Enum:        enum,
		required:    required,
		index:       field.Index,
	}, nil
}

var (
	typeString  = reflect.TypeOf("")
	typeUint    = reflect.TypeOf(uint(0))
	typeInt     = reflect.TypeOf(int(0))
	typeFloat64 = reflect.TypeOf(float64(0))
	typeFloat32 = reflect.TypeOf(float32(0))
)

// Return parameter type from a struct field
func paramType(field reflect.StructField) (string, error) {
	t := field.Type
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	switch field.Type {
	case typeString:
		return "string", nil
	case typeUint, typeInt:
		return "integer", nil
	case typeFloat64, typeFloat32:
		return "number", nil
	default:
		return "", llm.ErrBadParameter.Withf("unsupported type %v for field %q", field.Type, field.Name)
	}
}
