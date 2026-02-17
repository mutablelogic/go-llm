# Agent Definition Files

Agent definitions are markdown files with YAML front matter. They define
reusable LLM agents with structured input and output schemas.

## File Format

Each agent file has two parts:

1. **YAML front matter** (between `---` delimiters) — agent metadata
2. **Markdown body** — a Go template for the user message

```markdown
---
name: my_agent
title: A short description (at least 10 characters)
description: A longer description of what the agent does.
model: claude-haiku-4-5-20251001
provider: anthropic
system_prompt: |
  You are a helpful assistant.
input:
  type: object
  properties:
    query:
      type: string
      description: The user's query
  required:
    - query
format:
  type: object
  properties:
    answer:
      type: string
  required:
    - answer
---

Answer the following question: {{ .query }}
```

## Front Matter Fields

### Required

| Field   | Description                                      |
|---------|--------------------------------------------------|
| `name`  | Unique identifier (must be a valid identifier). If omitted, derived from the filename. |
| `title` | Human-readable title (minimum 10 characters). If omitted, extracted from the first markdown heading. |

### Optional

| Field             | Description                                         |
|-------------------|-----------------------------------------------------|
| `description`     | Longer description of the agent's purpose            |
| `model`           | LLM model name (e.g. `claude-haiku-4-5-20251001`)   |
| `provider`        | Provider name (e.g. `anthropic`, `google`, `mistral`)|
| `system_prompt`   | System prompt sent to the model                      |
| `format`          | JSON Schema defining the structured output format    |
| `input`           | JSON Schema defining the expected input variables    |
| `tools`           | List of tool names the agent is allowed to use       |
| `thinking`        | Enable thinking/reasoning (`true` or `false`)        |
| `thinking_budget` | Token budget for thinking (used with Anthropic)      |

## Template Body

The markdown body after the front matter is a
[Go template](https://pkg.go.dev/text/template) that constructs the user
message sent to the model. Template variables come from the `input` schema.

Use `{{ .field_name }}` to interpolate input fields. Standard Go template
constructs are supported:

```
{{ if .source_language }}The source language is {{ .source_language }}.{{ end }}

{{ range $i, $item := .items }}{{ if $i }}, {{ end }}{{ $item }}{{ end }}
```

## Template Functions

In addition to the [built-in Go template functions](https://pkg.go.dev/text/template#hdr-Functions),
the following custom functions are available in agent templates:

| Function  | Signature                       | Description                                                    |
|-----------|---------------------------------|----------------------------------------------------------------|
| `json`    | `json <value>`                  | Marshals a value to its JSON string representation.            |
| `default` | `default <fallback> <value>`    | Returns `value` unless it is nil or empty, in which case returns `fallback`. |
| `join`    | `join <list> <separator>`       | Joins a list of values into a single string with the given separator. |
| `upper`   | `upper <string>`                | Converts a string to uppercase.                                |
| `lower`   | `lower <string>`                | Converts a string to lowercase.                                |
| `trim`    | `trim <string>`                 | Removes leading and trailing whitespace from a string.         |

### Examples

```
{{/* Render an object as JSON */}}
{{ .data | json }}

{{/* Use a default value when a field may be omitted */}}
{{ .language | default "English" }}

{{/* Join a list of items */}}
{{ .tags | join ", " }}

{{/* Case conversion */}}
{{ .name | upper }}
{{ .name | lower }}

{{/* Trim whitespace */}}
{{ .description | trim }}
```

## CLI Usage

```sh
# Create or update agents from files
llm create-agent etc/agent/*.md

# List all agents
llm agents

# Filter agents by name
llm agents --name translate

# Get a specific agent by name
llm agent translate

# Get a specific version of an agent
llm agent translate@2

# Delete an agent (all versions)
llm delete-agent translate

# Delete a specific version of an agent
llm delete-agent translate@1
```

## Examples

See the files in this directory for complete examples:

- [translate.md](translate.md) — Translate text between languages
- [summarize.md](summarize.md) — Summarize text into key points with sentiment
- [extract_entities.md](extract_entities.md) — Extract named entities from text
