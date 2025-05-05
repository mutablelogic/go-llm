package schema

import (
	"fmt"

	// Packages
	llm "github.com/mutablelogic/go-llm"
)

////////////////////////////////////////////////////////////////////////////
// TYPES

type Request struct {
	Method  string `json:"method"`
	ID      uint64 `json:"id"`
	Payload any    `json:"params,omitempty"`
}

type Response struct {
	Version string `json:"jsonrpc,omitempty"`
	ID      uint64 `json:"id"`
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

type ResponseListTools struct {
	Tools      []llm.Tool `json:"tools"`
	NextCursor string     `json:"nextCursor,omitempty"`
}

type ResponseListPrompts struct {
	Prompts    []llm.Tool `json:"prompts"` // TODO: Fix
	NextCursor string     `json:"nextCursor,omitempty"`
}

type ResponseListResources struct {
	Resources  []llm.Tool `json:"resources"` // TODO: Fix
	NextCursor string     `json:"nextCursor,omitempty"`
}

////////////////////////////////////////////////////////////////////////////
// GLOBALS

const (
	RPCVersion                 = "2.0"
	ProtocolVersion            = "2024-11-05"
	MessageTypeInitialize      = "initialize"
	MessageTypePing            = "ping"
	MessageTypeListTools       = "tools/list"
	MessageTypeCallTool        = "tools/call"
	MessageTypeListResources   = "resources/list"
	MessageTypeListPrompts     = "prompts/list"
	NotificationTypeInitialize = "notifications/initialized"
	ErrorCodeMethodNotFound    = -32601
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

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Data != nil {
		return fmt.Sprintf("%d: %s (%v)", e.Code, e.Message, e.Data)
	}
	return fmt.Sprintf("%d: %s", e.Code, e.Message)
}
