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

// Path: /ask
func AskHandler(manager *agent.Manager) (string, http.HandlerFunc, *openapi.PathItem) {
	return "/ask", func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodPost:
				var req schema.MultipartAskRequest
				// Read the request body into the AskRequest struct. If a multipart file
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

				// Perform operation and return response
				resp, err := manager.Ask(r.Context(), req.AskRequest, nil)
				if err != nil {
					_ = httpresponse.Error(w, httpErr(err))
					return
				}
				_ = httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), resp)
			default:
				_ = httpresponse.Error(w, httpresponse.Err(http.StatusMethodNotAllowed), r.Method)
			}
		}, types.Ptr(openapi.PathItem{
			Post: &openapi.Operation{
				Description: "Send a stateless message and get a response",
			},
		})
}
