// Implements an MCP server based on the following specification:
// https://modelcontextprotocol.io/specification/2025-03-26/basic/lifecycle
package server

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

///////////////////////////////////////////////////////////////////////
// TYPES

type Server struct {
	sync.Mutex
	Name        string
	Version     string
	Initialised bool
}

type Request struct {
	Method  string `json:"method"`
	ID      uint64 `json:"id"`
	Payload any    `json:"payload,omitempty"`
}

type Error struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type Response struct {
	Version string `json:"jsonrpc,omitempty"`
	ID      uint64 `json:"id"`
	Result  any    `json:"result,omitempty"`
	Err     *Error `json:"error,omitempty"`
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

///////////////////////////////////////////////////////////////////////
// LIFECYCLE

// Create a new empty server
func New(name, version string) *Server {
	self := new(Server)
	self.Name = name
	self.Version = version
	return self
}

// Implements an MCP server with standard input and output,
// and run in the foreground until the context is done.
func (server *Server) RunStdio(ctx context.Context, r io.Reader, w io.Writer) error {
	var wg sync.WaitGroup
	defer wg.Wait()

	// Create a new buffered reader and writer
	reader := bufio.NewReader(r)
	// writer := bufio.NewWriter(w)
	writer := w

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
			//writer.Flush()
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
			fmt.Fprintln(os.Stderr, "Request -> ", request)
			response, err := server.processRequest(ctx, request)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error:", err)
				return
			} else if response != nil {
				// Write the response and a newline
				writerCh <- append(response, '\n')
				fmt.Fprintln(os.Stderr, "<- Response:", string(response))
			}
		}(request)

		// Reset the request
		request = ""
	}

	// Return the instance
	return nil
}

///////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func (server *Server) processRequest(ctx context.Context, payload string) ([]byte, error) {
	// Lock the server to prevent concurrent access
	server.Lock()
	defer server.Unlock()

	// Decode the request
	var request Request
	if err := json.Unmarshal([]byte(payload), &request); err != nil {
		return nil, err
	}

	// Encode the response
	response := Response{Version: "2.0", ID: request.ID}
	if result, err := server.Handle(ctx, &request); err != nil {
		response.Err = &Error{
			Code:    1,
			Message: err.Error(),
		}
	} else if result == nil {
		// No result is returned
		return nil, nil
	} else {
		response.Result = result
	}

	// Return the response
	return json.Marshal(response)
}

func (server *Server) Handle(ctx context.Context, request *Request) (any, error) {
	switch request.Method {
	case "initialize":
		response := new(ResponseInitialize)
		response.ServerInfo.Name = server.Name
		response.ServerInfo.Version = server.Version
		response.Capabilities.Prompts = map[string]any{
			"listChanged": true,
		}
		response.Capabilities.Resources = map[string]any{
			"listChanged": true,
			"subscribe":   true,
		}
		response.Capabilities.Tools = map[string]any{
			"listChanged": true,
		}
		response.Version = "2024-11-05"
		return response, nil
	case "notifications/initialized":
		// No response is needed
		fmt.Fprintln(os.Stderr, "<- Server is initialised")
		server.Initialised = true
		return nil, nil
	case "ping":
		return map[string]any{}, nil
	default:
		return nil, fmt.Errorf("method %q not implemented", request.Method)
	}
}
