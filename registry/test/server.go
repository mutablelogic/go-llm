package test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	// Packages
	registry "github.com/mutablelogic/go-llm/registry"
	httphandler "github.com/mutablelogic/go-llm/registry/httphandler"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
	openapi "github.com/mutablelogic/go-server/pkg/openapi/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// Server is a HTTP test server serving the llm registry API. Every request
// and response is recorded, and checked against the OpenAPI specification
// when the test ends.
type Server struct {
	*httptest.Server
	router *httprouter.Router

	mu        sync.Mutex
	exchanges []Exchange
}

// Exchange is a recorded request and response
type Exchange struct {
	Method string
	Path   string

	// Pattern is the route pattern which matched the request, such as
	// "/api/llm/provider", or empty if no route matched
	Pattern string

	RequestHeader  http.Header
	RequestBody    []byte
	Status         int
	ResponseHeader http.Header
	ResponseBody   []byte
}

// recorder captures the status code and body written by a handler
type recorder struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// Prefix is the path prefix for the API routes
const Prefix = "/api"

// errNotDocumented is returned when a path is not in the specification
var errNotDocumented = errors.New("path is not documented")

///////////////////////////////////////////////////////////////////////////////
// LIFECYCLE

// NewServer starts a HTTP test server serving the llm registry API for the
// registry. When the test ends the server is closed, and every recorded
// exchange is checked against the OpenAPI specification.
func NewServer(t *testing.T, r *registry.Registry) *Server {
	t.Helper()
	return NewServerFunc(t, func(router *httprouter.Router) error {
		return httphandler.RegisterHandlers(r, router)
	})
}

// NewServerFunc starts a HTTP test server serving the API registered by the
// function. When the test ends the server is closed, and every recorded
// exchange is checked against the OpenAPI specification.
func NewServerFunc(t *testing.T, register func(*httprouter.Router) error) *Server {
	t.Helper()

	// Create the router and register the handlers
	router, err := httprouter.NewRouter(context.Background(), http.NewServeMux(), Prefix, "*", "llm", "test")
	if err != nil {
		t.Fatal(err)
	}
	if err := register(router); err != nil {
		t.Fatal(err)
	}

	// Start the server, recording every exchange
	self := &Server{router: router}
	self.Server = httptest.NewServer(http.HandlerFunc(self.record))

	// Close the server and check the contract when the test ends
	t.Cleanup(func() {
		self.Close()
		for _, err := range self.Verify() {
			t.Error(err)
		}
	})

	// Return the server
	return self
}

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

// Spec returns the OpenAPI specification for the API
func (s *Server) Spec() *openapi.Spec {
	return s.router.Spec()
}

// Exchanges returns the recorded requests and responses
func (s *Server) Exchanges() []Exchange {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Exchange(nil), s.exchanges...)
}

// Reset discards the recorded requests and responses
func (s *Server) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.exchanges = nil
}

// Do sends a request to the API with a JSON body, and returns the status code
// and response body. The path is relative to the API prefix, and may include
// a query string. A body which is a string or []byte is sent as-is, nil sends
// no body, and any other value is encoded as JSON.
func (s *Server) Do(t *testing.T, method, path string, body any) (int, []byte) {
	t.Helper()
	return s.DoWithContentType(t, method, path, types.ContentTypeJSON, body)
}

// DoWithContentType sends a request with the body as the content type, which
// is used as-is for a string or []byte body, and otherwise encoded as JSON.
// Returns the status code and the response body.
func (s *Server) DoWithContentType(t *testing.T, method, path, contentType string, body any) (int, []byte) {
	t.Helper()

	// Encode the body
	var data []byte
	switch body := body.(type) {
	case nil:
	case string:
		data = []byte(body)
	case []byte:
		data = body
	default:
		var err error
		if data, err = json.Marshal(body); err != nil {
			t.Fatal(err)
		}
	}

	// Create the request
	path, query, _ := strings.Cut(path, "?")
	url := s.URL + types.JoinPath(Prefix, path)
	if strings.HasSuffix(path, "/") && !strings.HasSuffix(url, "/") {
		// Keep a trailing slash, which is significant to the router
		url += "/"
	}
	if query != "" {
		url += "?" + query
	}
	req, err := http.NewRequest(method, url, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Accept", types.ContentTypeJSON)
	if data != nil {
		req.Header.Set(types.ContentTypeHeader, contentType)
	}

	// Send the request, and read the response
	resp, err := s.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	response, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	// Return the status code and response body
	return resp.StatusCode, response
}

// JSON sends a request to the API as Do does, and decodes a successful
// response body into v. Returns the status code.
func (s *Server) JSON(t *testing.T, method, path string, body, v any) int {
	t.Helper()
	status, response := s.Do(t, method, path, body)
	if status >= 200 && status < 300 && v != nil {
		if err := json.Unmarshal(response, v); err != nil {
			t.Fatalf("%s %s: decode response: %v: %s", method, path, err, response)
		}
	}
	return status
}

// Verify checks every recorded exchange against the OpenAPI specification,
// and returns an error for each which does not conform
func (s *Server) Verify() []error {
	var result []error
	for _, exchange := range s.Exchanges() {
		if err := verify(s.Spec(), exchange); err != nil {
			result = append(result, fmt.Errorf("%s: %w", exchange, err))
		}
	}
	return result
}

///////////////////////////////////////////////////////////////////////////////
// STRINGIFY

func (e Exchange) String() string {
	// Compact a JSON body onto one line
	body := bytes.TrimSpace(e.ResponseBody)
	var compact bytes.Buffer
	if err := json.Compact(&compact, body); err == nil {
		body = compact.Bytes()
	}
	return fmt.Sprintf("%s %s -> %d %s", e.Method, e.Path, e.Status, body)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// record serves the request with the router, recording the request and
// response
func (s *Server) record(w http.ResponseWriter, r *http.Request) {
	// Read the request body, and replace it so the handler can read it
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	// Serve the request, capturing the response
	rec := &recorder{ResponseWriter: w, status: http.StatusOK}
	s.router.ServeHTTP(rec, r)

	// Record the exchange
	s.mu.Lock()
	defer s.mu.Unlock()
	s.exchanges = append(s.exchanges, Exchange{
		Method:         r.Method,
		Path:           r.URL.Path,
		Pattern:        r.Pattern,
		RequestHeader:  r.Header.Clone(),
		RequestBody:    body,
		Status:         rec.status,
		ResponseHeader: rec.Header().Clone(),
		ResponseBody:   rec.body.Bytes(),
	})
}

// verify checks an exchange against the specification
func verify(spec *openapi.Spec, exchange Exchange) error {
	// Find the operation. A redirect from a path which is not documented is
	// allowed, as the request it redirects to is checked.
	op, err := operation(spec, exchange.Method, exchange.Pattern)
	if err != nil {
		if isRedirect(exchange.Status) && errors.Is(err, errNotDocumented) {
			return nil
		}
		return err
	}

	// Find the response for the status code
	response, ok := op.Responses[strconv.Itoa(exchange.Status)]
	if !ok {
		if response, ok = op.Responses["default"]; !ok {
			return fmt.Errorf("status %d is not documented", exchange.Status)
		}
	}

	// Check the request body for successful requests. Requests which fail
	// may do so because the body is invalid.
	if exchange.Status >= 200 && exchange.Status < 300 && op.RequestBody != nil {
		if err := validate(op.RequestBody.Content, exchange.requestHeader(), exchange.RequestBody); err != nil {
			return fmt.Errorf("request body: %w", err)
		}
	}

	// Check the response body
	if len(response.Content) == 0 {
		if len(bytes.TrimSpace(exchange.ResponseBody)) > 0 {
			return errors.New("response body is not documented")
		}
		return nil
	} else if err := validate(response.Content, exchange.ResponseHeader, exchange.ResponseBody); err != nil {
		return fmt.Errorf("response body: %w", err)
	}

	// Return success
	return nil
}

// operation returns the operation in the specification for the method and
// the route pattern which matched the request. The pattern is converted to an
// OpenAPI path template in the same way as the specification's paths, so a
// wildcard such as {key...} matches the rest of the path.
func operation(spec *openapi.Spec, method, pattern string) (*openapi.Operation, error) {
	if spec.Paths == nil {
		return nil, errors.New("specification has no paths")
	} else if pattern == "" {
		return nil, errNotDocumented
	}
	template := openapi.PathTemplate(pattern)
	item, exists := spec.Paths.MapOfPathItemValues[template]
	if !exists {
		return nil, errNotDocumented
	}
	var op *openapi.Operation
	switch method {
	case http.MethodGet:
		op = item.Get
	case http.MethodPut:
		op = item.Put
	case http.MethodPost:
		op = item.Post
	case http.MethodDelete:
		op = item.Delete
	case http.MethodPatch:
		op = item.Patch
	case http.MethodHead:
		op = item.Head
	case http.MethodOptions:
		op = item.Options
	}
	if op == nil {
		return nil, fmt.Errorf("method is not documented for %s", template)
	}
	return op, nil
}

// requestHeader returns the request headers, with the content type JSON if
// the request didn't set one
func (e Exchange) requestHeader() http.Header {
	if e.RequestHeader.Get(types.ContentTypeHeader) != "" {
		return e.RequestHeader
	}
	return http.Header{types.ContentTypeHeader: {types.ContentTypeJSON}}
}

// validate checks a body against the schema for its content type
func validate(content map[string]openapi.MediaType, header http.Header, body []byte) error {
	contentType, _, err := mime.ParseMediaType(header.Get(types.ContentTypeHeader))
	if err != nil {
		return fmt.Errorf("content type: %w", err)
	}
	media, ok := mediaType(content, contentType)
	if !ok {
		return fmt.Errorf("content type %q is not documented", contentType)
	}
	if media.Schema == nil {
		return nil
	}

	// A text/plain body, or a stream of server-sent events, is a string,
	// which is validated as a JSON string, and a multipart form is an object
	switch contentType {
	case types.ContentTypeTextPlain, types.ContentTypeTextStream:
		data, err := json.Marshal(string(body))
		if err != nil {
			return err
		}
		body = data
	case types.ContentTypeFormData:
		data, err := formObject(header, body)
		if err != nil {
			return fmt.Errorf("form: %w", err)
		}
		body = data
	}
	return media.Schema.Validate(body)
}

// mediaType returns the documented media type for the content type, which
// is the content type, or a wildcard, such as image/* or */*
func mediaType(content map[string]openapi.MediaType, contentType string) (openapi.MediaType, bool) {
	if media, ok := content[contentType]; ok {
		return media, true
	}
	if typ, _, ok := strings.Cut(contentType, "/"); ok {
		if media, ok := content[typ+"/*"]; ok {
			return media, true
		}
	}
	media, ok := content["*/*"]
	return media, ok
}

// formObject returns a multipart form as a JSON object, with each field as a
// string, and each file as its name, a binary-format string, so the form is
// validated against its schema. A field with more than one value, or more
// than one file, is an array.
func formObject(header http.Header, body []byte) ([]byte, error) {
	_, params, err := mime.ParseMediaType(header.Get(types.ContentTypeHeader))
	if err != nil {
		return nil, err
	}
	form, err := multipart.NewReader(bytes.NewReader(body), params["boundary"]).ReadForm(int64(len(body)) + 1)
	if err != nil {
		return nil, err
	}
	defer form.RemoveAll()
	object := make(map[string]any, len(form.Value)+len(form.File))
	for key, values := range form.Value {
		object[key] = values
	}
	for key, files := range form.File {
		names := make([]string, 0, len(files))
		for _, file := range files {
			names = append(names, file.Filename)
		}
		object[key] = names
	}
	for key, value := range object {
		if values := value.([]string); len(values) == 1 {
			object[key] = values[0]
		}
	}
	return json.Marshal(object)
}

///////////////////////////////////////////////////////////////////////////////
// RECORDER

func (r *recorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *recorder) Write(data []byte) (int, error) {
	r.body.Write(data)
	return r.ResponseWriter.Write(data)
}

// Unwrap returns the response writer, so a response controller can flush a
// stream, and set its deadlines
func (r *recorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

// isRedirect returns true for a redirect status code
func isRedirect(status int) bool {
	return status >= 300 && status < 400
}
