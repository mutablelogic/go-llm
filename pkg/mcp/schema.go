package mcp

import (
	"encoding/json"
	"fmt"
)

////////////////////////////////////////////////////////////////////////////
// TYPES

type Request struct {
	Version string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	ID      any             `json:"id,omitempty"` // string or number for non-notifications
	Payload json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	Version string `json:"jsonrpc"`
	ID      any    `json:"id,omitempty"` // string or number
	Result  any    `json:"result,omitempty"`
	Err     *Error `json:"error,omitempty"`
}

type Error struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type ResponseInitialize struct {
	Capabilities struct {
		Prompts   map[string]any `json:"prompts"`
		Tools     map[string]any `json:"tools"`
		Resources map[string]any `json:"resources"`
	} `json:"capabilities"`
	ServerInfo struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"serverInfo"`
	Version string `json:"protocolVersion"`
}

type RequestList struct {
	Cursor string `json:"cursor,omitempty"`
}

type RequestToolCall struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

// Tool represents an MCP tool definition with schema
type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"inputSchema"`
}

type ResponseListTools struct {
	Tools      []*Tool `json:"tools"`
	NextCursor string  `json:"nextCursor,omitempty"`
}

type ResponseListPrompts struct {
	Prompts    []any  `json:"prompts"`
	NextCursor string `json:"nextCursor,omitempty"`
}

type ResponseListResources struct {
	Resources  []any  `json:"resources"`
	NextCursor string `json:"nextCursor,omitempty"`
}

type ResponseToolCall struct {
	Content []*Content `json:"content"`
	Error   bool       `json:"isError,omitempty"`
}

// Content represents a single piece of content in a tool result
type Content struct {
	Type     string `json:"type"` // "text", "image", "audio", "resource_link", "resource"
	Text     string `json:"text,omitempty"`
	Data     string `json:"data,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
	URI      string `json:"uri,omitempty"`
	Name     string `json:"name,omitempty"`
	Resource any    `json:"resource,omitempty"`
}

////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	RPCVersion      = "2.0"
	ProtocolVersion = "2025-06-18"

	// Message types
	MessageTypeInitialize    = "initialize"
	MessageTypePing          = "ping"
	MessageTypeListTools     = "tools/list"
	MessageTypeCallTool      = "tools/call"
	MessageTypeListResources = "resources/list"
	MessageTypeListPrompts   = "prompts/list"

	// Notification types
	NotificationTypeInitialize = "notifications/initialized"

	// Error codes
	ErrorCodeMethodNotFound    = -32601
	ErrorCodeInvalidParameters = -32602
	ErrorInternalError         = -32603
)

////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

func NewError(code int, message string, data ...any) *Error {
	switch len(data) {
	case 0:
		return &Error{Code: code, Message: message}
	case 1:
		return &Error{Code: code, Message: message, Data: data[0]}
	default:
		return &Error{Code: code, Message: message, Data: data}
	}
}

////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (e Error) Error() string {
	if e.Data != nil {
		return fmt.Sprintf("%d: %s (%v)", e.Code, e.Message, e.Data)
	}
	return fmt.Sprintf("%d: %s", e.Code, e.Message)
}
