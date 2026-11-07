package gemini

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"mime"
	"strings"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/provider/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	// metaKey is the key of a tool call's meta which says Gemini made it,
	// whose value is its signature, and the identifier the API gave it,
	// when it gave one, so it's sent back as a function call, as it was
	metaKey = "gemini"

	// emptyReply is the text of a reply which had neither text nor calls,
	// as it's sent back, as a reply can't be empty
	emptyReply = "(No reply.)"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// callMeta is what's kept of a call Gemini made, in its meta: its thought
// signature, which Gemini 3 needs sent back with it, and the identifier the
// API gave it, which isn't the call's identifier, when the API gave none
type callMeta struct {
	Signature string `json:"signature,omitempty"`
	ID        string `json:"id,omitempty"`
}

// toolCall is a tool which a model called, as it was sent, for its result,
// and whether it's sent back as a call, with its result as a response, or,
// when it isn't, its result is text
type toolCall struct {
	name   string
	input  json.RawMessage
	gemini *callMeta
	replay bool
}

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// newReply returns the reply as a message, with its thinking, which is a
// summary of the model's thoughts, its text, and its calls of tools, as
// content blocks, in order, each call with its identifier, which is the
// API's, or one of its own, when it gave none, and its signature, in its
// meta, why the model stopped, and its output tokens
func newReply(parts []part, finish, blocked string, usage usageMetadata) *schema.Message {
	reply := &schema.Message{
		MessageMeta: schema.MessageMeta{Sender: "assistant", Content: []schema.ContentBlock{}},
		Tokens:      uint64(usage.CandidatesTokenCount + usage.ThoughtsTokenCount),
	}
	calls := false
	for _, p := range parts {
		switch {
		case p.FunctionCall != nil:
			args := p.FunctionCall.Args
			if len(args) == 0 || string(args) == "null" {
				args = json.RawMessage(`{}`)
			}
			meta := callMeta{Signature: p.ThoughtSignature, ID: p.FunctionCall.ID}
			id := p.FunctionCall.ID
			if id == "" {
				id = newCallID()
			}
			reply.Content = append(reply.Content, schema.ContentBlock{ToolCall: &schema.ToolCall{
				ID: id, Name: p.FunctionCall.Name, Input: args, Meta: map[string]any{metaKey: meta.value()},
			}})
			calls = true
		case p.Thought:
			if p.Text != "" {
				appendThinking(reply, p.Text)
			}
		case p.Text != "":
			appendText(reply, p.Text)
		}
	}
	reply.Result = result(finish, blocked, calls)
	return reply
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// result returns why the model stopped, from the API's reason, or why the
// prompt was blocked, which is that it called tools, when it stopped, and
// called them
func result(reason, blocked string, calls bool) schema.ResultType {
	if blocked != "" {
		return schema.ResultBlocked
	}
	switch reason {
	case "STOP":
		if calls {
			return schema.ResultToolCall
		}
		return schema.ResultStop
	case "MAX_TOKENS":
		return schema.ResultMaxTokens
	case "SAFETY", "RECITATION", "LANGUAGE", "BLOCKLIST", "PROHIBITED_CONTENT", "SPII",
		"IMAGE_SAFETY", "IMAGE_PROHIBITED_CONTENT", "IMAGE_RECITATION":
		return schema.ResultBlocked
	case "MALFORMED_FUNCTION_CALL", "UNEXPECTED_TOOL_CALL", "TOO_MANY_TOOL_CALLS", "MISSING_THOUGHT_SIGNATURE", "MALFORMED_RESPONSE":
		return schema.ResultError
	case "":
		if calls {
			return schema.ResultToolCall
		}
		return schema.ResultStop
	default:
		return schema.ResultOther
	}
}

// appendText adds text to the reply, to its last block, when it's text, as
// a reply's text is streamed in pieces
func appendText(reply *schema.Message, text string) {
	if n := len(reply.Content); n > 0 && reply.Content[n-1].Text != nil {
		reply.Content[n-1].Text = types.Ptr(*reply.Content[n-1].Text + text)
		return
	}
	reply.Content = append(reply.Content, schema.ContentBlock{Text: types.Ptr(text)})
}

// appendThinking adds thinking to the reply, to its last block, when it's
// thinking
func appendThinking(reply *schema.Message, text string) {
	if n := len(reply.Content); n > 0 && reply.Content[n-1].Thinking != nil {
		reply.Content[n-1].Thinking = types.Ptr(*reply.Content[n-1].Thinking + text)
		return
	}
	reply.Content = append(reply.Content, schema.ContentBlock{Thinking: types.Ptr(text)})
}

// newCallID returns an identifier of a call, for a call the API gave none,
// so its result refers to it
func newCallID() string {
	var data [8]byte
	_, _ = rand.Read(data[:])
	return "call_" + hex.EncodeToString(data[:])
}

// value returns the meta of a call, as it's kept in its meta
func (m callMeta) value() map[string]any {
	value := map[string]any{}
	if m.Signature != "" {
		value["signature"] = m.Signature
	}
	if m.ID != "" {
		value["id"] = m.ID
	}
	return value
}

// geminiCall returns the meta of a call Gemini made, or nil, for a call
// another model made, such as before the conversation's model changed
func geminiCall(meta map[string]any) *callMeta {
	value, ok := meta[metaKey]
	if !ok {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var result callMeta
	if err := json.Unmarshal(data, &result); err != nil {
		return nil
	}
	return &result
}

///////////////////////////////////////////////////////////////////////////////
// MESSAGES

// newContents returns the messages, as the API's contents, and the system
// prompt, which is the system messages, as the API has a system instruction,
// rather than system messages. Thinking is left out, as Gemini's is a
// summary, and its signature is with its calls, and another model's can't
// be sent. A message whose role is the same as the one before it is added
// to it, as the API's roles take turns, and the results of tools are the
// user's.
//
// As a conversation's model can change, calls of tools Gemini made are sent
// back as calls, with their signatures, and their results as responses, when
// the model accepts them, and calls which another model made, or which it
// doesn't accept, which Gemini 3 doesn't without their signatures, are left
// out of the replies, and their results are text, which says which tool was
// called, with what, and what it returned.
func newContents(ctx context.Context, model *Model, messages []llm.Message) ([]content, string, error) {
	calls := toolCalls(messages, needsSignatures(model))
	var system []string
	result := make([]content, 0, len(messages))
	for _, m := range messages {
		if m == nil {
			return nil, "", llm.ErrBadParameter.With("missing message")
		}
		var role string
		var parts []part
		switch m.Role() {
		case "system":
			if text := strings.TrimSpace(m.Text()); text != "" {
				system = append(system, text)
			}
			continue
		case "thinking":
			continue
		case "user", "tool":
			role = "user"
			var err error
			if parts, err = userParts(ctx, m, calls); err != nil {
				return nil, "", err
			}
		case "assistant":
			role = "model"
			parts = modelParts(m, calls)
		default:
			return nil, "", llm.ErrBadParameter.Withf("unsupported message role %q", m.Role())
		}
		if len(parts) == 0 {
			continue
		}
		if n := len(result); n > 0 && result[n-1].Role == role {
			result[n-1].Parts = append(result[n-1].Parts, parts...)
		} else {
			result = append(result, content{Role: role, Parts: parts})
		}
	}
	if len(result) == 0 {
		return nil, "", llm.ErrBadParameter.With("missing messages")
	}
	return result, strings.Join(system, "\n\n"), nil
}

// toolCalls returns the calls of tools in the messages, by their
// identifiers, with whether Gemini made them, and whether they're sent back
// as calls, which is decided for each reply, as its calls were made
// together: a reply's calls Gemini made are, unless the model needs their
// signatures, and none of them has one, as a reply of an older model which
// didn't think has none. Of calls made together, only the first is signed,
// so the others are sent back with it.
func toolCalls(messages []llm.Message, signed bool) map[string]toolCall {
	result := make(map[string]toolCall)
	for _, m := range messages {
		if m == nil || m.Role() != "assistant" {
			continue
		}
		var calls []*schema.ToolCall
		hasSignature := false
		for _, block := range contentOf(m) {
			if call := block.ToolCall; call != nil && call.ID != "" {
				calls = append(calls, call)
				if meta := geminiCall(call.Meta); meta != nil && meta.Signature != "" {
					hasSignature = true
				}
			}
		}
		replay := !signed || hasSignature
		for _, call := range calls {
			meta := geminiCall(call.Meta)
			result[call.ID] = toolCall{name: call.Name, input: call.Input, gemini: meta, replay: replay && meta != nil}
		}
	}
	return result
}

// needsSignatures returns whether the model needs the signatures of calls
// which are sent back, which Gemini 3, and later models, do, and Gemini 2.5
// doesn't
func needsSignatures(model *Model) bool {
	return model == nil || !strings.HasPrefix(model.Name(), "gemini-2")
}

// userParts returns the parts of a message from the user, or of the results
// of tools: the results first, as responses, to calls Gemini made, or as
// text, to calls another model made, then its attachments, which are read,
// and its text
func userParts(ctx context.Context, m llm.Message, calls map[string]toolCall) ([]part, error) {
	var parts []part
	for _, r := range m.ToolResults() {
		call, ok := calls[r.ID]
		if ok && call.replay {
			name := r.Name
			if name == "" {
				name = call.name
			}
			parts = append(parts, part{FunctionResponse: &functionResponse{ID: call.gemini.ID, Name: name, Response: responseObject(r.Content, r.IsError)}})
		} else {
			parts = append(parts, part{Text: resultText(r, call)})
		}
	}
	for _, resource := range m.Resources() {
		// An image in a format the API doesn't take, such as an SVG, is a
		// note, rather than the request failing
		if !imageFormat(resource.Type()) {
			parts = append(parts, part{Text: schema.Unreadable(resource.URI(), resource.Type())})
			continue
		}
		data, err := llm.ReadAll(ctx, resource)
		if err != nil {
			return nil, err
		}
		parts = append(parts, part{InlineData: &blob{MimeType: resource.Type(), Data: data}})
	}
	if text := m.Text(); strings.TrimSpace(text) != "" {
		parts = append(parts, part{Text: text})
	}
	return parts, nil
}

// modelParts returns the parts of a reply: its text, and the calls which
// are sent back, with their signatures, or text which says there's no
// reply, when there's neither, as a reply can't be empty. Its thinking, and
// calls which aren't sent back, such as those another model made, are left
// out.
func modelParts(m llm.Message, calls map[string]toolCall) []part {
	var parts []part
	replied := false
	for _, block := range contentOf(m) {
		switch {
		case block.Text != nil:
			if strings.TrimSpace(*block.Text) != "" {
				parts = append(parts, part{Text: *block.Text})
				replied = true
			}
		case block.ToolCall != nil:
			call, ok := calls[block.ToolCall.ID]
			if !ok || !call.replay {
				continue
			}
			meta := call.gemini
			args := block.ToolCall.Input
			if len(args) == 0 {
				args = json.RawMessage(`{}`)
			}
			parts = append(parts, part{
				FunctionCall:     &functionCall{ID: meta.ID, Name: block.ToolCall.Name, Args: args},
				ThoughtSignature: meta.Signature,
			})
			replied = true
		}
	}
	if !replied {
		if hasCalls(m) {
			// Calls another model made, whose results are text
			return nil
		}
		parts = append(parts, part{Text: emptyReply})
	}
	return parts
}

// hasCalls returns whether the message has calls of tools
func hasCalls(m llm.Message) bool {
	return len(m.ToolCalls()) > 0
}

// contentOf returns the content blocks of a message, as the schema has
// them, or its text, and the tools it called, for a message which isn't one
// of the schema's
func contentOf(m llm.Message) []schema.ContentBlock {
	switch v := m.(type) {
	case *schema.Message:
		if v != nil {
			return v.Content
		}
	case schema.Message:
		return v.Content
	case *schema.MessageMeta:
		if v != nil {
			return v.Content
		}
	case schema.MessageMeta:
		return v.Content
	}
	content := []schema.ContentBlock{{Text: types.Ptr(m.Text())}}
	for _, call := range m.ToolCalls() {
		content = append(content, schema.ContentBlock{ToolCall: &schema.ToolCall{ID: call.ID, Name: call.Name, Input: call.Input}})
	}
	return content
}

// responseObject returns the content of a tool's result as an object, as
// the API needs: an object as it is, anything else as its result, and an
// error as its error
func responseObject(content json.RawMessage, isError bool) json.RawMessage {
	if len(content) == 0 {
		content = json.RawMessage(`null`)
	}
	key := "result"
	if isError {
		key = "error"
	} else if trimmed := strings.TrimSpace(string(content)); strings.HasPrefix(trimmed, "{") {
		return content
	}
	data, err := json.Marshal(map[string]json.RawMessage{key: content})
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return data
}

// resultText returns the result of a call another model made, as text,
// with the tool, and its input, as the call isn't sent
func resultText(r llm.ToolResult, call toolCall) string {
	name := r.Name
	if name == "" {
		name = call.name
	}
	content := strings.TrimSpace(string(r.Content))
	var text string
	if json.Unmarshal(r.Content, &text) == nil {
		content = text
	}
	if content == "" {
		content = "nothing"
	}
	input := strings.TrimSpace(string(call.input))
	if input == "" {
		input = "{}"
	}
	if r.IsError {
		return "(The tool " + name + " was called with " + input + ", and failed: " + content + ")"
	}
	return "(The tool " + name + " was called with " + input + ", and returned: " + content + ")"
}

// imageFormats are the formats of images the API takes
var imageFormats = map[string]bool{"image/png": true, "image/jpeg": true, "image/webp": true, "image/heic": true, "image/heif": true}

// imageFormat returns whether the type isn't an image, or is one in a
// format the API takes
func imageFormat(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || !strings.HasPrefix(mediaType, "image/") {
		return err == nil
	}
	return imageFormats[mediaType]
}
