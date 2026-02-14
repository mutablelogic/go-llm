package httphandler

import (
	"net/http"

	// Packages
	agent "github.com/mutablelogic/go-llm/pkg/agent"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	httprequest "github.com/mutablelogic/go-server/pkg/httprequest"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	openapi "github.com/mutablelogic/go-server/pkg/openapi/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// HANDLER FUNCTIONS

// Path: /session/{session}/ask
// POST: Stateless generation using the session's configuration (provider, model,
// system prompt) but does NOT store any messages in the session.
func SessionAskHandler(manager *agent.Manager) (string, http.HandlerFunc, *openapi.PathItem) {
	return "/session/{session}/ask", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				_ = httpresponse.Error(w, httpresponse.Err(http.StatusMethodNotAllowed), r.Method)
				return
			}
			completionHandler(w, r, manager, false)
		}, types.Ptr(openapi.PathItem{
			Post: &openapi.Operation{
				Description: "Generate a response without storing messages in the session",
			},
		})
}

// Path: /session/{session}/chat
// POST: Stateful generation within a session's conversation. Stores the
// exchange and handles tool-call loops if a toolkit is configured.
func SessionChatHandler(manager *agent.Manager) (string, http.HandlerFunc, *openapi.PathItem) {
	return "/session/{session}/chat", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				_ = httpresponse.Error(w, httpresponse.Err(http.StatusMethodNotAllowed), r.Method)
				return
			}
			completionHandler(w, r, manager, true)
		}, types.Ptr(openapi.PathItem{
			Post: &openapi.Operation{
				Description: "Generate a response within a session conversation, storing messages",
			},
		})
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// completionHandler is the shared handler for ask and chat endpoints.
func completionHandler(w http.ResponseWriter, r *http.Request, manager *agent.Manager, stateful bool) {
	// Parse the request body (JSON or multipart/form-data)
	var req schema.CompletionRequest
	if err := httprequest.Read(r, &req); err != nil {
		_ = httpresponse.Error(w, err)
		return
	}

	// Get the session
	id := r.PathValue("session")
	session, err := manager.GetSession(r.Context(), schema.GetSessionRequest{ID: id})
	if err != nil {
		_ = httpresponse.Error(w, httpErr(err))
		return
	}

	// Build the message from the request
	message, err := messageFromRequest(&req)
	if err != nil {
		_ = httpresponse.Error(w, httpresponse.ErrBadRequest.With(err))
		return
	}

	// Call Ask or Chat
	var resp *schema.Message
	if stateful {
		resp, err = manager.Chat(r.Context(), session, message)
	} else {
		resp, err = manager.Ask(r.Context(), session, message)
	}
	if err != nil {
		_ = httpresponse.Error(w, httpErr(err))
		return
	}

	// Return the response
	_ = httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), schema.CompletionResponse{
		Role:    resp.Role,
		Content: resp.Content,
		Result:  resp.Result,
	})
}

// messageFromRequest converts a CompletionRequest into a schema.Message.
func messageFromRequest(req *schema.CompletionRequest) (*schema.Message, error) {
	// Build content blocks
	blocks := []schema.ContentBlock{
		{Text: types.Ptr(req.Text)},
	}

	// Add attachments from multipart upload
	attachments, err := req.Attachments()
	if err != nil {
		return nil, err
	}
	blocks = append(blocks, attachments...)

	return &schema.Message{
		Role:    schema.RoleUser,
		Content: blocks,
	}, nil
}
