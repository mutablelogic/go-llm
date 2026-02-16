---
name: translate
title: Translate text between Languages
description: Translates text from one language to another, returning the translated text and detected source language.
system_prompt: |
  You are a professional translator. Translate the provided text accurately
  while preserving tone, style, and meaning. Always detect the source language
  if not explicitly provided.
input:
  type: object
  properties:
    text:
      type: string
      description: The text to translate
    target_language:
      type: string
      description: The target language (e.g. "English", "French", "Spanish")
    source_language:
      type: string
      description: The source language, if known (auto-detected if omitted)
  required:
    - text
    - target_language
  additionalProperties: false
format:
  type: object
  properties:
    translated_text:
      type: string
      description: The translated text
    source_language:
      type: string
      description: The detected or provided source language
    target_language:
      type: string
      description: The target language
    confidence:
      type: number
      description: Confidence score from 0.0 to 1.0
  required:
    - translated_text
    - source_language
    - target_language
  additionalProperties: false
---

Translate the following text to {{ .target_language }}:

{{ .text }}

{{ if .source_language }}The source language is {{ .source_language }}.{{ end }}
