package httpclient_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	// Packages
	uuid "github.com/google/uuid"
	goclient "github.com/mutablelogic/go-client"
	httpclient "github.com/mutablelogic/go-llm/kernel/httpclient"
	schema "github.com/mutablelogic/go-llm/kernel/schema"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	types "github.com/mutablelogic/go-server/pkg/types"
)

func newChannelServer(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/session/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) != 4 || parts[0] != "api" || parts[1] != "session" || parts[3] != "channel" {
			http.NotFound(w, r)
			return
		}

		sessionID, err := uuid.Parse(parts[2])
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		stream, err := httpresponse.NewJSONStream(w, r)
		if err != nil {
			_ = httpresponse.Error(w, err)
			return
		}
		defer stream.Close()

		sessionFrame, err := json.Marshal(map[string]any{"id": sessionID.String()})
		if err != nil {
			_ = httpresponse.Error(w, err)
			return
		}
		if err := stream.Send(json.RawMessage(sessionFrame)); err != nil {
			return
		}

		for frame := range stream.Recv() {
			if frame == nil {
				continue
			}

			var req schema.SessionChannelRequest
			if err := json.Unmarshal(frame, &req); err != nil {
				payload, marshalErr := json.Marshal(httpresponse.ErrResponse{Code: http.StatusBadRequest, Reason: err.Error()})
				if marshalErr != nil {
					return
				}
				if err := stream.Send(json.RawMessage(payload)); err != nil {
					return
				}
				continue
			}

			response, err := json.Marshal(schema.ChatResponse{
				CompletionResponse: schema.CompletionResponse{
					Role:   schema.RoleAssistant,
					Result: schema.ResultStop,
					Content: []schema.ContentBlock{
						{Text: types.Ptr("echo: " + req.Text)},
					},
				},
			})
			if err != nil {
				return
			}
			if err := stream.Send(json.RawMessage(response)); err != nil {
				return
			}
		}
	})

	return httptest.NewServer(mux)
}

func newChannelClient(t *testing.T, serverURL string) *httpclient.Client {
	t.Helper()

	client, err := httpclient.New(serverURL + "/api")
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestChannel(t *testing.T) {
	server := newChannelServer(t)
	defer server.Close()

	httpClient := newChannelClient(t, server.URL)
	sessionID := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	err := httpClient.Channel(context.Background(), sessionID, func(ctx context.Context, stream goclient.JSONStream) error {
		var session struct {
			ID string `json:"id"`
		}
		if err := recvChannelFrame(ctx, stream, &session); err != nil {
			return err
		}
		if session.ID != sessionID.String() {
			return fmt.Errorf("expected session %q, got %q", sessionID, session.ID)
		}

		payload, err := json.Marshal(schema.SessionChannelRequest{Text: "hello"})
		if err != nil {
			return err
		}
		if err := stream.Send(json.RawMessage(payload)); err != nil {
			return err
		}

		var response schema.ChatResponse
		if err := recvChannelFrame(ctx, stream, &response); err != nil {
			return err
		}
		if response.Role != schema.RoleAssistant {
			return fmt.Errorf("expected role %q, got %q", schema.RoleAssistant, response.Role)
		}
		if len(response.Content) == 0 || response.Content[0].Text == nil {
			return fmt.Errorf("expected response text")
		}
		if got := *response.Content[0].Text; got != "echo: hello" {
			return fmt.Errorf("expected %q, got %q", "echo: hello", got)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestChannelNilSession(t *testing.T) {
	server := newChannelServer(t)
	defer server.Close()

	httpClient := newChannelClient(t, server.URL)
	err := httpClient.Channel(context.Background(), uuid.Nil, func(context.Context, goclient.JSONStream) error {
		return nil
	})
	if err == nil {
		t.Fatal("expected error for nil session")
	}
}

func TestChannelNilCallback(t *testing.T) {
	server := newChannelServer(t)
	defer server.Close()

	httpClient := newChannelClient(t, server.URL)
	err := httpClient.Channel(context.Background(), uuid.MustParse("11111111-1111-1111-1111-111111111111"), nil)
	if err == nil {
		t.Fatal("expected error for nil callback")
	}
}

func recvChannelFrame(ctx context.Context, stream goclient.JSONStream, target any) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case frame, ok := <-stream.Recv():
			if !ok {
				return fmt.Errorf("stream closed")
			}
			if frame == nil {
				continue
			}
			return json.Unmarshal(frame, target)
		}
	}
}
