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
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	openapi "github.com/mutablelogic/go-server/pkg/openapi/schema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// HANDLER FUNCTIONS

// Path: credential/{url}
func CredentialHandler(manager *manager.Manager) (string, http.HandlerFunc, *openapi.PathItem) {
	urlParam := openapi.Parameter{
		Name:        "url",
		In:          openapi.ParameterInPath,
		Description: "Server URL",
		Required:    true,
		Schema:      pathParamSchema,
	}
	credSchema, _ := jsonschema.For[schema.OAuthCredentials]()
	return "credential/{url}", func(w http.ResponseWriter, r *http.Request) {
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
				Tags:        []string{"Credential"},
				Description: "Get a credential by server URL",
				Parameters:  []openapi.Parameter{urlParam},
				Responses: map[string]openapi.Response{
					"200":     {Description: "Credential details", Content: map[string]openapi.MediaType{types.ContentTypeJSON: {Schema: credSchema}}},
					"default": openapi.ErrorResponse("An error occurred"),
				},
			},
			Post: &openapi.Operation{
				Tags:        []string{"Credential"},
				Description: "Store or update a credential for a server URL",
				Parameters:  []openapi.Parameter{urlParam},
				RequestBody: &openapi.RequestBody{
					Required: true,
					Content:  map[string]openapi.MediaType{types.ContentTypeJSON: {Schema: credSchema}},
				},
				Responses: map[string]openapi.Response{
					"204":     {Description: "Credential stored"},
					"default": openapi.ErrorResponse("An error occurred"),
				},
			},
			Delete: &openapi.Operation{
				Tags:        []string{"Credential"},
				Description: "Delete a credential by server URL",
				Parameters:  []openapi.Parameter{urlParam},
				Responses: map[string]openapi.Response{
					"204":     {Description: "Credential deleted"},
					"default": openapi.ErrorResponse("An error occurred"),
				},
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

// redactCredential returns a copy of the credential with sensitive
// token fields partially masked. For strings longer than 6 characters, the
// first 3 characters are preserved and the rest replaced with asterisks.
// Shorter strings are fully replaced.
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
