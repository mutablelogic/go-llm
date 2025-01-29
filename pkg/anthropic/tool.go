package anthropic

import (
	"encoding/json"
	"reflect"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  struct {
		Type       string                   `json:"type,omitempty"`
		Required   []string                 `json:"required,omitempty"`
		Properties map[string]ToolParameter `json:"properties,omitempty"`
	} `json:"input_schema"`
	proto reflect.Type // Prototype for parameter return
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
// LIFECYCLE

// Return a tool, or panic if there is an error
func MustTool(name, description string, params any) *Tool {
	tool, err := NewTool(name, description, params)
	if err != nil {
		panic(err)
	}
	return tool
}

// Return a new tool definition
func NewTool(name, description string, params any) (*Tool, error) {
	tool := Tool{
		Name:        name,
		Description: description,
		proto:       reflect.TypeOf(params),
	}

	// Add parameters
	tool.Parameters.Type = "object"
	toolparams, err := paramsFor(params)
	if err != nil {
		return nil, err
	}

	// Set parameters
	tool.Parameters.Required = make([]string, 0, len(toolparams))
	tool.Parameters.Properties = make(map[string]ToolParameter, len(toolparams))
	for _, param := range toolparams {
		if _, exists := tool.Parameters.Properties[param.Name]; exists {
			return nil, llm.ErrConflict.Withf("parameter %q already exists", param.Name)
		} else {
			tool.Parameters.Properties[param.Name] = param
		}
		if param.required {
			tool.Parameters.Required = append(tool.Parameters.Required, param.Name)
		}
	}

	// Return success
	return &tool, nil
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (t Tool) String() string {
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(data)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// Return tool parameters from a struct
func paramsFor(params any) ([]ToolParameter, error) {
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
