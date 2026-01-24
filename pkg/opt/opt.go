package opt

import (
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"strings"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// A generic option type, which can set options on an agent or session
type Opt func(*opts) error

// set of options
type opts struct {
	values   map[string]any
	progress ProgressFn
}

// Options is the interface for accessing options
type Options interface {
	// Returns true if the key exists
	Has(key string) bool

	// Return a value for key, or nil
	Get(key string) any

	// Get a string value for key
	GetString(key string) string

	// Get a string array for key
	GetStringArray(key string) []string

	// Get a boolean value for key
	GetBool(key string) bool

	// Get a float64 value for key
	GetFloat64(key string) float64

	// Get a uint value for key
	GetUint(key string) uint

	// Return a set of keys as a url.Values
	Query(keys ...string) url.Values
}

// ProgressFn is a callback function for progress updates
// status: descriptive status message (e.g., "downloading", "verifying")
// percent: progress percentage (0-100)
type ProgressFn func(status string, percent float64)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// Apply returns a structure of applied options
func Apply(o ...Opt) (*opts, error) {
	opts := &opts{values: make(map[string]any)}
	for _, opt := range o {
		if err := opt(opts); err != nil {
			return nil, err
		}
	}
	return opts, nil
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (o *opts) Query(keys ...string) url.Values {
	query := make(url.Values)
	for _, key := range keys {
		if o.Has(key) {
			query[key] = o.GetStringArray(key)
		}
	}
	return query
}

// GetString returns the trimmed value for key, or empty string if not set
func (o *opts) GetString(key string) string {
	v := o.Get(key)
	if v == nil {
		return ""
	}
	if arr, ok := v.([]string); ok {
		if len(arr) == 0 {
			return ""
		}
		return strings.TrimSpace(arr[0])
	}
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

// GetStringArray returns all values for key, each trimmed
func (o *opts) GetStringArray(key string) []string {
	v := o.Get(key)
	if v == nil {
		return nil
	}
	var result []string
	switch val := v.(type) {
	case []string:
		result = append([]string(nil), val...)
	case string:
		result = []string{val}
	default:
		return nil
	}
	for i, s := range result {
		result[i] = strings.TrimSpace(s)
	}
	return result
}

// GetBool returns true if key is present, false if absent
func (o *opts) GetBool(key string) bool {
	v := o.Get(key)
	if v == nil {
		return false
	}
	if b, ok := v.(bool); ok {
		return b
	}
	if arr, ok := v.([]string); ok {
		if len(arr) > 0 {
			if b, err := strconv.ParseBool(strings.TrimSpace(arr[0])); err == nil {
				return b
			}
		}
	}
	if s, ok := v.(string); ok {
		if b, err := strconv.ParseBool(strings.TrimSpace(s)); err == nil {
			return b
		}
	}
	return false
}

// GetFloat64 returns the float64 value for key, or 0 if not set or invalid
func (o *opts) GetFloat64(key string) float64 {
	v := o.Get(key)
	if v == nil {
		return 0
	}
	if arr, ok := v.([]string); ok {
		if len(arr) > 0 {
			if f, err := strconv.ParseFloat(strings.TrimSpace(arr[0]), 64); err == nil {
				return f
			}
		}
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Float32, reflect.Float64:
		return rv.Float()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(rv.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(rv.Uint())
	case reflect.String:
		f, err := strconv.ParseFloat(strings.TrimSpace(rv.String()), 64)
		if err == nil {
			return f
		}
	}
	return 0
}

// GetUint returns the uint value for key, or 0 if not set or invalid
func (o *opts) GetUint(key string) uint {
	v := o.Get(key)
	if v == nil {
		return 0
	}
	if arr, ok := v.([]string); ok {
		if len(arr) > 0 {
			if u, err := strconv.ParseUint(strings.TrimSpace(arr[0]), 10, 64); err == nil {
				return uint(u)
			}
		}
	}
	switch val := v.(type) {
	case uint:
		return val
	case uint8:
		return uint(val)
	case uint16:
		return uint(val)
	case uint32:
		return uint(val)
	case uint64:
		return uint(val)
	case int:
		if val < 0 {
			return 0
		}
		return uint(val)
	case int8:
		if val < 0 {
			return 0
		}
		return uint(val)
	case int16:
		if val < 0 {
			return 0
		}
		return uint(val)
	case int32:
		if val < 0 {
			return 0
		}
		return uint(val)
	case int64:
		if val < 0 {
			return 0
		}
		return uint(val)
	case string:
		if u, err := strconv.ParseUint(strings.TrimSpace(val), 10, 64); err == nil {
			return uint(u)
		}
	}
	return 0
}

// Has returns true if the key exists
func (o *opts) Has(key string) bool {
	_, ok := o.values[key]
	return ok
}

// Get returns the arbitrary value for key, or nil if not set
func (o *opts) Get(key string) any {
	// Check arbitrary map first (for non-string objects)
	if val, ok := o.values[key]; ok {
		return val
	} else {
		return nil
	}
}

// Set stores an arbitrary value for key
func (o *opts) Set(key string, value any) error {
	if value == nil {
		delete(o.values, key)
	} else {
		o.values[key] = value
	}
	return nil
}

////////////////////////////////////////////////////////////////////////////////
// OPTIONS

// Error returns an option that always returns an error
func Error(err error) Opt {
	return func(o *opts) error {
		return err
	}
}

// WithOpts combines multiple options into a single option
func WithOpts(options ...Opt) Opt {
	return func(o *opts) error {
		for _, opt := range options {
			if err := opt(o); err != nil {
				return err
			}
		}
		return nil
	}
}

// SetString sets a string value for key, replacing any existing values
func SetString(key string, value string) Opt {
	return func(o *opts) error {
		return o.Set(key, value)
	}
}

// AddString appends string values for key, preserving any existing values
func AddString(key string, value ...string) Opt {
	return func(o *opts) error {
		existing, _ := o.values[key].([]string)
		o.values[key] = append(existing, value...)
		return nil
	}
}

// SetUint sets a uint value for key, replacing any existing values
func SetUint(key string, value uint) Opt {
	return func(o *opts) error {
		return o.Set(key, fmt.Sprintf("%d", value))
	}
}

// AddUint appends uint values for key, preserving any existing values
func AddUint(key string, value ...uint) Opt {
	return func(o *opts) error {
		existing, _ := o.values[key].([]string)
		for _, v := range value {
			existing = append(existing, fmt.Sprintf("%d", v))
		}
		o.values[key] = existing
		return nil
	}
}

// SetFloat64 sets a float64 value for key, replacing any existing values
func SetFloat64(key string, value float64) Opt {
	return func(o *opts) error {
		return o.Set(key, strconv.FormatFloat(value, 'f', -1, 64))
	}
}

// AddFloat64 appends a float64 value for key, preserving any existing values
func AddFloat64(key string, value float64) Opt {
	return func(o *opts) error {
		existing, _ := o.values[key].([]string)
		existing = append(existing, strconv.FormatFloat(value, 'f', -1, 64))
		o.values[key] = existing
		return nil
	}
}

// SetBool sets a boolean value for key, replacing any existing values
func SetBool(key string, value bool) Opt {
	return func(o *opts) error {
		return o.Set(key, strconv.FormatBool(value))
	}
}

// WithToolkit sets a toolkit for the options (any to avoid import cycles)
func WithToolkit(toolkit any) Opt {
	return func(o *opts) error {
		return o.Set("toolkit", toolkit)
	}
}

// GetToolkit returns the toolkit if set, or nil
func (o *opts) GetToolkit() any {
	return o.Get("toolkit")
}

////////////////////////////////////////////////////////////////////////////////
// CALLBACK TYPES

// WithProgress sets a progress callback function
func WithProgress(fn ProgressFn) Opt {
	return func(o *opts) error {
		o.progress = fn
		return nil
	}
}

// GetProgress returns the progress callback function, or nil if not set
func (o *opts) GetProgress() ProgressFn {
	return o.progress
}
