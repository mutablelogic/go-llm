package prompt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	// Packages
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - READ

// Prepare renders the prompt's Go template against the variables in input (a
// JSON object) and returns the resulting string along with any opts derived
// from the prompt's front matter (model, provider, system prompt, etc.).
func (p *prompt) Prepare(_ context.Context, input json.RawMessage) (string, []opt.Opt, error) {
	// Validate the input against the prompt's input schema, if any, and convert it to
	// a map for template execution
	data, err := validatePrepareInput(p.m.Input, input)
	if err != nil {
		return "", nil, err
	}

	// Execute the prompt's template against the input data
	text, err := executeTemplate(p.m.Name, p.m.Template, data)
	if err != nil {
		return "", nil, err
	}

	// Prepare options from the prompt's front matter
	opts, err := p.options()
	if err != nil {
		return "", nil, err
	}

	// Return success
	return text, opts, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func (p *prompt) options() ([]opt.Opt, error) {
	var opts []opt.Opt

	if p.m.Model != nil && *p.m.Model != "" {
		opts = append(opts, opt.SetString(opt.ModelKey, *p.m.Model))
	}
	if p.m.Provider != nil && *p.m.Provider != "" {
		opts = append(opts, opt.SetString(opt.ProviderKey, *p.m.Provider))
	}
	if p.m.SystemPrompt != nil && *p.m.SystemPrompt != "" {
		opts = append(opts, opt.SetString(opt.SystemPromptKey, *p.m.SystemPrompt))
	}
	if len(p.m.Format) > 0 {
		opts = append(opts, opt.SetAny(opt.JSONSchemaKey, p.m.Format))
	}
	if p.m.Thinking != nil {
		opts = append(opts, opt.SetBool(opt.ThinkingKey, types.Value(p.m.Thinking)))
	}
	if p.m.ThinkingBudget != nil && *p.m.ThinkingBudget > 0 {
		opts = append(opts, opt.SetUint(opt.ThinkingBudgetKey, *p.m.ThinkingBudget))
	}
	if len(p.m.Tools) > 0 {
		opts = append(opts, opt.AddString(opt.ToolKey, p.m.Tools...))
	}

	// Return options
	return opts, nil
}

func validatePrepareInput(inputSchema schema.JSONSchema, input json.RawMessage) (map[string]any, error) {
	var data map[string]any
	if len(input) > 0 {
		if err := json.Unmarshal(input, &data); err != nil {
			return nil, schema.ErrBadParameter.Withf("input: invalid JSON: %v", err)
		}
	}

	if len(inputSchema) == 0 {
		if data == nil {
			data = make(map[string]any)
		}
		return data, nil
	}

	s, err := jsonschema.FromJSON(json.RawMessage(inputSchema))
	if err != nil {
		return nil, schema.ErrBadParameter.Withf("input schema: %v", err)
	}

	validatedInput := input
	if len(validatedInput) == 0 {
		validatedInput = json.RawMessage(`{}`)
	}
	if err := s.Validate(validatedInput); err != nil {
		return nil, schema.ErrBadParameter.Withf("input validation: %v", err)
	}

	if data == nil {
		data = make(map[string]any)
	}
	return data, nil
}

func executeTemplate(name, tmplText string, data map[string]any) (string, error) {
	if tmplText == "" {
		return "", nil
	}

	tmpl, err := template.New(name).Funcs(templateFuncMap()).Parse(tmplText)
	if err != nil {
		return "", schema.ErrBadParameter.Withf("template: %v", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", schema.ErrBadParameter.Withf("template: %v", err)
	}

	return buf.String(), nil
}

func templateFuncMap() template.FuncMap {
	return template.FuncMap{
		"json": func(v any) (string, error) {
			data, err := json.Marshal(v)
			if err != nil {
				return "", err
			}
			return string(data), nil
		},
		"default": func(def, val any) any {
			if val == nil || val == "" {
				return def
			}
			return val
		},
		"join": func(list []any, sep string) string {
			var buf strings.Builder
			for i, v := range list {
				if i > 0 {
					buf.WriteString(sep)
				}
				buf.WriteString(fmt.Sprint(v))
			}
			return buf.String()
		},
		"upper": strings.ToUpper,
		"lower": strings.ToLower,
		"trim":  strings.TrimSpace,
	}
}
