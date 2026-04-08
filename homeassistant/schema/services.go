package schema

import "github.com/mutablelogic/go-server/pkg/types"

///////////////////////////////////////////////////////////////////////////////
// TYPES

type Domain struct {
	Domain   string              `json:"domain"`
	Services map[string]*Service `json:"services,omitempty"`
}

type Service struct {
	Call        string           `json:"call,omitempty"`
	Name        string           `json:"name,omitempty"`
	Description string           `json:"description,omitempty"`
	Fields      map[string]Field `json:"fields,omitempty"`
}

type Field struct {
	Required bool                `json:"required,omitempty"`
	Example  any                 `json:"example,omitempty"`
	Selector map[string]Selector `json:"selector,omitempty"`
}

type Selector struct {
	Text              string  `json:"text,omitempty"`
	Mode              string  `json:"mode,omitempty"`
	Min               float32 `json:"min,omitempty"`
	Max               float32 `json:"max,omitempty"`
	UnitOfMeasurement string  `json:"unit_of_measurement,omitempty"`
}

// CallResponse is returned when calling a service with return_response=true.
type CallResponse struct {
	ChangedStates   []*State       `json:"changed_states"`
	ServiceResponse map[string]any `json:"service_response"`
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (v Domain) String() string {
	return types.Stringify(v)
}

func (v Service) String() string {
	return types.Stringify(v)
}

func (v Field) String() string {
	return types.Stringify(v)
}
