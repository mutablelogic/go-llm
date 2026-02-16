---
name: classifier
title: Sentiment Classifier
description: Classifies text sentiment as positive, negative, or neutral
model: claude-sonnet
provider: anthropic
thinking: true
thinking_budget: 2048
system_prompt: You are a sentiment analysis expert.
format:
  type: object
  properties:
    sentiment:
      type: string
      enum:
        - positive
        - negative
        - neutral
    confidence:
      type: number
  required:
    - sentiment
    - confidence
---
Analyze the following text and classify its sentiment.

Respond with your classification and confidence score (0.0 to 1.0).

Text:
{{ .text }}
