package gemini

///////////////////////////////////////////////////////////////////////////////
// TYPES

// apiModel is a model, as GET /v1beta/models returns it: its name, which is
// models/<model>, the model it's a version of, its name for people, what it
// is, its limits, the methods it supports, such as generateContent, or
// embedContent, its sampling options, and whether it thinks
type apiModel struct {
	Name                       string   `json:"name"`
	BaseModelID                string   `json:"baseModelId,omitempty"`
	Version                    string   `json:"version,omitempty"`
	DisplayName                string   `json:"displayName,omitempty"`
	Description                string   `json:"description,omitempty"`
	InputTokenLimit            uint     `json:"inputTokenLimit,omitempty"`
	OutputTokenLimit           uint     `json:"outputTokenLimit,omitempty"`
	SupportedGenerationMethods []string `json:"supportedGenerationMethods,omitempty"`
	Temperature                *float64 `json:"temperature,omitempty"`
	MaxTemperature             *float64 `json:"maxTemperature,omitempty"`
	TopP                       *float64 `json:"topP,omitempty"`
	TopK                       *uint    `json:"topK,omitempty"`
	Thinking                   *bool    `json:"thinking,omitempty"`
}

// modelList is a page of models, and the token of the next page, which is
// empty, when there are no more
type modelList struct {
	Models        []apiModel `json:"models"`
	NextPageToken string     `json:"nextPageToken,omitempty"`
}

// content is a message, as the API has it: who it's from, which is user,
// or model, and its parts
type content struct {
	Role  string `json:"role,omitempty"`
	Parts []part `json:"parts"`
}

// part is a part of a message, which is its text
type part struct {
	Text string `json:"text,omitempty"`
}

// countRequest is a request to POST /v1beta/models/<model>:countTokens,
// which counts the tokens of the contents
type countRequest struct {
	Contents []content `json:"contents"`
}

// countResponse is the tokens which the contents are
type countResponse struct {
	TotalTokens uint `json:"totalTokens"`
}
