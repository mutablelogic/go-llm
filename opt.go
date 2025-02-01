package llm

import (
	"io"
	"time"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// A generic option type, which can set options on an agent or session
type Opt func(*Opts) error

// set of options
type Opts struct {
	agents      map[string]Agent // Set of agents
	toolkit     ToolKit          // Toolkit for tools
	callback    func(Context)    // Streaming callback
	attachments []*Attachment    // Attachments
	options     map[string]any   // Additional options
}

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// ApplyOpts returns a structure of options
func ApplyOpts(opts ...Opt) (*Opts, error) {
	o := new(Opts)
	o.agents = make(map[string]Agent)
	o.options = make(map[string]any)
	for _, opt := range opts {
		if err := opt(o); err != nil {
			return nil, err
		}
	}
	return o, nil
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - PROPERTIES

// Return the set of tools
func (o *Opts) ToolKit() ToolKit {
	return o.toolkit
}

// Return the stream function
func (o *Opts) StreamFn() func(Context) {
	return o.callback
}

// Return the array of registered agents
func (o *Opts) Agents() []Agent {
	result := make([]Agent, 0, len(o.agents))
	for _, agent := range o.agents {
		result = append(result, agent)
	}
	return result
}

// Return attachments
func (o *Opts) Attachments() []*Attachment {
	return o.attachments
}

// Set an option value
func (o *Opts) Set(key string, value any) {
	o.options[key] = value
}

// Get an option value
func (o *Opts) Get(key string) any {
	if value, exists := o.options[key]; exists {
		return value
	}
	return nil
}

// Has an option value
func (o *Opts) Has(key string) bool {
	_, exists := o.options[key]
	return exists
}

// Get an option value as a string
func (o *Opts) GetString(key string) string {
	if value, exists := o.options[key]; exists {
		if v, ok := value.(string); ok {
			return v
		}
	}
	return ""
}

// Get an option value as a boolean
func (o *Opts) GetBool(key string) bool {
	if value, exists := o.options[key]; exists {
		if v, ok := value.(bool); ok {
			return v
		}
	}
	return false
}

// Get an option value as a duration
func (o *Opts) GetDuration(key string) time.Duration {
	if value, exists := o.options[key]; exists {
		if v, ok := value.(time.Duration); ok {
			return v
		}
	}
	return 0
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - SET OPTIONS

// Set toolkit of tools
func WithToolKit(toolkit ToolKit) Opt {
	return func(o *Opts) error {
		o.toolkit = toolkit
		return nil
	}
}

// Set chat streaming function
func WithStream(fn func(Context)) Opt {
	return func(o *Opts) error {
		o.callback = fn
		return nil
	}
}

// Set agent
func WithAgent(agent Agent) Opt {
	return func(o *Opts) error {
		// Check parameters
		if agent == nil {
			return ErrBadParameter.With("withAgent")
		}

		// Add agent
		name := agent.Name()
		if _, exists := o.agents[name]; exists {
			return ErrConflict.Withf("Agent %q already exists", name)
		} else {
			o.agents[name] = agent
		}

		// Return success
		return nil
	}

}

// Create an attachment
func WithAttachment(r io.Reader) Opt {
	return func(o *Opts) error {
		if attachment, err := ReadAttachment(r); err != nil {
			return err
		} else {
			o.attachments = append(o.attachments, attachment)
		}
		return nil
	}
}

// The temperature of the model. Increasing the temperature will make the model answer more creatively.
func WithTemperature(v float64) Opt {
	return func(o *Opts) error {
		if v < 0.0 || v > 1.0 {
			return ErrBadParameter.With("temperature must be between 0.0 and 1.0")
		}
		o.Set("temperature", v)
		return nil
	}
}

// Works together with top-k. A higher value (e.g., 0.95) will lead to more diverse text, while
// a lower value (e.g., 0.5) will generate more focused and conservative text.
func WithTopP(v float64) Opt {
	return func(o *Opts) error {
		if v < 0.0 || v > 1.0 {
			return ErrBadParameter.With("top_p must be between 0.0 and 1.0")
		}
		o.Set("top_p", v)
		return nil
	}
}

// Reduces the probability of generating nonsense. A higher value (e.g. 100) will give more
// diverse answers, while a lower value (e.g. 10) will be more conservative.
func WithTopK(v uint) Opt {
	return func(o *Opts) error {
		o.Set("top_k", v)
		return nil
	}
}
