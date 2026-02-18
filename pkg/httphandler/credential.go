package httphandler

import (
	"net/http"
	"net/url"

	// Packages
	manager "github.com/mutablelogic/go-llm/pkg/manager"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
	httprequest "github.com/mutablelogic/go-server/pkg/httprequest"
	httpresponse "github.com/mutablelogic/go-server/pkg/httpresponse"
	openapi "github.com/mutablelogic/go-server/pkg/openapi/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// HANDLER FUNCTIONS

// Path: /credential/{url}
func CredentialHandler(manager *manager.Manager) (string, http.HandlerFunc, *openapi.PathItem) {
	return "/credential/{url}", func(w http.ResponseWriter, r *http.Request) {
			rawURL, err := url.PathUnescape(r.PathValue("url"))
			if err != nil {
				_ = httpresponse.Error(w, httpresponse.ErrBadRequest.With(err))
				return
			}
			switch r.Method {
			case http.MethodGet:
				resp, err := manager.GetCredential(r.Context(), rawURL)
				if err != nil {
					_ = httpresponse.Error(w, httpErr(err))
					return
				}
				_ = httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), resp)
			case http.MethodPost:
				var req schema.OAuthCredentials
				if err := httprequest.Read(r, &req); err != nil {
					_ = httpresponse.Error(w, err)
					return
				}
				if err := manager.SetCredential(r.Context(), rawURL, req); err != nil {
					_ = httpresponse.Error(w, httpErr(err))
					return
				}
				w.WriteHeader(http.StatusNoContent)
			case http.MethodDelete:
				if err := manager.DeleteCredential(r.Context(), rawURL); err != nil {
					_ = httpresponse.Error(w, httpErr(err))
					return
				}
				w.WriteHeader(http.StatusNoContent)
			default:
				_ = httpresponse.Error(w, httpresponse.Err(http.StatusMethodNotAllowed), r.Method)
			}
		}, types.Ptr(openapi.PathItem{
			Get: &openapi.Operation{
				Description: "Get a credential by server URL",
			},
			Post: &openapi.Operation{
				Description: "Store or update a credential for a server URL",
			},
			Delete: &openapi.Operation{
				Description: "Delete a credential by server URL",
			},
		})
}
