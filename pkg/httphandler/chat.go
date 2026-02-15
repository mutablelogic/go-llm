package httphandler

import (
	"net/http"
	"strings"

	// Packages
	agent "github.com/mutablelogic/go-llm/pkg/agent"
	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	httprequest "github.com/mutablelogic/go-server/pkg/httprequest"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	openapi "github.com/mutablelogic/go-server/pkg/openapi/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// HANDLER FUNCTIONS

// Path: /chat
func ChatHandler(manager *agent.Manager) (string, http.HandlerFunc, *openapi.PathItem) {
	return "/chat", func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodPost:
				var req schema.MultipartChatRequest
				// Read the request body into the ChatRequest struct. If a multipart file
				// was uploaded, convert it to an attachment and add it to the request.
				if err := httprequest.Read(r, &req); err != nil {
					_ = httpresponse.Error(w, err)
					return
				} else if attachment, err := req.FileAttachment(); err != nil {
					_ = httpresponse.Error(w, httpresponse.ErrBadRequest.With(err))
					return
				} else if attachment != nil {
					req.Attachments = append(req.Attachments, *attachment)
				}

				// Check Accept header for streaming vs JSON
				accept := acceptType(r)
				switch accept {
				case acceptStream:
					chatStream(w, r, manager, req.ChatRequest)
				case acceptJSON:
					chatJSON(w, r, manager, req.ChatRequest)
				default:
					_ = httpresponse.Error(w, httpresponse.Err(http.StatusNotAcceptable))
				}
			default:
				_ = httpresponse.Error(w, httpresponse.Err(http.StatusMethodNotAllowed), r.Method)
			}
		}, types.Ptr(openapi.PathItem{
			Post: &openapi.Operation{
				Description: "Send a message within a session and get a response",
			},
		})
}

// chatJSON sends the chat response as a single JSON object.
func chatJSON(w http.ResponseWriter, r *http.Request, manager *agent.Manager, req schema.ChatRequest) {
	resp, err := manager.Chat(r.Context(), req, nil)
	if err != nil {
		_ = httpresponse.Error(w, httpErr(err))
		return
	}
	_ = httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), resp)
}

// chatStream sends the chat response as a text/event-stream, emitting
// delta events for each streamed chunk and a final response event.
func chatStream(w http.ResponseWriter, r *http.Request, manager *agent.Manager, req schema.ChatRequest) {
	stream := httpresponse.NewTextStream(w)
	if stream == nil {
		_ = httpresponse.Error(w, httpresponse.ErrInternalError)
		return
	}
	defer stream.Close()

	// Stream callback: dispatch role to the appropriate SSE event name
	fn := opt.StreamFn(func(role, text string) {
		switch role {
		case schema.RoleAssistant:
			stream.Write(schema.EventAssistant, schema.StreamDelta{Role: role, Text: text})
		case schema.RoleThinking:
			stream.Write(schema.EventThinking, schema.StreamDelta{Role: role, Text: text})
		case schema.RoleTool:
			stream.Write(schema.EventTool, schema.StreamDelta{Role: role, Text: text})
		default:
			stream.Write(schema.EventAssistant, schema.StreamDelta{Role: role, Text: text})
		}
	})

	resp, err := manager.Chat(r.Context(), req, fn)
	if err != nil {
		stream.Write(schema.EventError, schema.StreamError{Error: err.Error()})
		return
	}

	// Send the final complete response
	stream.Write(schema.EventResult, resp)
}

// acceptKind classifies the negotiated response format.
type acceptKind int

const (
	acceptJSON        acceptKind = iota // application/json (or no Accept header)
	acceptStream                        // text/event-stream
	acceptUnsupported                   // unsupported media type
)

// acceptType inspects the Accept header and returns the negotiated format.
// When no Accept header is present, defaults to JSON.
func acceptType(r *http.Request) acceptKind {
	header := r.Header.Get("Accept")
	if header == "" {
		return acceptJSON
	}
	for _, part := range strings.Split(header, ",") {
		mt := strings.TrimSpace(part)
		// Strip quality parameters (e.g. ";q=0.9")
		if idx := strings.IndexByte(mt, ';'); idx >= 0 {
			mt = strings.TrimSpace(mt[:idx])
		}
		switch mt {
		case "text/event-stream":
			return acceptStream
		case "application/json", "*/*":
			return acceptJSON
		}
	}
	return acceptUnsupported
}
