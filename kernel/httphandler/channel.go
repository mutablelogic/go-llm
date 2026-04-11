package httphandler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	// Packages
	middleware "github.com/djthorpe/go-auth/pkg/middleware"
	uuid "github.com/google/uuid"
	llmmanager "github.com/mutablelogic/go-llm/kernel/manager"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	httprequest "github.com/mutablelogic/go-server/pkg/httprequest"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	opts "github.com/mutablelogic/go-server/pkg/openapi"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func SessionChannelHandler(manager *llmmanager.Manager) (string, *jsonschema.Schema, httprequest.PathItem) {
	return "session/{session}/channel", jsonschema.MustFor[schema.SessionIDSelector](), httprequest.NewPathItem(
		"Session channel operations",
		"Open an NDJSON channel for sending chat turns to an existing session and receiving responses.",
		"Sessions",
	).Post(
		func(w http.ResponseWriter, r *http.Request) {
			if session, err := getSessionChannelRequest(r, manager); err != nil {
				httpresponse.Error(w, err)
				return
			} else if stream, err := httpresponse.NewJSONStream(w, r); err != nil {
				httpresponse.Error(w, err)
				return
			} else {
				defer stream.Close()
				_ = sessionChannel(r, manager, session, stream)
			}
		},
		"Open session channel",
		opts.WithRequest(types.ContentTypeJSONStream, jsonschema.MustFor[schema.SessionChannelRequest]()),
		opts.WithResponse(200, types.ContentTypeJSONStream, jsonschema.MustFor[schema.ChatResponse](), "NDJSON stream of chat responses and error frames."),
		opts.WithErrorResponse(400, "Invalid session ID, request headers, or channel frame."),
		opts.WithErrorResponse(404, "Session not found."),
		opts.WithErrorResponse(406, "Unsupported Accept header."),
		opts.WithErrorResponse(415, "Unsupported Content-Type header."),
	)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

func sessionChannel(req *http.Request, manager *llmmanager.Manager, session *schema.Session, stream httpresponse.JSONStream) error {
	ctx, cancel := context.WithCancel(req.Context())
	defer cancel()

	sessionFrame, err := json.Marshal(session)
	if err != nil {
		return sendSessionChannelError(stream, httpresponse.ErrInternalError.With(err))
	}
	if err := stream.Send(json.RawMessage(sessionFrame)); err != nil {
		return err
	}

	user := middleware.UserFromContext(req.Context())
	recv := stream.Recv()
	var busy atomic.Bool
	var chats sync.WaitGroup
	chatErr := make(chan error, 1)

	reportChatErr := func(err error) {
		if err == nil {
			return
		}
		select {
		case chatErr <- err:
			cancel()
		default:
		}
	}

	startChat := func(channelReq schema.SessionChannelRequest) {
		busy.Store(true)
		chats.Add(1)

		go func() {
			defer chats.Done()
			defer busy.Store(false)

			chatCtx, chatCancel := context.WithCancel(ctx)
			defer chatCancel()

			streamFn := func(role, text string) {
				if strings.TrimSpace(text) == "" {
					return
				}
				if err := sendSessionChannelDelta(stream, schema.StreamDelta{Role: role, Text: text}); err != nil {
					reportChatErr(err)
					chatCancel()
				}
			}

			resp, err := manager.Chat(chatCtx, schema.ChatRequest{
				Session:       session.ID,
				Text:          channelReq.Text,
				Tools:         channelReq.Tools,
				MaxIterations: channelReq.MaxIterations,
				SystemPrompt:  channelReq.SystemPrompt,
			}, streamFn, user)
			if err != nil {
				if chatCtx.Err() != nil {
					return
				}
				reportChatErr(sendSessionChannelError(stream, schema.HTTPErr(err)))
				return
			}
			if chatCtx.Err() != nil {
				return
			}

			payload, err := json.Marshal(resp)
			if err != nil {
				reportChatErr(sendSessionChannelError(stream, httpresponse.ErrInternalError.With(err)))
				return
			}

			reportChatErr(stream.Send(json.RawMessage(payload)))
		}()
	}

	for {
		select {
		case err := <-chatErr:
			chats.Wait()
			return err
		case frame, ok := <-recv:
			if !ok {
				cancel()
				chats.Wait()
				select {
				case err := <-chatErr:
					return err
				default:
					return nil
				}
			}
			if frame == nil {
				continue
			}

			var channelReq schema.SessionChannelRequest
			if err := json.Unmarshal(frame, &channelReq); err != nil {
				if sendErr := sendSessionChannelError(stream, httpresponse.ErrBadRequest.With(err)); sendErr != nil {
					cancel()
					chats.Wait()
					return sendErr
				}
				continue
			}

			if busy.Load() {
				if sendErr := sendSessionChannelError(stream, httpresponse.ErrConflict.With("chat already in progress")); sendErr != nil {
					cancel()
					chats.Wait()
					return sendErr
				}
				continue
			}

			startChat(channelReq)
		}
	}
}

func getSessionChannelRequest(r *http.Request, manager *llmmanager.Manager) (*schema.Session, error) {
	session, err := uuid.Parse(r.PathValue("session"))
	if err != nil {
		return nil, httpresponse.ErrBadRequest.With(err)
	}
	if err := validateSessionChannelHeaders(r); err != nil {
		return nil, err
	}
	if manager == nil {
		return nil, httpresponse.ErrInternalError.With("session channel manager is nil")
	}
	if session, err := manager.GetSession(r.Context(), session, middleware.UserFromContext(r.Context())); err != nil {
		return nil, schema.HTTPErr(err)
	} else {
		return session, nil
	}
}

func validateSessionChannelHeaders(r *http.Request) error {
	if header := strings.TrimSpace(r.Header.Get(types.ContentTypeHeader)); header != "" {
		contentType, err := types.RequestContentType(r)
		if err != nil {
			return httpresponse.ErrBadRequest.With(err)
		}
		if contentType != types.ContentTypeJSONStream {
			return httpresponse.Err(http.StatusUnsupportedMediaType).Withf("expected %q request body, got %q", types.ContentTypeJSONStream, contentType)
		}
	}
	if !acceptsJSONStream(r.Header.Get(types.ContentAcceptHeader)) {
		return httpresponse.Err(http.StatusNotAcceptable).Withf("expected Accept %q", types.ContentTypeJSONStream)
	}
	return nil
}

func acceptsJSONStream(header string) bool {
	if strings.TrimSpace(header) == "" {
		return true
	}
	for _, part := range strings.Split(header, ",") {
		value := strings.TrimSpace(part)
		if idx := strings.IndexByte(value, ';'); idx >= 0 {
			value = strings.TrimSpace(value[:idx])
		}
		if value == types.ContentTypeJSONStream || value == types.ContentTypeAny {
			return true
		}
	}
	return false
}

func sendSessionChannelError(stream httpresponse.JSONStream, err error) error {
	payload, marshalErr := marshalSessionChannelError(err)
	if marshalErr != nil {
		return marshalErr
	}
	return stream.Send(payload)
}

func sendSessionChannelDelta(stream httpresponse.JSONStream, delta schema.StreamDelta) error {
	payload, err := json.Marshal(delta)
	if err != nil {
		return err
	}
	return stream.Send(json.RawMessage(payload))
}

func marshalSessionChannelError(err error) (json.RawMessage, error) {
	if err == nil {
		return nil, nil
	}

	resp := httpresponse.ErrResponse{
		Code:   http.StatusInternalServerError,
		Reason: err.Error(),
	}
	var code httpresponse.Err
	if errors.As(err, &code) {
		resp.Code = int(code)
	}

	data, err := json.Marshal(resp)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}
