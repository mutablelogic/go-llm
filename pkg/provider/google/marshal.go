package google

import (
	"encoding/base64"
	"encoding/json"
	"maps"
	"net/url"

	// Packages
	"github.com/google/uuid"
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	"github.com/mutablelogic/go-llm/pkg/tool"
)

///////////////////////////////////////////////////////////////////////////////
// SESSION / MESSAGE → GEMINI WIRE FORMAT (OUTBOUND)

// geminiContentsFromSession converts a schema.Conversation into gemini wire Content
// slices. System messages are skipped (handled via SystemInstruction separately).
func geminiContentsFromSession(session *schema.Conversation) ([]*geminiContent, error) {
	if session == nil {
		return nil, nil
	}

	contents := make([]*geminiContent, 0, len(*session))
	for _, msg := range *session {
		if msg.Role == schema.RoleSystem {
			continue
		}
		// Skip empty assistant messages (no content blocks) — these can
		// occur when the model returns a tool call with no text.
		if msg.Role == schema.RoleAssistant && len(msg.Content) == 0 {
			continue
		}
		c, err := geminiContentFromMessage(msg)
		if err != nil {
			return nil, err
		}
		contents = append(contents, c)
	}
	return contents, nil
}

// geminiContentFromMessage converts a single schema.Message to gemini wire Content.
// Handles role mapping (assistant→model) and thinking block round-tripping.
func geminiContentFromMessage(msg *schema.Message) (*geminiContent, error) {
	parts := make([]*geminiPart, 0, len(msg.Content))

	// Extract thinking signature from message metadata (for round-trip)
	var thoughtSig string
	if msg.Meta != nil {
		if v, ok := msg.Meta["thought_signature"].(string); ok {
			thoughtSig = v
		}
	}

	for i := range msg.Content {
		block := &msg.Content[i]

		// Thinking content
		if block.Thinking != nil {
			part := &geminiPart{
				Text:    *block.Thinking,
				Thought: true,
			}
			if thoughtSig != "" {
				part.ThoughtSignature = thoughtSig
			}
			parts = append(parts, part)
			continue
		}

		// Text content
		if block.Text != nil {
			parts = append(parts, &geminiPart{Text: *block.Text})
			continue
		}

		// Attachment — convert text/* to a text part since Gemini
		// doesn't support text MIME types as inline data
		if block.Attachment != nil {
			if block.Attachment.IsText() && len(block.Attachment.Data) > 0 {
				parts = append(parts, &geminiPart{Text: block.Attachment.TextContent()})
			} else if p := geminiPartFromAttachment(block.Attachment); p != nil {
				parts = append(parts, p)
			}
			continue
		}

		// Tool call (function call from the model)
		if block.ToolCall != nil {
			args := make(map[string]any)
			if len(block.ToolCall.Input) > 0 {
				if err := json.Unmarshal(block.ToolCall.Input, &args); err != nil {
					return nil, llm.ErrInternalServerError.Withf("unmarshal tool call args: %v", err)
				}
			}
			parts = append(parts, geminiNewFunctionCallPart(block.ToolCall.Name, args))
			continue
		}

		// Tool result (function response from the user)
		if block.ToolResult != nil {
			if p := geminiPartFromToolResult(block.ToolResult); p != nil {
				parts = append(parts, p)
			}
			continue
		}
	}

	// Role mapping: "assistant" → "model" for Gemini
	role := msg.Role
	if role == schema.RoleAssistant {
		role = "model"
	}

	return &geminiContent{
		Parts: parts,
		Role:  role,
	}, nil
}

// geminiPartFromAttachment converts a schema.Attachment to a gemini wire Part.
func geminiPartFromAttachment(att *schema.Attachment) *geminiPart {
	if len(att.Data) > 0 {
		return geminiNewInlineDataPart(att.Type, base64.StdEncoding.EncodeToString(att.Data))
	}
	if att.URL != nil {
		return geminiNewFileDataPart(att.Type, att.URL.String())
	}
	return nil
}

// geminiPartFromToolResult converts a schema.ToolResult to a gemini wire FunctionResponse Part.
func geminiPartFromToolResult(tr *schema.ToolResult) *geminiPart {
	name := tr.Name
	if name == "" {
		name = tr.ID
	}
	if name == "" {
		return nil
	}

	response := make(map[string]any)
	if len(tr.Content) > 0 {
		var content any
		if err := json.Unmarshal(tr.Content, &content); err != nil {
			// If the content is not valid JSON, pass it as a raw string
			response["output"] = string(tr.Content)
		} else {
			response["output"] = content
		}
	}
	if tr.IsError {
		response["error"] = true
	}

	return geminiNewFunctionResponsePart(name, response)
}

///////////////////////////////////////////////////////////////////////////////
// TOOL CONVERSION

// geminiFunctionDeclsFromTools converts a slice of tools to
// gemini wire FunctionDeclaration values, using ParametersJsonSchema.
func geminiFunctionDeclsFromTools(tools []tool.Tool) []*geminiFunctionDeclaration {
	decls := make([]*geminiFunctionDeclaration, 0, len(tools))
	for _, t := range tools {
		decl := &geminiFunctionDeclaration{
			Name:        t.Name(),
			Description: t.Description(),
		}

		// Convert the jsonschema.Schema to map[string]any via JSON round-trip
		if s, err := t.Schema(); err == nil && s != nil {
			if data, err := json.Marshal(s); err == nil {
				var m map[string]any
				if err := json.Unmarshal(data, &m); err == nil {
					decl.ParametersJSONSchema = m
				}
			}
		}

		decls = append(decls, decl)
	}
	return decls
}

///////////////////////////////////////////////////////////////////////////////
// GEMINI WIRE FORMAT → MESSAGE (INBOUND)

// messageFromGeminiResponse converts a gemini wire GenerateContentResponse to
// a schema.Message. Returns an empty message if the response has no candidates.
func messageFromGeminiResponse(response *geminiGenerateResponse) (*schema.Message, error) {
	if response == nil || len(response.Candidates) == 0 {
		return &schema.Message{}, nil
	}

	candidate := response.Candidates[0]
	if candidate.Content == nil {
		return &schema.Message{}, nil
	}

	// Convert parts to content blocks, collecting provider-specific metadata
	content := make([]schema.ContentBlock, 0, len(candidate.Content.Parts))
	var meta map[string]any
	for _, part := range candidate.Content.Parts {
		block, partMeta := blockFromGeminiPart(part)
		content = append(content, block)
		if partMeta != nil {
			if meta == nil {
				meta = make(map[string]any)
			}
			maps.Copy(meta, partMeta)
		}
	}

	// Role mapping: "model" → "assistant"
	role := candidate.Content.Role
	if role == "model" {
		role = schema.RoleAssistant
	}

	// Determine result type, upgrading to ResultToolCall if we have function calls
	result := resultFromGeminiFinishReason(candidate.FinishReason)
	for _, block := range content {
		if block.ToolCall != nil {
			result = schema.ResultToolCall
			break
		}
	}

	return &schema.Message{
		Role:    role,
		Content: content,
		Result:  result,
		Meta:    meta,
	}, nil
}

// blockFromGeminiPart converts a gemini wire Part to a schema.ContentBlock.
// Returns the block and any provider-specific metadata for the message.
func blockFromGeminiPart(part *geminiPart) (schema.ContentBlock, map[string]any) {
	// Thinking — stored as text with provider metadata for round-trip
	if part.Thought {
		meta := map[string]any{"thought": true}
		if part.ThoughtSignature != "" {
			meta["thought_signature"] = part.ThoughtSignature
		}
		return schema.ContentBlock{
			Thinking: &part.Text,
		}, meta
	}

	// Text
	if part.Text != "" {
		return schema.ContentBlock{
			Text: &part.Text,
		}, nil
	}

	// Inline data → Attachment with raw bytes
	if part.InlineData != nil {
		data, _ := base64.StdEncoding.DecodeString(part.InlineData.Data)
		return schema.ContentBlock{
			Attachment: &schema.Attachment{
				Type: part.InlineData.MIMEType,
				Data: data,
			},
		}, nil
	}

	// File data → Attachment with URL
	if part.FileData != nil {
		u, _ := url.Parse(part.FileData.FileURI)
		return schema.ContentBlock{
			Attachment: &schema.Attachment{
				Type: part.FileData.MIMEType,
				URL:  u,
			},
		}, nil
	}

	// Function call → ToolCall
	if part.FunctionCall != nil {
		var input json.RawMessage
		if part.FunctionCall.Args != nil {
			if data, err := json.Marshal(part.FunctionCall.Args); err == nil {
				input = data
			}
		}
		return schema.ContentBlock{
			ToolCall: &schema.ToolCall{
				ID:    uuid.New().String(),
				Name:  part.FunctionCall.Name,
				Input: input,
			},
		}, nil
	}

	// Function response → ToolResult
	if part.FunctionResponse != nil {
		raw, err := json.Marshal(part.FunctionResponse.Response)
		if err != nil {
			raw = []byte("{}")
		}
		return schema.ContentBlock{
			ToolResult: &schema.ToolResult{
				ID:      uuid.New().String(),
				Name:    part.FunctionResponse.Name,
				Content: raw,
			},
		}, nil
	}

	// Unknown part type — return empty text block
	empty := ""
	return schema.ContentBlock{
		Text: &empty,
	}, nil
}

///////////////////////////////////////////////////////////////////////////////
// FINISH REASON → RESULT TYPE

// resultFromGeminiFinishReason maps a Gemini REST API finish reason string
// to a schema.ResultType. Callers should check for FunctionCall parts
// separately to upgrade to ResultToolCall.
func resultFromGeminiFinishReason(reason string) schema.ResultType {
	switch reason {
	case geminiFinishReasonStop:
		return schema.ResultStop
	case geminiFinishReasonMaxTokens:
		return schema.ResultMaxTokens
	case geminiFinishReasonSafety, geminiFinishReasonImageSafety:
		return schema.ResultBlocked
	case geminiFinishReasonRecitation, geminiFinishReasonImageRecitation:
		return schema.ResultBlocked
	case geminiFinishReasonMalformedFunctionCall, geminiFinishReasonUnexpectedToolCall:
		return schema.ResultError
	case geminiFinishReasonBlocklist, geminiFinishReasonProhibitedContent,
		geminiFinishReasonSPII, geminiFinishReasonImageProhibitedContent:
		return schema.ResultBlocked
	case geminiFinishReasonLanguage:
		return schema.ResultBlocked
	default:
		return schema.ResultOther
	}
}
