package schema

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"

	// Packages
	uuid "github.com/google/uuid"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	pg "github.com/mutablelogic/go-pg"
	types "github.com/mutablelogic/go-server/pkg/types"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// Message represents a message in a conversation with an LLM.
// It uses a universal content block representation that can be marshaled
// to any provider's format.
type Message struct {
	Role    string         `json:"role" help:"Message role" enum:"user,assistant,system,thinking,tool" example:"assistant"`
	Content []ContentBlock `json:"content" help:"Structured content blocks that make up the message" example:"[{\"text\":\"Unit tests catch regressions early and make refactoring safer.\"}]"`
	Tokens  uint           `json:"tokens,omitempty" help:"Token count attributed to this message" example:"12"`
	Result  ResultType     `json:"result" help:"Message result status encoded as a string in JSON" enum:"stop,max_tokens,blocked,tool_call,error,other,max_iterations" example:"stop"`
	Meta    map[string]any `json:"meta,omitzero" help:"Optional provider-specific message metadata" optional:"" example:"{\"thinking_signature\":\"abc123\"}"`
}

// MessageInsert persists a message within a session conversation.
type MessageInsert struct {
	Session uuid.UUID `json:"session" help:"Session ID"`
	Message `embed:""`
}

// MessageListRequest represents a request to list stored messages.
type MessageListRequest struct {
	pg.OffsetLimit
	Role string `json:"role,omitempty" help:"Filter by exact message role" optional:""`
	Text string `json:"text,omitempty" help:"Case-insensitive text search over message content" optional:""`
}

// MessageList represents a paginated list of stored messages.
type MessageList struct {
	MessageListRequest
	Count uint       `json:"count" help:"Total number of matching messages" example:"2"`
	Body  []*Message `json:"body,omitzero" help:"Messages returned for the current page" example:"[{\"role\":\"assistant\",\"content\":[{\"text\":\"Daily news summary\"}],\"tokens\":4,\"result\":\"stop\"}]"`
}

// ContentBlock represents a single piece of content within a message.
// Exactly one of the fields should be non-nil/non-empty.
type ContentBlock struct {
	Text       *string     `json:"text,omitempty" help:"Plain text content emitted by the model" example:"Unit tests catch regressions early and make refactoring safer."`
	Thinking   *string     `json:"thinking,omitempty" help:"Reasoning or thinking text emitted by the model" example:"I should keep this answer brief and concrete."`
	Attachment *Attachment `json:"attachment,omitempty" help:"Attachment content such as an image, document, or audio asset" example:"{\"type\":\"image/png\",\"url\":\"https://example.com/image.png\"}"`
	ToolCall   *ToolCall   `json:"tool_call,omitempty" help:"Tool invocation requested by the model" example:"{\"id\":\"call_123\",\"name\":\"get_weather\",\"input\":{\"city\":\"London\"}}"`
	ToolResult *ToolResult `json:"tool_result,omitempty" help:"Tool execution result returned to the model" example:"{\"id\":\"call_123\",\"name\":\"get_weather\",\"content\":{\"temperature_c\":18},\"is_error\":false}"`
}

// Attachment represents binary or URI-referenced media (images, documents, etc.)
type Attachment struct {
	ContentType string   `json:"type" help:"Attachment MIME type, for example image/png or application/pdf" example:"image/png"`
	Data        []byte   `json:"data,omitempty" help:"Inline attachment payload encoded as a byte string" example:"iVBORw0KGgo="`
	URL         *url.URL `json:"url,omitempty" help:"Attachment URL reference, for example https, gs, or file" example:"https://example.com/image.png"`
}

func (a Attachment) MarshalJSON() ([]byte, error) {
	type attachmentJSON struct {
		ContentType string `json:"type"`
		Data        []byte `json:"data,omitempty"`
		URL         string `json:"url,omitempty"`
	}

	out := attachmentJSON{
		ContentType: a.ContentType,
		Data:        a.Data,
	}
	if a.URL != nil {
		out.URL = a.URL.String()
	}

	return json.Marshal(out)
}

func (a *Attachment) UnmarshalJSON(data []byte) error {
	type attachmentJSON struct {
		ContentType string          `json:"type"`
		Data        []byte          `json:"data,omitempty"`
		URL         json.RawMessage `json:"url,omitempty"`
	}

	var in attachmentJSON
	if err := json.Unmarshal(data, &in); err != nil {
		return err
	}

	a.ContentType = in.ContentType
	a.Data = in.Data

	parsed, err := unmarshalAttachmentURL(in.URL)
	if err != nil {
		return err
	}
	a.URL = parsed

	return nil
}

func unmarshalAttachmentURL(data json.RawMessage) (*url.URL, error) {
	if len(data) == 0 || string(data) == "null" {
		return nil, nil
	}

	var raw string
	if err := json.Unmarshal(data, &raw); err == nil {
		if raw == "" {
			return nil, nil
		}
		parsed, err := url.Parse(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid attachment url %q: %w", raw, err)
		}
		return parsed, nil
	}

	// Accept the legacy object encoding for backward compatibility.
	var parsed url.URL
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("invalid attachment url: %w", err)
	}
	if parsed.String() == "" {
		return nil, nil
	}
	return &parsed, nil
}

// IsText returns true if the attachment has a text/* MIME type (e.g. text/plain,
// text/html, text/csv). Handles MIME parameters like charset gracefully.
// Such attachments can be converted to text blocks when providers don't
// support them as media uploads.
func (a Attachment) IsText() bool {
	mediaType, _, err := mime.ParseMediaType(a.ContentType)
	if err != nil {
		return strings.HasPrefix(a.ContentType, "text/")
	}
	return strings.HasPrefix(mediaType, "text/")
}

// TextContent returns the attachment's data as a string, optionally prefixed
// with the filename and content type for context. Only meaningful when
// IsText() returns true.
func (a Attachment) TextContent() string {
	text := string(a.Data)
	var header string
	if a.URL != nil && a.URL.Path != "" {
		header += "File: " + a.URL.Path + "\n"
	}
	if a.ContentType != "" {
		header += "Content-Type: " + a.ContentType + "\n"
	}
	if header != "" {
		return header + "\n" + text
	}
	return text
}

// URI returns the URL of the attachment as a string, or an empty string if
// the attachment is inline data only. Satisfies llm.Resource.
func (a Attachment) URI() string {
	if a.URL != nil {
		return a.URL.String()
	}
	return ""
}

// Name returns the last path segment of the attachment URL, or an empty
// string for inline data. Satisfies llm.Resource.
func (a Attachment) Name() string {
	if a.URL != nil && a.URL.Path != "" {
		return path.Base(a.URL.Path)
	}
	return ""
}

// Description returns an empty string. Satisfies llm.Resource.
func (a Attachment) Description() string { return "" }

// Type returns the MIME type of the attachment. Satisfies llm.Resource.
func (a Attachment) Type() string { return a.ContentType }

// maxAttachmentBytes caps the amount of data read from a remote URL to
// prevent unbounded memory use when fetching large responses.
const maxAttachmentBytes = 32 * 1024 * 1024 // 32 MiB

// Read returns the attachment's raw bytes. If Data is non-empty it is returned
// directly. If URL is set and has a supported scheme (http, https, file) the
// content is fetched. Satisfies llm.Resource.
func (a Attachment) Read(ctx context.Context) ([]byte, error) {
	if len(a.Data) > 0 {
		return a.Data, nil
	}
	if a.URL == nil {
		return nil, nil
	}
	switch a.URL.Scheme {
	case "http", "https":
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.URL.String(), nil)
		if err != nil {
			return nil, err
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("attachment: HTTP %d fetching %s", resp.StatusCode, a.URL)
		}
		return io.ReadAll(io.LimitReader(resp.Body, maxAttachmentBytes))
	case "file":
		return os.ReadFile(a.URL.Path)
	default:
		return nil, fmt.Errorf("attachment: unsupported URL scheme %q", a.URL.Scheme)
	}
}

// ToolCall represents a tool invocation requested by the model
type ToolCall struct {
	ID    string          `json:"id,omitempty" help:"Provider-assigned tool call identifier" example:"call_123"`
	Name  string          `json:"name" help:"Tool name to invoke" example:"get_weather"`
	Input json.RawMessage `json:"input,omitempty" help:"JSON-encoded arguments passed to the tool" example:"{\"city\":\"London\"}"`
	Meta  map[string]any  `json:"meta,omitempty" help:"Provider-specific metadata associated with the tool call" example:"{\"provider\":\"demo\"}"`
}

// ToolResult represents the result of running a tool
type ToolResult struct {
	ID      string          `json:"id,omitempty" help:"Tool call identifier this result belongs to" example:"call_123"`
	Name    string          `json:"name,omitempty" help:"Tool name that produced this result" example:"get_weather"`
	Content json.RawMessage `json:"content,omitempty" help:"JSON-encoded tool output content" example:"{\"temperature_c\":18}"`
	IsError bool            `json:"is_error,omitempty" help:"Whether the tool result represents an error" example:"false"`
}

////////////////////////////////////////////////////////////////////////////////
// CONSTANTS

// Message role constants
const (
	RoleUser              = "user"
	RoleAssistant         = "assistant"
	RoleSystem            = "system"
	RoleThinking          = "thinking"
	RoleTool              = "tool"
	MessageListMax uint64 = 100
)

////////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// Create a new message with the given role and text content
func NewMessage(role string, text string, opts ...opt.Opt) (*Message, error) {
	o, err := opt.Apply(opts...)
	if err != nil {
		return nil, err
	}

	// Create content blocks
	blocks := []ContentBlock{
		{Text: types.Ptr(text)},
	}
	if v := o.Get(opt.ContentBlockKey); v != nil {
		if attachments, ok := v.([]ContentBlock); !ok {
			return nil, fmt.Errorf("invalid attachments option")
		} else {
			blocks = append(blocks, attachments...)
		}
	}

	// Return the message
	return types.Ptr(Message{
		Role:    role,
		Content: blocks,
	}), nil
}

// NewToolResult creates a content block containing a successful tool result
func NewToolResult(id, name string, v any) ContentBlock {
	data, err := json.Marshal(v)
	if err != nil {
		return NewToolError(id, name, err)
	}
	return ContentBlock{
		ToolResult: &ToolResult{
			ID:      id,
			Name:    name,
			Content: json.RawMessage(data),
		},
	}
}

// NewToolError creates a content block containing a tool error result
func NewToolError(id, name string, err error) ContentBlock {
	return ContentBlock{
		ToolResult: &ToolResult{
			ID:      id,
			Name:    name,
			Content: json.RawMessage(fmt.Sprintf("%q", err.Error())),
			IsError: true,
		},
	}
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Text returns the concatenated text content from all text blocks in the message
func (m Message) Text() string {
	var result []string
	for _, block := range m.Content {
		if block.Text != nil {
			result = append(result, *block.Text)
		}
	}
	return strings.Join(result, "\n")
}

// EstimateTokens returns a rough token count for the message content.
// It estimates ~4 characters per token for text, plus a fixed cost per
// non-text block (attachments, tool calls/results). This is useful for
// attributing per-message token costs without a provider-specific tokeniser.
func (m Message) EstimateTokens() uint {
	tokens := uint(0)
	for _, block := range m.Content {
		switch {
		case block.Text != nil:
			// ~4 characters per token, minimum 1
			n := uint(len(*block.Text)+3) / 4
			if n == 0 {
				n = 1
			}
			tokens += n
		case block.Thinking != nil:
			n := uint(len(*block.Thinking)+3) / 4
			if n == 0 {
				n = 1
			}
			tokens += n
		case block.ToolCall != nil:
			// Tool name + JSON arguments
			n := uint(len(block.ToolCall.Name)+len(block.ToolCall.Input)+3) / 4
			if n == 0 {
				n = 1
			}
			tokens += n
		case block.ToolResult != nil:
			n := uint(len(block.ToolResult.Content)+3) / 4
			if n == 0 {
				n = 1
			}
			tokens += n
		case block.Attachment != nil:
			// Rough estimate for binary data (images, etc.)
			n := max(uint(len(block.Attachment.Data)+3)/4, 10)
			tokens += n
		}
	}
	if tokens == 0 {
		tokens = 1
	}
	return tokens
}

// ToolCalls returns all tool call blocks in the message
func (m Message) ToolCalls() []ToolCall {
	var result []ToolCall
	for _, block := range m.Content {
		if block.ToolCall != nil {
			result = append(result, *block.ToolCall)
		}
	}
	return result
}

////////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (m Message) String() string {
	return types.Stringify(m)
}

func (r MessageListRequest) String() string {
	return types.Stringify(r)
}

func (r MessageList) String() string {
	return types.Stringify(r)
}

func (m *Message) Scan(row pg.Row) error {
	var result string
	if err := row.Scan(&m.Role, &m.Content, &m.Tokens, &result, &m.Meta); err != nil {
		return err
	}
	m.Result = parseMessageResult(result)
	if m.Meta == nil {
		m.Meta = make(map[string]any)
	}
	if m.Content == nil {
		m.Content = []ContentBlock{}
	}
	return nil
}

func (m *MessageInsert) Scan(row pg.Row) error {
	var result string

	if err := row.Scan(&m.Session, &m.Role, &m.Content, &m.Tokens, &result, &m.Meta); err != nil {
		return err
	}
	m.Result = parseMessageResult(result)
	if m.Meta == nil {
		m.Meta = make(map[string]any)
	}
	if m.Content == nil {
		m.Content = []ContentBlock{}
	}
	return nil
}

func (list *MessageList) Scan(row pg.Row) error {
	var message Message
	if err := message.Scan(row); err != nil {
		return err
	}
	list.Body = append(list.Body, &message)
	return nil
}

func (list *MessageList) ScanCount(row pg.Row) error {
	if err := row.Scan(&list.Count); err != nil {
		return err
	}
	return nil
}

func (req MessageListRequest) Query() url.Values {
	values := url.Values{}
	if req.Offset > 0 {
		values.Set("offset", strconv.FormatUint(req.Offset, 10))
	}
	if req.Limit != nil {
		values.Set("limit", strconv.FormatUint(types.Value(req.Limit), 10))
	}
	if role := strings.TrimSpace(req.Role); role != "" {
		values.Set("role", role)
	}
	if text := strings.TrimSpace(req.Text); text != "" {
		values.Set("text", text)
	}
	return values
}

func (req MessageListRequest) Select(bind *pg.Bind, op pg.Op) (string, error) {
	bind.Del("where")

	session, ok := bind.Get("session").(uuid.UUID)
	if !ok || session == uuid.Nil {
		return "", ErrBadParameter.With("message session is required")
	}
	bind.Append("where", `message.session = `+bind.Set("session", session))
	if role := strings.TrimSpace(req.Role); role != "" {
		bind.Append("where", `message.role = `+bind.Set("role", role))
	}
	if text := strings.TrimSpace(req.Text); text != "" {
		bind.Append("where", `message.content::text ILIKE `+bind.Set("text", "%"+text+"%"))
	}

	where := bind.Join("where", " AND ")
	bind.Set("orderby", `ORDER BY message.created_at ASC, message.id ASC`)
	req.OffsetLimit.Bind(bind, MessageListMax)

	switch op {
	case pg.List:
		if messageListHasUser(bind) {
			if where == "" {
				bind.Set("where", "")
			} else {
				bind.Set("where", "AND "+where)
			}
			return bind.Query("message.list_for_user"), nil
		}
		if where == "" {
			bind.Set("where", "")
		} else {
			bind.Set("where", "WHERE "+where)
		}
		return bind.Query("message.list"), nil
	default:
		return "", ErrNotImplemented.Withf("unsupported MessageListRequest operation %q", op)
	}
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS - WRITER

func (m MessageInsert) Insert(bind *pg.Bind) (string, error) {
	if m.Session == uuid.Nil {
		return "", ErrBadParameter.With("message session is required")
	}

	role := strings.TrimSpace(m.Role)
	if role == "" {
		return "", ErrBadParameter.With("message role is required")
	}

	content := m.Content
	if content == nil {
		content = []ContentBlock{}
	}

	bind.Set("session", m.Session)
	bind.Set("role", role)
	bind.Set("content", content)

	if m.Tokens == 0 {
		bind.Set("tokens", nil)
	} else {
		bind.Set("tokens", m.Tokens)
	}

	result := strings.TrimSpace(m.Result.String())
	if result == "" || result == "unknown" || (m.Result == ResultStop && role != RoleAssistant && role != RoleThinking && role != RoleTool) {
		bind.Set("result", nil)
	} else {
		bind.Set("result", result)
	}

	if len(m.Meta) == 0 {
		bind.Set("meta", nil)
	} else {
		bind.Set("meta", m.Meta)
	}

	return bind.Query("message.insert"), nil
}

func (m MessageInsert) Update(_ *pg.Bind) error {
	return fmt.Errorf("MessageInsert: update: not supported")
}

func messageListHasUser(bind *pg.Bind) bool {
	if user, ok := bind.Get("user").(uuid.UUID); ok {
		return user != uuid.Nil
	}
	return false
}

func parseMessageResult(result string) ResultType {
	switch strings.TrimSpace(result) {
	case "", ResultStop.String():
		return ResultStop
	case ResultMaxTokens.String():
		return ResultMaxTokens
	case ResultBlocked.String():
		return ResultBlocked
	case ResultToolCall.String():
		return ResultToolCall
	case ResultError.String():
		return ResultError
	case ResultOther.String():
		return ResultOther
	case ResultMaxIterations.String():
		return ResultMaxIterations
	default:
		return ResultOther
	}
}

////////////////////////////////////////////////////////////////////////////////
// MESSAGE OPTIONS

// WithAttachmentURL creates an attachment from data read from the provided reader
// The MIME type is detected from the data. This is suitable for small attachments
// the caller is responsible for closing the reader after the data is read.
func WithAttachment(r io.Reader) opt.Opt {
	data, err := io.ReadAll(r)
	if err != nil {
		return opt.Error(err)
	}
	return opt.AddAny(opt.ContentBlockKey, ContentBlock{
		Attachment: types.Ptr(Attachment{
			ContentType: http.DetectContentType(data),
			Data:        data,
		}),
	})
}

// WithAttachmentURL creates an attachment from a URL and explicit MIME type
func WithAttachmentURL(u string, mimetype string) opt.Opt {
	url, err := url.Parse(u)
	if err != nil {
		return opt.Error(fmt.Errorf("invalid URL: %w", err))
	}
	return opt.AddAny(opt.ContentBlockKey, ContentBlock{
		Attachment: types.Ptr(Attachment{
			ContentType: mimetype,
			URL:         url,
		}),
	})
}
