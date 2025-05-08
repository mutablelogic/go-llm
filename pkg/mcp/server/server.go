// Implements an MCP server based on the following specification:
// https://modelcontextprotocol.io/specification/2025-03-26/basic/lifecycle
package server

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
	"github.com/mutablelogic/go-llm"
	"github.com/mutablelogic/go-llm/pkg/internal/impl"
	"github.com/mutablelogic/go-llm/pkg/mcp/schema"
	"github.com/mutablelogic/go-llm/pkg/tool"
)

///////////////////////////////////////////////////////////////////////
// TYPES

type Server struct {
	Initialised bool

	// Private members
	slock    sync.Mutex         // This is the overall server lock
	lock     sync.RWMutex       // Handler map lock
	handlers map[string]Handler // Method handlers
	toolkit  llm.ToolKit        // ToolKit for the server
}

type Handler func(context.Context, uint64, json.RawMessage) (any, error)

///////////////////////////////////////////////////////////////////////
// LIFECYCLE

// Create a new empty server
func New(name, version string, opt ...Opt) (*Server, error) {
	self := new(Server)
	self.handlers = make(map[string]Handler, 10)

	// Apply options
	if err := self.apply(opt...); err != nil {
		return nil, err
	}

	// Set up default handlers - initialize, ping
	self.HandlerFunc(schema.MessageTypeInitialize, func(ctx context.Context, _ uint64, _ json.RawMessage) (any, error) {
		response := new(schema.ResponseInitialize)
		response.Version = schema.ProtocolVersion
		response.ServerInfo.Name = name
		response.ServerInfo.Version = version
		response.Capabilities.Prompts = map[string]any{
			"listChanged": false,
		}
		response.Capabilities.Resources = map[string]any{
			"listChanged": false,
			"subscribe":   false,
		}
		response.Capabilities.Tools = map[string]any{
			"listChanged": false,
		}
		return response, nil
	})
	self.HandlerFunc(schema.MessageTypePing, func(ctx context.Context, _ uint64, _ json.RawMessage) (any, error) {
		return map[string]any{}, nil
	})
	self.HandlerFunc(schema.NotificationTypeInitialize, func(ctx context.Context, _ uint64, _ json.RawMessage) (any, error) {
		self.Initialised = true
		return nil, nil
	})

	// Set up default handlers - list resources, tools, prompts
	self.HandlerFunc(schema.MessageTypeListPrompts, func(ctx context.Context, _ uint64, _ json.RawMessage) (any, error) {
		response := new(schema.ResponseListPrompts)
		response.Prompts = []llm.Tool{}
		return response, nil
	})
	self.HandlerFunc(schema.MessageTypeListResources, func(ctx context.Context, _ uint64, _ json.RawMessage) (any, error) {
		response := new(schema.ResponseListResources)
		response.Resources = []llm.Tool{}
		return response, nil
	})
	self.HandlerFunc(schema.MessageTypeListTools, func(ctx context.Context, _ uint64, _ json.RawMessage) (any, error) {
		response := new(schema.ResponseListTools)
		response.Tools = self.toolkit.Tools("mcp")
		return response, nil
	})

	// Process a tool call request
	self.HandlerFunc(schema.MessageTypeCallTool, func(ctx context.Context, id uint64, payload json.RawMessage) (any, error) {
		var req schema.RequestToolCall
		if err := json.Unmarshal(payload, &req); err != nil {
			return nil, err
		}

		// Run the tool(s)
		results, err := self.toolkit.Run(ctx, tool.NewCall(fmt.Sprint(id), req.Name, nil))
		if err != nil {
			return nil, err
		} else if len(results) == 0 {
			return nil, schema.NewError(schema.ErrorInternalError, "no results returned")
		}

		// Append results to the response
		var resp schema.ResponseToolCall
		for _, result := range results {
			if content, err := impl.FromValue(result.Value()); err != nil {
				return nil, err
			} else {
				resp.Content = append(resp.Content, content)
			}
		}

		// Return the results
		return resp, nil
	})

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

	wg.Add(1)
	go func() {
		defer wg.Done()
		for data := range writerCh {
			if _, err := writer.Write(data); err != nil {
				fmt.Fprintln(os.Stderr, "Error writing to output:", err)
				return
			}
			// Flush the writer to ensure data is sent immediately
			writer.Flush()
		}
	}()

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
		wg.Add(1)
		go func(request string) {
			defer wg.Done()
			response, err := server.processRequest(ctx, request)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				return
			} else if response != nil {
				// Write the response and a newline
				writerCh <- append(response, '\n')
			}
		}(request)

		// Reset the request
		request = ""
	}

	// Return success
	return nil
}

// Set a new handler for a method
func (server *Server) HandlerFunc(method string, fn Handler) {
	server.lock.Lock()
	defer server.lock.Unlock()
	if fn == nil {
		delete(server.handlers, method)
	} else {
		server.handlers[method] = fn
	}
}

///////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func (server *Server) processRequest(ctx context.Context, payload string) ([]byte, error) {
	server.slock.Lock()
	defer server.slock.Unlock()

	// Decode the request
	var request schema.Request
	if err := json.Unmarshal([]byte(payload), &request); err != nil {
		return nil, err
	}

	// Encode the response
	response := schema.Response{Version: "2.0", ID: request.ID}
	if result, err := server.call(ctx, &request); err != nil {
		var target schema.Error
		if errors.As(err, &target) {
			// If the error is already a schema.Error, use it
			response.Err = &target
		} else {
			// Use a generic error
			response.Err = schema.NewError(0, err.Error())
		}
	} else if result == nil {
		// No result is returned
		return nil, nil
	} else {
		response.Result = result
	}

	data, _ := json.MarshalIndent(response, "", "  ")
	fmt.Fprintln(os.Stderr, "Response:", string(data))

	// Return the response
	return json.Marshal(response)
}

func (server *Server) call(ctx context.Context, request *schema.Request) (any, error) {
	server.lock.RLock()
	defer server.lock.RUnlock()

	// Get the handler for the method
	fn, exists := server.handlers[request.Method]
	if !exists {
		return nil, schema.NewError(schema.ErrorCodeMethodNotFound, "method not found", request.Method)
	}

	// Call the handler function and return the result
	return fn(ctx, request.ID, request.Payload)
}
