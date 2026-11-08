package schema

///////////////////////////////////////////////////////////////////////////////
// TYPES

// AuthorizeRequest is the request to authorize a connector's server, with
// the URL its authorization server redirects to, once a person has signed
// in, which is the API's callback, of the request's origin, when it's
// omitted
type AuthorizeRequest struct {
	Redirect string `json:"redirect,omitempty" help:"The URL the authorization server redirects to, once a person has signed in, which is the API's callback, of the request's origin, when it's omitted" format:"uri" optional:""`
}

// AuthorizeResponse is the URL a person signs in at, to authorize the
// connector's server
type AuthorizeResponse struct {
	URL string `json:"url" help:"The URL a person signs in at, to authorize the connector's server" format:"uri" readonly:""`
}
