package httphandler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

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

const channelFeedDedupTTL = 15 * time.Minute

func sessionChannel(req *http.Request, manager *llmmanager.Manager, session *schema.Session, stream httpresponse.JSONStream) error {
	ctx, cancel := context.WithCancel(req.Context())
	defer cancel()
	var sendMu sync.Mutex
	deduper := newChannelDeduper(channelFeedDedupTTL)

	sendRaw := func(payload json.RawMessage) error {
		sendMu.Lock()
		defer sendMu.Unlock()
		return stream.Send(payload)
	}
	sendError := func(err error) error {
		payload, marshalErr := marshalSessionChannelError(err)
		if marshalErr != nil {
			return marshalErr
		}
		return sendRaw(payload)
	}
	sendDelta := func(delta schema.StreamDelta) error {
		payload, err := json.Marshal(delta)
		if err != nil {
			return err
		}
		return sendRaw(json.RawMessage(payload))
	}
	sendResponse := func(response schema.ChatResponse) error {
		payload, err := json.Marshal(response)
		if err != nil {
			return err
		}
		return sendRaw(json.RawMessage(payload))
	}

	sessionFrame, err := json.Marshal(session)
	if err != nil {
		return sendError(httpresponse.ErrInternalError.With(err))
	}
	if err := sendRaw(json.RawMessage(sessionFrame)); err != nil {
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

	if err := manager.SubscribeSession(ctx, session.ID, func(messages []*schema.Message) {
		for _, message := range deduper.Filter(messages) {
			if message == nil {
				continue
			}
			if !channelShouldSendMessage(message) {
				continue
			}
			if err := sendResponse(channelResponseFromMessage(message)); err != nil {
				reportChatErr(err)
				return
			}
		}
	}, user); err != nil {
		return sendError(schema.HTTPErr(err))
	}

	startChat := func(channelReq schema.SessionChannelRequest) {
		busy.Store(true)
		chats.Add(1)
		deduper.RememberMessage(channelRequestMessage(channelReq))

		go func() {
			defer chats.Done()
			defer busy.Store(false)

			chatCtx, chatCancel := context.WithCancel(ctx)
			defer chatCancel()
			streamed := make(map[string]string)

			streamFn := func(role, text string) {
				if strings.TrimSpace(text) == "" {
					return
				}
				streamed[role] += text
				if err := sendDelta(schema.StreamDelta{Role: role, Text: text}); err != nil {
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
				reportChatErr(sendError(schema.HTTPErr(err)))
				return
			}
			if chatCtx.Err() != nil {
				return
			}
			for role, text := range streamed {
				deduper.RememberText(role, text)
			}
			if resp == nil {
				return
			}
			deduper.RememberResponse(*resp)

			reportChatErr(sendResponse(*resp))
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
				if sendErr := sendError(httpresponse.ErrBadRequest.With(err)); sendErr != nil {
					cancel()
					chats.Wait()
					return sendErr
				}
				continue
			}

			if busy.Load() {
				if sendErr := sendError(httpresponse.ErrConflict.With("chat already in progress")); sendErr != nil {
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

type channelDeduper struct {
	mu   sync.Mutex
	ttl  time.Duration
	seen map[string]channelDedupEntry
}

type channelDedupEntry struct {
	expiry time.Time
	count  uint
}

func newChannelDeduper(ttl time.Duration) *channelDeduper {
	return &channelDeduper{ttl: ttl, seen: make(map[string]channelDedupEntry)}
}

func (d *channelDeduper) RememberMessage(message *schema.Message) {
	if message == nil {
		return
	}
	for _, key := range channelMessageKeys(message) {
		d.rememberKey(key)
	}
}

func (d *channelDeduper) RememberResponse(response schema.ChatResponse) {
	message := &schema.Message{
		ID:      response.ID,
		Session: response.Session,
		Role:    response.Role,
		Content: response.Content,
		Result:  response.Result,
	}
	d.RememberMessage(message)
}

func (d *channelDeduper) RememberText(role, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	message := &schema.Message{Role: role, Content: []schema.ContentBlock{}}
	if role == schema.RoleThinking {
		message.Content = append(message.Content, schema.ContentBlock{Thinking: types.Ptr(text)})
	} else {
		message.Content = append(message.Content, schema.ContentBlock{Text: types.Ptr(text)})
	}
	d.RememberMessage(message)
}

func (d *channelDeduper) Filter(messages []*schema.Message) []*schema.Message {
	now := time.Now()
	d.mu.Lock()
	defer d.mu.Unlock()
	d.pruneLocked(now)

	result := make([]*schema.Message, 0, len(messages))
	for _, message := range messages {
		if message == nil {
			continue
		}
		keys := channelMessageKeys(message)
		if len(keys) == 0 {
			result = append(result, message)
			continue
		}
		if d.consumeLocked(now, keys...) {
			continue
		}
		result = append(result, message)
	}
	return result
}

func (d *channelDeduper) consumeLocked(now time.Time, keys ...string) bool {
	matched := false
	for _, key := range keys {
		if key == "" {
			continue
		}
		entry, exists := d.seen[key]
		if !exists || !now.Before(entry.expiry) || entry.count == 0 {
			continue
		}
		matched = true
	}
	if !matched {
		return false
	}
	for _, key := range keys {
		if key == "" {
			continue
		}
		entry, exists := d.seen[key]
		if !exists || !now.Before(entry.expiry) || entry.count == 0 {
			continue
		}
		if entry.count == 1 {
			delete(d.seen, key)
		} else {
			entry.count--
			d.seen[key] = entry
		}
	}
	return true
}

func (d *channelDeduper) rememberKey(key string) {
	if key == "" {
		return
	}
	now := time.Now()
	d.mu.Lock()
	defer d.mu.Unlock()
	d.pruneLocked(now)
	entry := d.seen[key]
	entry.count++
	entry.expiry = now.Add(d.ttl)
	d.seen[key] = entry
}

func (d *channelDeduper) pruneLocked(now time.Time) {
	for key, entry := range d.seen {
		if !now.Before(entry.expiry) || entry.count == 0 {
			delete(d.seen, key)
		}
	}
}

func channelRequestMessage(req schema.SessionChannelRequest) *schema.Message {
	text := strings.TrimSpace(req.Text)
	if text == "" {
		return nil
	}
	return &schema.Message{
		Role:    schema.RoleUser,
		Content: []schema.ContentBlock{{Text: types.Ptr(text)}},
		Result:  schema.ResultStop,
	}
}

func channelResponseFromMessage(message *schema.Message) schema.ChatResponse {
	if message == nil {
		return schema.ChatResponse{}
	}
	return schema.ChatResponse{
		ID:      message.ID,
		Session: message.Session,
		CompletionResponse: schema.CompletionResponse{
			Role:    message.Role,
			Content: message.Content,
			Result:  message.Result,
		},
	}
}

func channelShouldSendMessage(message *schema.Message) bool {
	if message == nil {
		return false
	}
	if message.Result == schema.ResultToolCall {
		return false
	}
	for _, block := range message.Content {
		switch {
		case block.ToolCall != nil:
			return false
		case block.ToolResult != nil:
			return false
		}
	}
	for _, block := range message.Content {
		switch {
		case block.Text != nil && strings.TrimSpace(*block.Text) != "":
			return true
		case block.Thinking != nil && strings.TrimSpace(*block.Thinking) != "":
			return true
		case block.Attachment != nil:
			return true
		}
	}
	return false
}

func channelMessageKeys(message *schema.Message) []string {
	if message == nil {
		return nil
	}
	keys := make([]string, 0, 2)
	if message.ID > 0 {
		keys = append(keys, "id:"+types.Stringify(message.ID))
	}
	encoded, err := json.Marshal(struct {
		Role    string                `json:"role"`
		Content []schema.ContentBlock `json:"content"`
		Result  string                `json:"result,omitempty"`
	}{
		Role:    message.Role,
		Content: message.Content,
		Result:  message.Result.String(),
	})
	if err == nil {
		keys = append(keys, "content:"+string(encoded))
	}
	return keys
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
