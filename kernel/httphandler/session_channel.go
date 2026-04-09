package httphandler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

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
			_ = sessionChannel(r.Context(), manager, w, r)
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

func sessionChannel(ctx context.Context, manager *llmmanager.Manager, w http.ResponseWriter, r *http.Request) error {
	sessionID, err := uuid.Parse(r.PathValue("session"))
	if err != nil {
		return httpresponse.Error(w, httpresponse.ErrBadRequest, err)
	}
	if err := validateSessionChannelHeaders(r); err != nil {
		return httpresponse.Error(w, err)
	}

	user := middleware.UserFromContext(ctx)
	if _, err := manager.GetSession(ctx, sessionID, user); err != nil {
		return httpresponse.Error(w, schema.HTTPErr(err))
	}

	stream, err := httpresponse.NewJSONStream(w, r)
	if err != nil {
		return httpresponse.Error(w, err)
	}
	defer stream.Close()

	for {
		frame, err := stream.Recv()
		switch {
		case errors.Is(err, io.EOF):
			return nil
		case err != nil:
			_ = sendSessionChannelError(stream, httpresponse.ErrBadRequest.With(err))
			return nil
		}

		var req schema.SessionChannelRequest
		if err := json.Unmarshal(frame, &req); err != nil {
			if sendErr := sendSessionChannelError(stream, httpresponse.ErrBadRequest.With(err)); sendErr != nil {
				return sendErr
			}
			continue
		}

		resp, err := manager.Chat(stream.Context(), schema.ChatRequest{
			Session:       sessionID,
			Text:          req.Text,
			Tools:         req.Tools,
			MaxIterations: req.MaxIterations,
			SystemPrompt:  req.SystemPrompt,
		}, nil, user)
		if err != nil {
			if sendErr := sendSessionChannelError(stream, schema.HTTPErr(err)); sendErr != nil {
				return sendErr
			}
			continue
		}

		payload, err := json.Marshal(resp)
		if err != nil {
			if sendErr := sendSessionChannelError(stream, httpresponse.ErrInternalError.With(err)); sendErr != nil {
				return sendErr
			}
			continue
		}
		if err := stream.Send(payload); err != nil {
			if errors.Is(err, io.ErrClosedPipe) {
				return nil
			}
			return err
		}
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

func sendSessionChannelError(stream *httpresponse.JSONStream, err error) error {
	payload, marshalErr := marshalSessionChannelError(err)
	if marshalErr != nil {
		return marshalErr
	}
	return stream.Send(payload)
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
