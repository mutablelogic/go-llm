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

// part is a part of a message, which is its text, or data, such as an
// image, inline
type part struct {
	Text       string `json:"text,omitempty"`
	InlineData *blob  `json:"inlineData,omitempty"`
}

// blob is data, inline, with its type, encoded as base64, as JSON encodes
// bytes
type blob struct {
	MimeType string `json:"mimeType"`
	Data     []byte `json:"data"`
}

// embedRequest is a request to embed a message, in a request to POST
// /v1beta/models/<model>:batchEmbedContents, with the model, which is
// models/<model>, what the embedding is for, the title of the document it's
// from, and the number of dimensions of its vector
type embedRequest struct {
	Model                string  `json:"model"`
	Content              content `json:"content"`
	TaskType             string  `json:"taskType,omitempty"`
	Title                string  `json:"title,omitempty"`
	OutputDimensionality uint    `json:"outputDimensionality,omitempty"`
}

// batchEmbedRequest is a request to embed messages, each with its own
// request
type batchEmbedRequest struct {
	Requests []embedRequest `json:"requests"`
}

// batchEmbedResponse is the vectors of the messages, in their order
type batchEmbedResponse struct {
	Embeddings []struct {
		Values []float32 `json:"values"`
	} `json:"embeddings"`
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
