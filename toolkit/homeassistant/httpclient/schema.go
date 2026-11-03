package httpclient

import (
	"strings"
	"time"

	// Packages
	types "github.com/mutablelogic/go-server/pkg/types"
)

////////////////////////////////////////////////////////////////////////////////
// RESPONSE TYPES

// State is the state of an entity
type State struct {
	Entity       string         `json:"entity_id"`
	LastChanged  time.Time      `json:"last_changed"`
	LastReported time.Time      `json:"last_reported"`
	LastUpdated  time.Time      `json:"last_updated"`
	State        string         `json:"state"`
	Attributes   map[string]any `json:"attributes"`
	Context      struct {
		Id       string `json:"id,omitempty"`
		ParentId string `json:"parent_id,omitempty"`
		UserId   string `json:"user_id,omitempty"`
	} `json:"context"`
}

// Domain is a domain, such as "light", and its services, by name
type Domain struct {
	Domain   string              `json:"domain"`
	Services map[string]*Service `json:"services,omitempty"`
}

// Service is a service which can be called for a domain
type Service struct {
	Call        string           `json:"call,omitempty"`
	Name        string           `json:"name,omitempty"`
	Description string           `json:"description,omitempty"`
	Fields      map[string]Field `json:"fields,omitempty"`
}

// Field is a field of the data for a service
type Field struct {
	Required bool                `json:"required,omitempty"`
	Example  any                 `json:"example,omitempty"`
	Selector map[string]Selector `json:"selector,omitempty"`
}

// Selector describes the values of a field
type Selector struct {
	Text              string  `json:"text,omitempty"`
	Mode              string  `json:"mode,omitempty"`
	Min               float32 `json:"min,omitempty"`
	Max               float32 `json:"max,omitempty"`
	UnitOfMeasurement string  `json:"unit_of_measurement,omitempty"`
}

// CallResponse is the states which changed when calling a service, and the
// service's response data
type CallResponse struct {
	ChangedStates   []*State       `json:"changed_states"`
	ServiceResponse map[string]any `json:"service_response"`
}

// Event is an event type, and the number of its listeners
type Event struct {
	Event     string `json:"event"`
	Listeners uint   `json:"listener_count"`
}

// Config is the server's configuration
type Config struct {
	Components    []string       `json:"components"`
	ConfigDir     string         `json:"config_dir"`
	Elevation     float64        `json:"elevation"`
	Latitude      float64        `json:"latitude"`
	Longitude     float64        `json:"longitude"`
	LocationName  string         `json:"location_name"`
	TimeZone      string         `json:"time_zone"`
	UnitSystem    map[string]any `json:"unit_system"`
	Version       string         `json:"version"`
	ExternalDirs  []string       `json:"whitelist_external_dirs,omitempty"`
	AllowlistDirs []string       `json:"allowlist_external_dirs,omitempty"`
	AllowlistURLs []string       `json:"allowlist_external_urls,omitempty"`
	Currency      string         `json:"currency,omitempty"`
	Country       string         `json:"country,omitempty"`
	Language      string         `json:"language,omitempty"`
	SafeMode      bool           `json:"safe_mode,omitempty"`
	State         string         `json:"state,omitempty"`
	InternalURL   string         `json:"internal_url,omitempty"`
	ExternalURL   string         `json:"external_url,omitempty"`
	RecoveryMode  bool           `json:"recovery_mode,omitempty"`
}

////////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (s State) String() string {
	return types.Stringify(s)
}

func (d Domain) String() string {
	return types.Stringify(d)
}

func (s Service) String() string {
	return types.Stringify(s)
}

func (f Field) String() string {
	return types.Stringify(f)
}

func (c Config) String() string {
	return types.Stringify(c)
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Domain is the domain of the entity, such as "light" for "light.kitchen",
// which determines the services which can be called for it
func (s State) Domain() string {
	if domain, _, ok := strings.Cut(s.Entity, "."); ok {
		return domain
	}
	return ""
}

// Name is the friendly name of the entity, or its ID if it has none
func (s State) Name() string {
	if name, ok := s.Attributes["friendly_name"].(string); ok {
		return name
	}
	return s.Entity
}

// Value is the state of the entity, or empty if it's unavailable or unknown
func (s State) Value() string {
	switch strings.ToLower(s.State) {
	case "unavailable", "unknown", "--":
		return ""
	default:
		return s.State
	}
}

// Class is how the state should be interpreted, which is the entity's device
// class, or its domain if it has none
func (s State) Class() string {
	class, exists := s.Attributes["device_class"]
	if !exists {
		return s.Domain()
	} else if class, ok := class.(string); ok {
		return class
	}
	return ""
}

// UnitOfMeasurement is the unit of the state, or empty if it has none
func (s State) UnitOfMeasurement() string {
	if unit, ok := s.Attributes["unit_of_measurement"].(string); ok {
		return unit
	}
	return ""
}
