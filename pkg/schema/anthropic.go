package schema

import (
	"encoding/json"
	"fmt"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// AnthropicMessage wraps a universal Message for Anthropic-specific JSON marshaling
type AnthropicMessage struct {
	Message
}

// anthropicContentBlock represents Anthropic's JSON format for content blocks
type anthropicContentBlock struct {
	Type string `json:"type"`

	// Text block
	Text *string `json:"text,omitempty"`

	// Image/Document block - both use 'source' at top level
	Source json.RawMessage `json:"source,omitempty"`

	// Document-specific fields (in addition to source)
	Title     *string          `json:"title,omitempty"`
	Context   *string          `json:"context,omitempty"`
	Citations *CitationOptions `json:"citations,omitempty"`

	// Tool use block - Anthropic uses "id", "name", "input"
	ID    *string         `json:"id,omitempty"`
	Name  *string         `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`

	// Tool result block
	ToolUseID *string         `json:"tool_use_id,omitempty"`
	Content   json.RawMessage `json:"content,omitempty"`
	IsError   *bool           `json:"is_error,omitempty"`

	// Thinking block
	Thinking  *string `json:"thinking,omitempty"`
	Signature *string `json:"signature,omitempty"`

	// Redacted thinking block
	Data *string `json:"data,omitempty"`

	// Server-side tool use metrics
	CacheCreation            *CacheMetrics `json:"cache_creation,omitempty"`
	CacheReadInputTokens     *uint         `json:"cache_read_input_tokens,omitempty"`
	InputTokens              *uint         `json:"input_tokens,omitempty"`
	CacheCreationInputTokens *uint         `json:"cache_creation_input_tokens,omitempty"`

	// Cache control
	CacheControl *CacheControl `json:"cache_control,omitempty"`
}

///////////////////////////////////////////////////////////////////////////////
// MARSHALING

// MarshalJSON converts the universal Message to Anthropic's JSON format
func (am AnthropicMessage) MarshalJSON() ([]byte, error) {
	// Create Anthropic-specific structure
	anthContent := make([]anthropicContentBlock, 0, len(am.Content))

	for _, block := range am.Content {
		anthBlock := anthropicContentBlock{
			Type:                     block.Type,
			CacheControl:             block.CacheControl,
			CacheCreation:            block.CacheCreation,
			CacheReadInputTokens:     block.CacheReadInputTokens,
			InputTokens:              block.InputTokens,
			CacheCreationInputTokens: block.CacheCreationInputTokens,
		}

		switch block.Type {
		case "text":
			anthBlock.Text = block.Text

		case "image":
			if block.ImageSource != nil {
				sourceData, _ := json.Marshal(block.ImageSource)
				anthBlock.Source = sourceData
			}

		case "document":
			// Document uses source at top level (like images)
			if block.DocumentSource != nil {
				sourceData, _ := json.Marshal(block.DocumentSource)
				anthBlock.Source = sourceData
				anthBlock.Title = block.DocumentTitle
				anthBlock.Context = block.DocumentContext
				anthBlock.Citations = block.Citations
			}

		case "tool_use":
			// Map universal field names to Anthropic field names
			anthBlock.ID = block.ToolUse.ToolUseID
			anthBlock.Name = block.ToolUse.ToolName
			anthBlock.Input = block.ToolUse.ToolInput

		case "tool_result":
			anthBlock.ToolUseID = block.ToolResult.ToolResultID
			anthBlock.Content = block.ToolResult.ToolResultContent
			anthBlock.IsError = block.ToolResult.ToolError

		case "thinking":
			anthBlock.Thinking = block.Thinking
			anthBlock.Signature = block.ThinkingSignature

		case "redacted_thinking":
			anthBlock.Data = block.RedactedThinkingData

		case "server_tool_use":
			// Server-side tool use (web_search, web_fetch, code_execution)
			anthBlock.ID = block.ToolUse.ToolUseID
			anthBlock.Name = block.ToolUse.ToolName
			anthBlock.Input = block.ToolUse.ToolInput

		case "web_search_tool_result", "web_fetch_tool_result", "code_execution_tool_result":
			// Server-side tool results
			anthBlock.ToolUseID = block.ToolResult.ToolResultID
			anthBlock.Content = block.ToolResult.ToolResultContent
		}

		anthContent = append(anthContent, anthBlock)
	}

	// Marshal with Anthropic structure
	return json.Marshal(struct {
		Role    string                  `json:"role"`
		Content []anthropicContentBlock `json:"content"`
	}{
		Role:    am.Role,
		Content: anthContent,
	})
}

// UnmarshalJSON converts Anthropic's JSON format to the universal Message
func (am *AnthropicMessage) UnmarshalJSON(data []byte) error {
	// Unmarshal into Anthropic-specific structure
	var anthMsg struct {
		Role    string                  `json:"role"`
		Content []anthropicContentBlock `json:"content"`
	}

	if err := json.Unmarshal(data, &anthMsg); err != nil {
		return fmt.Errorf("invalid anthropic message format: %w", err)
	}

	// Convert to universal format
	universalContent := make([]ContentBlock, 0, len(anthMsg.Content))

	for _, anthBlock := range anthMsg.Content {
		block := ContentBlock{
			Type:                     anthBlock.Type,
			CacheControl:             anthBlock.CacheControl,
			CacheCreation:            anthBlock.CacheCreation,
			CacheReadInputTokens:     anthBlock.CacheReadInputTokens,
			InputTokens:              anthBlock.InputTokens,
			CacheCreationInputTokens: anthBlock.CacheCreationInputTokens,
		}

		switch anthBlock.Type {
		case "text":
			block.Text = anthBlock.Text

		case "image":
			if len(anthBlock.Source) > 0 {
				var imgSource ImageSource
				if err := json.Unmarshal(anthBlock.Source, &imgSource); err != nil {
					return fmt.Errorf("failed to unmarshal image source: %w", err)
				}
				block.ImageSource = &imgSource
			}

		case "document":
			// Document has source at top level (like images)
			if len(anthBlock.Source) > 0 {
				var docSource DocumentSource
				if err := json.Unmarshal(anthBlock.Source, &docSource); err != nil {
					return fmt.Errorf("failed to unmarshal document source: %w", err)
				}
				block.DocumentSource = &docSource
				block.DocumentTitle = anthBlock.Title
				block.DocumentContext = anthBlock.Context
				block.Citations = anthBlock.Citations
			}

		case "tool_use":
			// Map Anthropic field names to universal field names
			block.ToolUse.ToolUseID = anthBlock.ID
			block.ToolUse.ToolName = anthBlock.Name
			block.ToolUse.ToolInput = anthBlock.Input

		case "tool_result":
			block.ToolResult.ToolResultID = anthBlock.ToolUseID
			block.ToolResult.ToolResultContent = anthBlock.Content
			block.ToolResult.ToolError = anthBlock.IsError

		case "thinking":
			block.Thinking = anthBlock.Thinking
			block.ThinkingSignature = anthBlock.Signature

		case "redacted_thinking":
			block.RedactedThinkingData = anthBlock.Data

		case "server_tool_use":
			// Server-side tool use (web_search, web_fetch, code_execution)
			block.ToolUse.ToolUseID = anthBlock.ID
			block.ToolUse.ToolName = anthBlock.Name
			block.ToolUse.ToolInput = anthBlock.Input

		case "web_search_tool_result", "web_fetch_tool_result", "code_execution_tool_result":
			// Server-side tool results
			block.ToolResult.ToolResultID = anthBlock.ToolUseID
			block.ToolResult.ToolResultContent = anthBlock.Content
		}

		universalContent = append(universalContent, block)
	}

	am.Message = Message{
		Role:    anthMsg.Role,
		Content: universalContent,
	}

	return nil
}
