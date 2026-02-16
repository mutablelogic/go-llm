---
name: extract_entities
title: Extract named entities from text
description: Identifies and extracts named entities (people, organizations, locations, dates, etc.) from unstructured text.
system_prompt: |
  You are a named entity recognition expert. Extract all named entities from the
  provided text. Classify each entity by type and include the exact text span.
  Do not invent entities that are not present in the text.
input:
  type: object
  properties:
    text:
      type: string
      description: The text to extract entities from
    entity_types:
      type: array
      items:
        type: string
        enum:
          - person
          - organization
          - location
          - date
          - money
          - product
          - event
      description: Types of entities to extract (all types if omitted)
  required:
    - text
  additionalProperties: false
format:
  type: object
  properties:
    entities:
      type: array
      items:
        type: object
        properties:
          text:
            type: string
            description: The entity text as it appears in the source
          type:
            type: string
            enum:
              - person
              - organization
              - location
              - date
              - money
              - product
              - event
            description: The entity type
          normalized:
            type: string
            description: Normalized form of the entity (e.g. full name, ISO date)
        required:
          - text
          - type
    entity_count:
      type: integer
      description: Total number of entities found
  required:
    - entities
    - entity_count
  additionalProperties: false
---

Extract named entities from the following text{{ if .entity_types }}, focusing on these types: {{ range $i, $t := .entity_types }}{{ if $i }}, {{ end }}{{ $t }}{{ end }}{{ end }}:

{{ .text }}
