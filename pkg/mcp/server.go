// Implements an MCP server based on the following specification:
// https://modelcontextprotocol.io/specification/2025-03-26/basic/lifecycle
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	// Packages
	"github.com/mutablelogic/go-llm/pkg/opt"
	"github.com/mutablelogic/go-llm/pkg/tool"
)

///////////////////////////////////////////////////////////////////////
// TYPES

type Server struct {
	name    string
	version string

	// Private members
	mu          sync.RWMutex       // Handler map lock
	handlers    map[string]Handler // Method handlers
	toolkit     *tool.Toolkit      // Toolkit for the server
	initialised bool
}

type Handler func(context.Context, any, json.RawMessage) (any, error)

///////////////////////////////////////////////////////////////////////
// LIFECYCLE

// Create a new MCP server with the given name and version
func New(name, version string, opts ...opt.Opt) (*Server, error) {
	// Apply options
	o, err := opt.Apply(opts...)
	if err != nil {
		return nil, err
	}

	self := &Server{
		name:     name,
		version:  version,
		handlers: make(map[string]Handler, 10),
	}

	// Extract toolkit from options
	if v, ok := o.Get(optToolkit).(*tool.Toolkit); ok {
		self.toolkit = v
	}

	// Register default handlers
	self.HandlerFunc(MessageTypeInitialize, self.handleInitialize)
	self.HandlerFunc(MessageTypePing, self.handlePing)
	self.HandlerFunc(NotificationTypeInitialize, self.handleInitialized)
	self.HandlerFunc(MessageTypeListPrompts, self.handleListPrompts)
	self.HandlerFunc(MessageTypeListResources, self.handleListResources)
	self.HandlerFunc(MessageTypeListTools, self.handleListTools)
	self.HandlerFunc(MessageTypeCallTool, self.handleCallTool)

	// Return success
	return self, nil
}

// Implements an MCP server with standard input and output,
// and run in the foreground until the context is done.
func (server *Server) RunStdio(ctx context.Context, r io.Reader, w io.Writer) error {
	var wg sync.WaitGroup
	defer wg.Wait()

	// Create a new buffered reader and writer
	reader := bufio.NewReader(r)
	writer := bufio.NewWriter(w)

	// Writer channel will write until the context is done
	writerCh := make(chan []byte)
	defer close(writerCh)

	wg.Go(func() {
		for data := range writerCh {
			if _, err := writer.Write(data); err != nil {
				fmt.Fprintln(os.Stderr, "Error writing to output:", err)
				return
			}
			// Flush the writer to ensure data is sent immediately
			writer.Flush()
		}
	})

	// Continue receiving input until the context is done
	var request string
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if part, isPrefix, err := reader.ReadLine(); err != nil {
			if err == io.EOF {
				break
			}
			return err
		} else if isPrefix {
			request += string(part)
			continue
		} else {
			request += string(part)
		}
		if request = strings.TrimSpace(request); request == "" {
			continue
		}

		// Process a request in the background
		wg.Go(func() {
			response, err := server.processRequest(ctx, request)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				return
			} else if response != nil {
				// Write the response and a newline
				writerCh <- append(response, '\n')
			}
		})

		// Reset the request
		request = ""
	}

	// Return success
	return nil
}

// HandlerFunc registers (or removes) a handler for a method
func (server *Server) HandlerFunc(method string, fn Handler) {
	server.mu.Lock()
	defer server.mu.Unlock()
	if fn == nil {
		delete(server.handlers, method)
	} else {
		server.handlers[method] = fn
	}
}

///////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func (server *Server) processRequest(ctx context.Context, payload string) ([]byte, error) {
	// Decode the request
	var request Request
	if err := json.Unmarshal([]byte(payload), &request); err != nil {
		return nil, err
	}

	// Look up and call the handler
	response := Response{Version: RPCVersion, ID: request.ID}
	if result, err := server.call(ctx, &request); err != nil {
		var target Error
		if errors.As(err, &target) {
			response.Err = &target
		} else {
			response.Err = NewError(0, err.Error())
		}
	} else if result == nil {
		// Notification — no response
		return nil, nil
	} else {
		response.Result = result
	}

	// Return the response
	return json.Marshal(response)
}

func (server *Server) call(ctx context.Context, request *Request) (any, error) {
	server.mu.RLock()
	defer server.mu.RUnlock()

	fn, exists := server.handlers[request.Method]
	if !exists {
		return nil, NewError(ErrorCodeMethodNotFound, "method not found", request.Method)
	}

	return fn(ctx, request.ID, request.Payload)
}

///////////////////////////////////////////////////////////////////////
// HANDLERS

func (server *Server) handleInitialize(_ context.Context, _ any, _ json.RawMessage) (any, error) {
	response := new(ResponseInitialize)
	response.Version = ProtocolVersion
	response.ServerInfo.Name = server.name
	response.ServerInfo.Version = server.version
	response.Capabilities.Prompts = map[string]any{
		"listChanged": true,
	}
	response.Capabilities.Resources = map[string]any{
		"listChanged": true,
		"subscribe":   false,
	}
	response.Capabilities.Tools = map[string]any{
		"listChanged": true,
	}
	return response, nil
}

func (server *Server) handlePing(_ context.Context, _ any, _ json.RawMessage) (any, error) {
	return map[string]any{}, nil
}

func (server *Server) handleInitialized(_ context.Context, _ any, _ json.RawMessage) (any, error) {
	server.initialised = true
	return nil, nil
}

func (server *Server) handleListPrompts(_ context.Context, _ any, _ json.RawMessage) (any, error) {
	response := new(ResponseListPrompts)
	response.Prompts = []any{}
	return response, nil
}

func (server *Server) handleListResources(_ context.Context, _ any, _ json.RawMessage) (any, error) {
	response := new(ResponseListResources)
	response.Resources = []any{}
	return response, nil
}

func (server *Server) handleListTools(_ context.Context, _ any, _ json.RawMessage) (any, error) {
	response := new(ResponseListTools)
	if server.toolkit == nil {
		response.Tools = []*Tool{}
		return response, nil
	}
	for _, t := range server.toolkit.Tools() {
		jsonSchema, err := t.Schema()
		if err != nil {
			jsonSchema = nil
		}
		response.Tools = append(response.Tools, &Tool{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: jsonSchema,
		})
	}
	return response, nil
}

func (server *Server) handleCallTool(ctx context.Context, _ any, payload json.RawMessage) (any, error) {
	if server.toolkit == nil {
		return nil, NewError(ErrorCodeMethodNotFound, "no tools configured")
	}

	var req RequestToolCall
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, NewError(ErrorCodeInvalidParameters, err.Error())
	}

	// Marshal arguments to pass to the toolkit
	var input json.RawMessage
	if req.Arguments != nil {
		data, err := json.Marshal(req.Arguments)
		if err != nil {
			return nil, NewError(ErrorCodeInvalidParameters, err.Error())
		}
		input = data
	}

	// Run the tool
	result, err := server.toolkit.Run(ctx, req.Name, input)
	if err != nil {
		// Return the error as a tool error response (not a JSON-RPC error)
		return &ResponseToolCall{
			Content: []*Content{{Type: "text", Text: err.Error()}},
			Error:   true,
		}, nil
	}

	// Marshal the result to JSON text
	data, err := json.Marshal(result)
	if err != nil {
		return nil, NewError(ErrorInternalError, err.Error())
	}

	return &ResponseToolCall{
		Content: []*Content{{Type: "text", Text: string(data)}},
	}, nil
}
