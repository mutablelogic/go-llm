package llm

import (
	"encoding/json"
	"io"
	"time"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// A generic option type, which can set options on an agent or session
type Opt func(*Opts) error

// set of options
type Opts struct {
	prompt      bool
	agents      map[string]Agent // Set of agents
	toolkit     ToolKit          // Toolkit for tools
	callback    func(Completion) // Streaming callback
	attachments []*Attachment    // Attachments
	system      string           // System prompt
	options     map[string]any   // Additional options
}

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// ApplyOpts returns a structure of options
func ApplyOpts(opts ...Opt) (*Opts, error) {
	return applyOpts(false, opts...)
}

// ApplyPromptOpts returns a structure of options for a prompt
func ApplyPromptOpts(opts ...Opt) (*Opts, error) {
	if opt, err := applyOpts(true, opts...); err != nil {
		return nil, err
	} else {
		return opt, nil
	}
}

// ApplySessionOpts returns a structure of options
func applyOpts(prompt bool, opts ...Opt) (*Opts, error) {
	o := new(Opts)
	o.prompt = prompt
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
// STRINGIFY

func (o Opts) MarshalJSON() ([]byte, error) {
	var j struct {
		ToolKit     ToolKit          `json:"toolkit,omitempty"`
		Agents      map[string]Agent `json:"agents,omitempty"`
		System      string           `json:"system,omitempty"`
		Attachments []*Attachment    `json:"attachments,omitempty"`
		Options     map[string]any   `json:"options,omitempty"`
	}
	j.ToolKit = o.toolkit
	j.Agents = o.agents
	j.Attachments = o.attachments
	j.System = o.system
	j.Options = o.options
	return json.Marshal(j)
}

func (o Opts) String() string {
	data, err := json.Marshal(o)
	if err != nil {
		return err.Error()
	}
	return string(data)
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - PROPERTIES

// Return the set of tools
func (o *Opts) ToolKit() ToolKit {
	return o.toolkit
}

// Return the stream function
func (o *Opts) StreamFn() func(Completion) {
	return o.callback
}

// Return the system prompt
func (o *Opts) SystemPrompt() string {
	return o.system
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

// Get an option value as an unsigned integer
func (o *Opts) GetUint64(key string) uint64 {
	if value, exists := o.options[key]; exists {
		if v, ok := value.(uint64); ok {
			return v
		}
	}
	return 0
}

// Get an option value as a float64
func (o *Opts) GetFloat64(key string) float64 {
	if value, exists := o.options[key]; exists {
		if v, ok := value.(float64); ok {
			return v
		}
	}
	return 0
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
func WithStream(fn func(Completion)) Opt {
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
		// Only attach if prompt is set
		if !o.prompt {
			return nil
		}
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
func WithTopK(v uint64) Opt {
	return func(o *Opts) error {
		o.Set("top_k", v)
		return nil
	}
}

func WithPresencePenalty(v float64) Opt {
	return func(o *Opts) error {
		if v < -2 || v > 2 {
			return ErrBadParameter.With("presence_penalty")
		}
		o.Set("presence_penalty", v)
		return nil
	}
}

func WithFrequencyPenalty(v float64) Opt {
	return func(o *Opts) error {
		if v < -2 || v > 2 {
			return ErrBadParameter.With("frequency_penalty")
		}
		o.Set("frequency_penalty", v)
		return nil
	}
}

// The maximum number of tokens to generate in the completion.
func WithMaxTokens(v uint64) Opt {
	return func(o *Opts) error {
		o.Set("max_tokens", v)
		return nil
	}
}

// Set system prompt
func WithSystemPrompt(v string) Opt {
	return func(o *Opts) error {
		o.system = v
		return nil
	}
}

// Set stop sequence
func WithStopSequence(v ...string) Opt {
	return func(o *Opts) error {
		o.Set("stop", v)
		return nil
	}
}

// Set random seed for deterministic behavior
func WithSeed(v uint64) Opt {
	return func(o *Opts) error {
		o.Set("seed", v)
		return nil
	}
}

// Set format
func WithFormat(v any) Opt {
	return func(o *Opts) error {
		o.Set("format", v)
		return nil
	}
}

// Set tool choices: can be auto, none, required, any or a list of tool names
func WithToolChoice(v ...string) Opt {
	return func(o *Opts) error {
		o.Set("tool_choice", v)
		return nil
	}
}

// Number of completions to return for each request
func WithNumCompletions(v uint64) Opt {
	return func(o *Opts) error {
		if v < 1 || v > 8 {
			return ErrBadParameter.With("num_completions must be between 1 and 8")
		}
		o.Set("num_completions", v)
		return nil
	}
}

// Inject a safety prompt before all conversations.
func WithSafePrompt() Opt {
	return func(o *Opts) error {
		o.Set("safe_prompt", true)
		return nil
	}
}
