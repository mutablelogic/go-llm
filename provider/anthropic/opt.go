package anthropic

import (
	"encoding/json"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// defaultMaxTokens is the most tokens of a reply, when neither the
	// option nor the model says, and maxTokens the most, unless the option
	// says otherwise, as replies are streamed, so a long one doesn't time
	// out
	defaultMaxTokens = 16000
	maxTokens        = 64000

	// thinkingBudget is the most tokens an older model, which thinks within
	// a budget, thinks with, which is less than the most tokens of its reply,
	// and minThinkingBudget the least, which is the least the API accepts
	thinkingBudget    = 8000
	minThinkingBudget = 1024

	// bindingBeta is the beta which lets thinking which was sent back be
	// dropped, rather than fail the request, when the conversation before it
	// changed, such as when it was compacted
	bindingBeta = "thinking-binding-controls-2026-08-01"
)

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// apply sets the options on the request, for the model, or returns an error
// if an option isn't valid, or the API doesn't support it, such as the
// sampling options, which the API's version doesn't accept, and which the
// registry leaves out, as the models don't take a temperature. A tool
// choice of "any" is "auto", as the newest models don't accept a forced
// tool choice.
func (r *messagesRequest) apply(model *Model, opts ...llm.Opt) error {
	o, err := llm.ApplyOpts(opts...)
	if err != nil {
		return err
	}
	for _, key := range o.Keys() {
		var err error
		switch key {
		case llm.ContextWindowKey:
			// Ignored
		case llm.MaxTokensKey:
			r.MaxTokens, err = llm.GetOpt[uint](o, key)
		case llm.StopSequencesKey:
			r.StopSequences, err = llm.GetOpt[[]string](o, key)
		case llm.SystemPromptKey:
			var prompt string
			if prompt, err = llm.GetOpt[string](o, key); err == nil && strings.TrimSpace(prompt) != "" {
				r.System = strings.TrimSpace(strings.Join([]string{prompt, r.System}, "\n\n"))
			}
		case llm.ThinkingKey:
			_, err = llm.GetOpt[bool](o, key)
		case llm.ThinkingEffortKey:
			var effort string
			if effort, err = llm.GetOpt[string](o, key); err == nil && model.caps.Effort.Supported {
				// The nearest the model has, such as "high" for "max", for a
				// model which hasn't it
				r.outputConfig().Effort = llm.NearestEffort(effort, model.caps.Effort.levels()...)
			}
		case llm.JSONSchemaKey:
			var schema json.RawMessage
			if schema, err = llm.GetOpt[json.RawMessage](o, key); err == nil {
				if schema, err = strictSchema(schema); err == nil {
					r.outputConfig().Format = &outputFormat{Type: "json_schema", Schema: schema}
				}
			}
		case llm.ToolChoiceKey:
			var choice string
			if choice, err = llm.GetOpt[string](o, key); err == nil {
				switch choice {
				case "none":
					r.ToolChoice = &toolChoice{Type: "none"}
				case "auto", "any":
					r.ToolChoice = &toolChoice{Type: "auto"}
				default:
					err = llm.ErrBadParameter.Withf("tool choice %q isn't supported", choice)
				}
			}
		case llm.UserKey:
			var user string
			if user, err = llm.GetOpt[string](o, key); err == nil && user != "" {
				r.Metadata = &metadata{UserID: user}
			}
		case llm.ServiceTierKey:
			r.ServiceTier, err = llm.GetOpt[string](o, key)
		default:
			err = llm.ErrBadParameter.Withf("option %q isn't supported by anthropic", key)
		}
		if err != nil {
			return err
		}
	}

	// The most tokens of the reply are the option's, or the model's, up to
	// maxTokens
	if r.MaxTokens == 0 {
		r.MaxTokens = defaultMaxTokens
		if limit := model.OutputTokenLimit(); limit != nil {
			r.MaxTokens = min(*limit, maxTokens)
		}
	}

	// A model thinks, as it decides, and its thinking is summarized, so it's
	// shown, unless thinking is off, when it isn't shown, and the model makes
	// the least effort, unless there's an effort, as the newest models can't
	// stop thinking. An older model only thinks when it's asked to, within a
	// budget, which is at least the least the API accepts, and less than
	// the most tokens of the reply.
	think, err := llm.GetOpt[bool](o, llm.ThinkingKey)
	asked := err == nil
	switch model.thinks() {
	case "adaptive":
		if asked && !think {
			r.Thinking = &thinking{Type: "adaptive", Display: "omitted"}
			if model.caps.Effort.Supported && (r.OutputConfig == nil || r.OutputConfig.Effort == "") {
				r.outputConfig().Effort = "low"
			}
		} else {
			r.Thinking = &thinking{Type: "adaptive", Display: "summarized"}
		}
	case "enabled":
		if asked && think {
			if r.MaxTokens <= minThinkingBudget {
				return llm.ErrBadParameter.Withf("%s needs more than %d tokens to think", llm.MaxTokensKey, minThinkingBudget)
			}
			r.Thinking = &thinking{Type: "enabled", BudgetTokens: max(min(thinkingBudget, r.MaxTokens/2), minThinkingBudget)}
		}
	default:
		if asked && think {
			return llm.ErrBadParameter.Withf("%s doesn't think", model.Name())
		}
	}

	// Thinking which was sent back, and is bound to the conversation it was
	// in, is dropped, rather than fails the request, when the conversation
	// before it changed, as it does when it's compacted
	if r.Thinking != nil {
		r.Thinking.BlockBinding = &blockBinding{PrefixMismatchBehavior: "drop_block"}
		r.withBeta(bindingBeta)
	}

	// A tool choice needs tools
	if len(r.Tools) == 0 {
		r.ToolChoice = nil
	}
	return nil
}

// outputConfig returns the request's output configuration, which is created
// when it has none
func (r *messagesRequest) outputConfig() *outputConfig {
	if r.OutputConfig == nil {
		r.OutputConfig = &outputConfig{}
	}
	return r.OutputConfig
}

// withBeta adds the beta to those the request needs
func (r *messagesRequest) withBeta(name string) {
	if r.beta == nil {
		r.beta = make(map[string]bool)
	}
	r.beta[name] = true
}

// betas returns the betas the request needs, as the value of the
// anthropic-beta header, or empty, when it needs none
func (r *messagesRequest) betas() string {
	names := make([]string, 0, len(r.beta))
	for name := range r.beta {
		names = append(names, name)
	}
	return strings.Join(names, ",")
}

// strictSchema returns the JSON schema, with each object's
// additionalProperties false, unless it says otherwise, as the API's
// structured outputs need. Returns ErrBadParameter when it isn't a JSON
// object.
func strictSchema(data json.RawMessage) (json.RawMessage, error) {
	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		return nil, llm.ErrBadParameter.Withf("invalid json_schema: %v", err)
	}
	strict(schema)
	return json.Marshal(schema)
}

// schemaMaps are the keywords of a JSON schema whose values are maps of
// schemas, by name, schemaLists those whose values are lists of schemas,
// and schemaValues those whose value is a schema, or, for items, a list of
// them. Other keywords, such as enum, const and default, have values which
// aren't schemas, which are left as they are.
var (
	schemaMaps   = []string{"properties", "patternProperties", "$defs", "definitions", "dependentSchemas"}
	schemaLists  = []string{"allOf", "anyOf", "oneOf", "prefixItems"}
	schemaValues = []string{"items", "additionalProperties", "not", "if", "then", "else", "contains", "propertyNames", "unevaluatedItems", "unevaluatedProperties"}
)

// strict sets additionalProperties false on the schema, when it's an object
// without it, and on the schemas within it, which are only those of the
// keywords whose values are schemas
func strict(value any) {
	switch v := value.(type) {
	case map[string]any:
		if v["type"] == "object" {
			if _, exists := v["additionalProperties"]; !exists {
				v["additionalProperties"] = false
			}
		}
		for _, key := range schemaMaps {
			if children, ok := v[key].(map[string]any); ok {
				for _, child := range children {
					strict(child)
				}
			}
		}
		for _, key := range schemaLists {
			if children, ok := v[key].([]any); ok {
				for _, child := range children {
					strict(child)
				}
			}
		}
		for _, key := range schemaValues {
			strict(v[key])
		}
	case []any:
		// A list of schemas, such as the items of an older schema
		for _, child := range v {
			strict(child)
		}
	}
}
