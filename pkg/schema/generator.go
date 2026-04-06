package schema

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"

	// Packages
	types "github.com/mutablelogic/go-server/pkg/types"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// GeneratorMeta represents generator settings which are persisted on a session
// as URL-style values within the session meta object.
type GeneratorMeta struct {
	Provider       string     `json:"provider,omitempty" yaml:"provider" help:"Provider name" optional:"" example:"ollama"`
	Model          string     `json:"model,omitempty" yaml:"model" help:"Model name" optional:"" example:"llama3.2"`
	SystemPrompt   string     `json:"system_prompt,omitempty" yaml:"system_prompt" help:"System prompt" optional:"" example:"Be concise and answer in one sentence."`
	Format         JSONSchema `json:"format,omitempty" yaml:"output" help:"JSON schema for structured output" optional:"" example:"{\"type\":\"object\",\"properties\":{\"summary\":{\"type\":\"string\"}}}"`
	Thinking       *bool      `json:"thinking,omitempty" yaml:"thinking" help:"Enable thinking/reasoning" optional:"" example:"true"`
	ThinkingBudget uint       `json:"thinking_budget,omitempty" yaml:"thinking_budget" help:"Thinking token budget (required for Anthropic, optional for Google)" optional:"" example:"2048"`
}

////////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (g GeneratorMeta) String() string {
	return types.Stringify(g)
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Values encodes generator settings as URL values so they can be stored in a
// session meta JSON object.
func (g GeneratorMeta) Values() url.Values {
	values := make(url.Values)
	if provider := strings.TrimSpace(g.Provider); provider != "" {
		values.Set("provider", provider)
	}
	if model := strings.TrimSpace(g.Model); model != "" {
		values.Set("model", model)
	}
	if prompt := strings.TrimSpace(g.SystemPrompt); prompt != "" {
		values.Set("system_prompt", prompt)
	}
	if len(g.Format) > 0 {
		values.Set("format", string(g.Format))
	}
	if g.Thinking != nil {
		values.Set("thinking", strconv.FormatBool(types.Value(g.Thinking)))
	}
	if g.ThinkingBudget > 0 {
		values.Set("thinking_budget", strconv.FormatUint(uint64(g.ThinkingBudget), 10))
	}
	if len(values) == 0 {
		return nil
	}
	return values
}

// GeneratorMetaFromValues decodes generator settings from session meta values.
func GeneratorMetaFromValues(values url.Values) GeneratorMeta {
	var meta GeneratorMeta
	if len(values) == 0 {
		return meta
	}
	meta.Provider = strings.TrimSpace(values.Get("provider"))
	meta.Model = strings.TrimSpace(values.Get("model"))
	meta.SystemPrompt = strings.TrimSpace(values.Get("system_prompt"))
	if format := strings.TrimSpace(values.Get("format")); format != "" {
		meta.Format = JSONSchema(json.RawMessage(format))
	}
	if thinking := strings.TrimSpace(values.Get("thinking")); thinking != "" {
		if parsed, err := strconv.ParseBool(thinking); err == nil {
			meta.Thinking = types.Ptr(parsed)
		}
	}
	if budget := strings.TrimSpace(values.Get("thinking_budget")); budget != "" {
		if parsed, err := strconv.ParseUint(budget, 10, 64); err == nil {
			meta.ThinkingBudget = uint(parsed)
		}
	}
	return meta
}

// ApplyGeneratorMeta replaces generator-related keys in values with the
// encoded contents of meta while preserving unrelated keys.
func ApplyGeneratorMeta(values url.Values, meta GeneratorMeta) url.Values {
	clone := make(url.Values)
	for key, vals := range values {
		clone[key] = append([]string(nil), vals...)
	}
	for _, key := range []string{"provider", "model", "system_prompt", "format", "thinking", "thinking_budget"} {
		delete(clone, key)
	}
	for key, vals := range meta.Values() {
		clone[key] = append([]string(nil), vals...)
	}
	if len(clone) == 0 {
		return nil
	}
	return clone
}

// MergeGeneratorMeta fills blank fields in primary from fallback.
func MergeGeneratorMeta(primary, fallback GeneratorMeta) GeneratorMeta {
	merged := primary
	if merged.Provider == "" {
		merged.Provider = fallback.Provider
	}
	if merged.Model == "" {
		merged.Model = fallback.Model
	}
	if merged.SystemPrompt == "" {
		merged.SystemPrompt = fallback.SystemPrompt
	}
	if len(merged.Format) == 0 {
		merged.Format = fallback.Format
	}
	if merged.Thinking == nil {
		merged.Thinking = fallback.Thinking
	}
	if merged.ThinkingBudget == 0 {
		merged.ThinkingBudget = fallback.ThinkingBudget
	}
	return merged
}