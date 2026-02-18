package httphandler

import (
	"net/http"
	"net/url"
	"strings"

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
				_ = httpresponse.JSON(w, http.StatusOK, httprequest.Indent(r), redactCredential(resp))
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

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

const redacted = "***"

// redactString returns the first 3 characters followed by asterisks for the rest.
// Returns fully redacted if the string is 6 characters or shorter.
func redactString(s string) string {
	if len(s) <= 6 {
		return redacted
	}
	return s[:3] + strings.Repeat("*", len(s)-3)
}

// redactCredential returns a shallow copy of the credential with sensitive
// token fields replaced by a placeholder. The original is not modified.
func redactCredential(cred *schema.OAuthCredentials) *schema.OAuthCredentials {
	result := *cred
	if result.Token != nil {
		tokenCopy := *result.Token
		if tokenCopy.AccessToken != "" {
			tokenCopy.AccessToken = redactString(tokenCopy.AccessToken)
		}
		if tokenCopy.RefreshToken != "" {
			tokenCopy.RefreshToken = redactString(tokenCopy.RefreshToken)
		}
		result.Token = &tokenCopy
	}
	if result.ClientSecret != "" {
		result.ClientSecret = redactString(result.ClientSecret)
	}
	return &result
}
