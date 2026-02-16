---
name: caption
title: Generate a caption from an attachment
description: Analyzes an attached image or document and generates a descriptive caption.
model: gemini-flash-latest
provider: gemini
system_prompt: |
  You are an expert at describing visual and textual content.
  When given an attachment, produce a clear, accurate caption that
  captures the essential content. Be concise but descriptive.
format:
  type: object
  properties:
    caption:
      type: string
      description: A concise descriptive caption for the attachment
    tags:
      type: array
      items:
        type: string
      description: Relevant tags or keywords
  required:
    - caption
    - tags
  additionalProperties: false
---
Describe the attached content and generate a caption for it.
