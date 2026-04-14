package prompt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - READ

// Prepare renders the prompt's Go template against the variables in input (a
// JSON object) and returns the resulting string along with any opts derived
// from the prompt's front matter (model, provider, system prompt, etc.).
func (p *prompt) Prepare(ctx context.Context, resources ...llm.Resource) (string, []opt.Opt, error) {

	// Read the first resource as JSON input, if provided
	var vars map[string]any
	if len(p.m.Input) > 0 {
		if len(resources) == 0 {
			return "", nil, schema.ErrBadParameter.Withf("input resource required for prompt with input schema")
		} else if resources[0] == nil || resources[0].Type() != types.ContentTypeJSON {
			return "", nil, schema.ErrBadParameter.Withf("input resource must be of type %q", types.ContentTypeJSON)
		} else if data, err := resources[0].Read(ctx); err != nil {
			return "", nil, err
		} else if vars_, err := validatePrepareInput(p.m.Input, data); err != nil {
			return "", nil, err
		} else {
			vars = vars_
		}
	}

	// Execute the prompt's template against the input data
	text, err := executeTemplate(p.m.Name, p.m.Template, vars)
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

	// Comvert data to map[string]any for templater
	if len(input) > 0 {
		if err := json.Unmarshal(input, &data); err != nil {
			return nil, schema.ErrBadParameter.Withf("input: invalid JSON: %v", err)
		}
	} else {
		data = make(map[string]any)
	}

	// Don't validate if no schema
	if len(inputSchema) == 0 {
		return data, nil
	}

	// Validate input against schema
	input_schema, err := jsonschema.FromJSON(json.RawMessage(inputSchema))
	if err != nil {
		return nil, schema.ErrBadParameter.Withf("input schema: %v", err)
	} else if err := input_schema.Validate(input); err != nil {
		return nil, schema.ErrBadParameter.Withf("input validation: %v", err)
	}

	// Return the template variables as a map
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
