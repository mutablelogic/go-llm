package httphandler

import (
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	deadline "github.com/mutablelogic/go-llm/internal/deadline"
	taggroup "github.com/mutablelogic/go-llm/internal/taggroup"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	schema "github.com/mutablelogic/go-llm/toolkit/schema"
	httprequest "github.com/mutablelogic/go-server/pkg/httprequest"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	httprouter "github.com/mutablelogic/go-server/pkg/httprouter"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	openapi "github.com/mutablelogic/go-server/pkg/openapi"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// toolName is the path parameter naming a tool
type toolName struct {
	Name string `json:"name" help:"Canonical name of the tool, such as weatherapi:current, or its name without a namespace"`
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func toolHandlers(tools *toolkit.Toolkit, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	router.Spec().AddTag(taggroup.ToolsAndAgents, documentation.Section(2, "Tool Operations").Body)
	taggroup.Add(router.Spec(), taggroup.LLM, taggroup.ToolsAndAgents)
	return errors.Join(
		namespaceListHandlers(tools, router, documentation),
		toolListHandlers(tools, router, documentation),
		toolGetCallHandlers(tools, router, documentation),
	)
}

func namespaceListHandlers(tools *toolkit.Toolkit, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return router.Register("llm/namespace", nil, func(path httprequest.PathItem) {
		path.Tag(taggroup.ToolsAndAgents)

		// GET
		path.Get(func(w http.ResponseWriter, r *http.Request) {
			if list, err := tools.ListNamespaces(r.Context()); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err))
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), list)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("List Namespaces")
			op.Description(documentation.Section(3, "GET /llm/namespace").Body)
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.NamespaceList](), "List of namespaces")
		})
	})
}

func toolListHandlers(tools *toolkit.Toolkit, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return router.Register("llm/tool", nil, func(path httprequest.PathItem) {
		path.Tag(taggroup.ToolsAndAgents)

		// GET
		path.Get(func(w http.ResponseWriter, r *http.Request) {
			var req schema.ToolListRequest
			if err := httprequest.Query(r.URL.Query(), &req); err != nil {
				httpresponse.Error(w, httpresponse.ErrBadRequest.With(err.Error()))
			} else if list, err := tools.ListTools(r.Context(), req); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), req.String())
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), list)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("List Tools")
			op.Description(documentation.Section(3, "GET /llm/tool").Body)
			op.Query(jsonschema.MustFor[schema.ToolListRequest]())
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.ToolList](), "List of tools")
			op.ErrorResponse(http.StatusBadRequest, "Invalid request")
		})
	})
}

func toolGetCallHandlers(tools *toolkit.Toolkit, router *httprouter.Router, documentation *openapi.MarkdownDoc) error {
	return router.Register("llm/tool/{name}", jsonschema.MustFor[toolName](), func(path httprequest.PathItem) {
		path.Tag(taggroup.ToolsAndAgents)

		// GET
		path.Get(func(w http.ResponseWriter, r *http.Request) {
			name := r.PathValue("name")
			if tool, err := tools.GetTool(r.Context(), name); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), name)
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), tool)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Get Tool")
			op.Description(documentation.Section(3, "GET /llm/tool/{name}").Body)
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[schema.Tool](), "Tool")
			op.ErrorResponse(http.StatusNotFound, "Tool not found")
			op.ErrorResponse(http.StatusConflict, "The name has no namespace, and more than one namespace has a tool with the name")
		})

		// POST
		path.Post(func(w http.ResponseWriter, r *http.Request) {
			var req schema.CallToolRequest
			name := r.PathValue("name")
			if err := httprequest.Read(r, &req); err != nil {
				httpresponse.Error(w, httpresponse.ErrBadRequest.Withf("%w", err))
			} else if accept, err := types.AcceptContentType(r); err != nil {
				httpresponse.Error(w, httpresponse.ErrBadRequest.With(err.Error()))
			} else if accept == types.ContentTypeTextStream {
				callToolStream(w, r, tools, name, req.Input)
			} else if result, err := tools.CallTool(r.Context(), name, req.Input); err != nil {
				httpresponse.Error(w, llm.HTTPErr(err), name)
			} else if result == nil {
				httpresponse.Empty(w, http.StatusNoContent)
			} else {
				httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), result)
			}
		}, func(op httprequest.PathOperation) {
			op.Summary("Call Tool")
			op.Description(documentation.Section(3, "POST /llm/tool/{name}").Body)
			op.RequestBody(jsonschema.MustFor[schema.CallToolRequest]())
			op.JSONResponse(http.StatusOK, jsonschema.MustFor[json.RawMessage](), "Result of the tool as JSON, or a stream of server-sent events when text/event-stream is accepted")
			op.TextStreamResponse(http.StatusOK)
			op.EmptyResponse("The tool has no result")
			op.ErrorResponse(http.StatusBadRequest, "Invalid request body, or the input doesn't match the tool's input schema")
			op.ErrorResponse(http.StatusNotFound, "Tool not found")
			op.ErrorResponse(http.StatusConflict, "The name has no namespace, and more than one namespace has a tool with the name")
		})
	})
}

// callToolStream calls the tool with the name, with the input, and streams
// what it logs, and its progress, as server-sent events, as it runs, and
// then its result, or its error. A client which stops reading has each
// write time out, rather than holding the tool up.
func callToolStream(w http.ResponseWriter, r *http.Request, tools *toolkit.Toolkit, name string, input json.RawMessage) {
	dw, err := deadline.New(w, streamWriteTimeout)
	if err != nil {
		httpresponse.Error(w, httpresponse.ErrInternalError.With(err.Error()))
		return
	}
	stream := httpresponse.NewTextStream(dw, "X-Accel-Buffering", "no")
	if stream == nil {
		httpresponse.Error(w, httpresponse.ErrInternalError.With("streaming isn't supported"))
		return
	}
	defer stream.Close()

	// What the tool streams, from each goroutine it streams from, until it
	// returns, when its result is written
	var mu sync.Mutex
	done := false
	session := toolkit.NewSession(name, uuid.Nil(), func(what llm.StreamType, text string) {
		mu.Lock()
		defer mu.Unlock()
		if done {
			return
		}
		switch what {
		case llm.StreamLog:
			stream.Write(schema.EventLog, schema.LogEvent{Text: text})
		case llm.StreamProgress:
			p := llm.ParseProgress(text)
			percent, _ := p.Percent()
			stream.Write(schema.EventProgress, schema.ProgressEvent{Status: p.Message, Percent: percent, Progress: p.Progress, Total: p.Total, Message: p.Message})
		}
	}, nil)
	result, err := tools.CallTool(toolkit.WithSession(r.Context(), session), name, input)

	mu.Lock()
	defer mu.Unlock()
	done = true
	if err != nil {
		stream.Write(schema.EventError, schema.ErrorEvent{Error: llm.HTTPErr(err).Error()})
	} else {
		stream.Write(schema.EventResult, schema.ResultEvent{Result: result})
	}
}

// streamWriteTimeout is the longest a write to a client's stream can take,
// before the client is taken to have stopped reading
var streamWriteTimeout = 30 * time.Second
