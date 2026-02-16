---
name: summarizer
title: Text Summarizer
description: Summarizes input text into a concise paragraph
model: gemini-2.0-flash
provider: gemini
system_prompt: You are a professional text summarizer.
format:
  type: object
  properties:
    summary:
      type: string
      description: The summarized text
  required:
    - summary
input:
  type: object
  properties:
    text:
      type: string
      description: The text to summarize
  required:
    - text
---
Produce a concise summary that captures the key points.

Input text:
{{ .text }}
