---
name: summarize
title: Summarize text into key points
description: Summarizes long-form text into a structured response with a short summary, key points, and sentiment analysis.
system_prompt: |
  You are an expert summarizer. Extract the most important information from the
  provided text. Be concise but thorough. Identify the overall sentiment and
  list distinct key points.
input:
  type: object
  properties:
    text:
      type: string
      description: The text to summarize
    max_points:
      type: integer
      description: Maximum number of key points to extract (default 5)
  required:
    - text
  additionalProperties: false
format:
  type: object
  properties:
    summary:
      type: string
      description: A concise one-paragraph summary
    key_points:
      type: array
      items:
        type: string
      description: List of key points extracted from the text
    sentiment:
      type: string
      enum:
        - positive
        - negative
        - neutral
        - mixed
      description: Overall sentiment of the text
    word_count:
      type: integer
      description: Word count of the original text
  required:
    - summary
    - key_points
    - sentiment
  additionalProperties: false
---

Summarize the following text{{ if .max_points }}, extracting at most {{ .max_points }} key points{{ end }}:

{{ .text }}
