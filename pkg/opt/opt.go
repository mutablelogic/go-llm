package opt

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// A generic option type, which can set options on an agent or session
type Opt func(*opts) error

// set of options
type opts struct {
	url.Values
	progress ProgressFn
}

// ProgressFn is a callback function for progress updates
// status: descriptive status message (e.g., "downloading", "verifying")
// percent: progress percentage (0-100)
type ProgressFn func(status string, percent float64)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// Apply returns a structure of applied options
func Apply(o ...Opt) (*opts, error) {
	opts := &opts{
		Values: make(url.Values),
	}
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
		if value, ok := o.Values[key]; ok {
			query[key] = value
		}
	}
	return query
}

// GetString returns the trimmed value for key, or empty string if not set
func (o *opts) GetString(key string) string {
	if values, ok := o.Values[key]; ok && len(values) > 0 {
		return strings.TrimSpace(values[0])
	}
	return ""
}

// GetStringArray returns all values for key, each trimmed
func (o *opts) GetStringArray(key string) []string {
	values, ok := o.Values[key]
	if !ok {
		return nil
	}
	result := make([]string, len(values))
	for i, v := range values {
		result[i] = strings.TrimSpace(v)
	}
	return result
}

// GetBool returns true if key is present, false if absent
func (o *opts) GetBool(key string) bool {
	_, ok := o.Values[key]
	return ok
}

// GetFloat64 returns the float64 value for key, or 0 if not set or invalid
func (o *opts) GetFloat64(key string) float64 {
	if values, ok := o.Values[key]; ok && len(values) > 0 {
		if v, err := strconv.ParseFloat(strings.TrimSpace(values[0]), 64); err == nil {
			return v
		}
	}
	return 0
}

// GetUint returns the uint value for key, or 0 if not set or invalid
func (o *opts) GetUint(key string) uint {
	if values, ok := o.Values[key]; ok && len(values) > 0 {
		if v, err := strconv.ParseUint(strings.TrimSpace(values[0]), 10, 64); err == nil {
			return uint(v)
		}
	}
	return 0
}

// Has returns true if the key exists
func (o *opts) Has(key string) bool {
	switch key {
	case "progressfn":
		return o.progress != nil
	default:
		_, ok := o.Values[key]
		return ok
	}
}

// Get returns the arbitrary value for key, or nil if not set
func (o *opts) Get(key string) any {
	switch key {
	case "progressfn":
		return o.progress != nil
	default:
		return o.Values.Get(key)
	}
}

// Set stores an arbitrary value for key
func (o *opts) Set(key string, value any) error {
	switch key {
	case "progressfn":
		if fn, ok := value.(ProgressFn); !ok || fn == nil {
			return fmt.Errorf("progressfn must be a non-nil ProgressFn")
		} else {
			o.progress = fn
		}
		return nil
	default:
		if value == nil {
			o.Values.Del(key)
		} else {
			o.Values.Set(key, fmt.Sprintf("%v", value))
		}
		return nil
	}
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
		for _, v := range value {
			o.Values.Add(key, v)
		}
		return nil
	}
}

// SetUint sets a uint value for key, replacing any existing values
func SetUint(key string, value uint) Opt {
	return func(o *opts) error {
		return o.Set(key, value)
	}
}

// AddUint appends uint values for key, preserving any existing values
func AddUint(key string, value ...uint) Opt {
	return func(o *opts) error {
		for _, v := range value {
			o.Values.Add(key, fmt.Sprintf("%d", v))
		}
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
		o.Values.Add(key, strconv.FormatFloat(value, 'f', -1, 64))
		return nil
	}
}

// SetBool sets a boolean value for key, replacing any existing values
func SetBool(key string, value bool) Opt {
	return func(o *opts) error {
		return o.Set(key, strconv.FormatBool(value))
	}
}

////////////////////////////////////////////////////////////////////////////////
// CALLBACK TYPES

// WithProgress sets a progress callback function
func WithProgress(fn ProgressFn) Opt {
	return func(o *opts) error {
		return o.Set("progressfn", fn)
	}
}

// GetProgress returns the progress callback function, or nil if not set
func (o *opts) GetProgress() ProgressFn {
	return o.progress
}
