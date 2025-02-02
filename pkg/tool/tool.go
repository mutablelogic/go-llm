package tool

import (
	"encoding/json"
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
	Required   []string                 `json:"required"`
	Properties map[string]ToolParameter `json:"properties"`
}

type ToolParameter struct {
	Name        string   `json:"-"`
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Enum        []string `json:"enum,omitempty"`
	required    bool
	index       []int // Field index into prototype for setting a field
}

type ToolFunction struct {
	Type     string `json:"type"` // function
	llm.Tool `json:"function"`
}

////////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (t tool) String() string {
	data, err := json.MarshalIndent(t.ToolMeta, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(data)
}

func (t ToolFunction) String() string {
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(data)
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
func paramsFor(root []int, params any) ([]ToolParameter, error) {
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

	return paramsForStruct(root, rt)
}

func paramsForStruct(root []int, rt reflect.Type) ([]ToolParameter, error) {
	result := make([]ToolParameter, 0, rt.NumField())

	// Iterate over fields
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)

		// Ignore unexported fields
		name := fieldName(field)
		if name == "" {
			continue
		}

		// Recurse into struct
		ft := field.Type
		if ft.Kind() == reflect.Ptr {
			ft = field.Type.Elem()
		}
		if ft.Kind() == reflect.Struct {
			if param, err := paramsForStruct(append(root, field.Index...), ft); err != nil {
				return nil, err
			} else {
				result = append(result, param...)
			}
			continue
		}

		// Determine parameter
		if param, err := paramFor(root, field); err != nil {
			return nil, err
		} else {
			result = append(result, param)
		}
	}

	// Return success
	return result, nil
}

// Return tool parameters from a struct field
func paramFor(root []int, field reflect.StructField) (ToolParameter, error) {
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
		for _, e := range strings.Split(enum_, ",") {
			enum = append(enum, strings.TrimSpace(e))
		}
	}

	// Return success
	return ToolParameter{
		Name:        fieldName(field),
		Type:        typ,
		Description: field.Tag.Get("help"),
		Enum:        enum,
		required:    required,
		index:       append(root, field.Index...),
	}, nil
}

// Return the name field, or empty name if field
// should be ignored
func fieldName(field reflect.StructField) string {
	name, exists := field.Tag.Lookup("name")
	if !exists {
		name, exists = field.Tag.Lookup("json")
		if names := strings.Split(name, ","); exists && len(names) > 0 {
			name = names[0]
		}
	}
	if !exists {
		name = field.Name
	} else if name == "-" {
		return ""
	}
	return name
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
