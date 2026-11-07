package gemini

import "encoding/json"

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
// image, inline, or a call of a function, or its response, or, in a
// reply, its thinking, which is its thought, and the signature of the
// thought before it, which is sent back with it
type part struct {
	Text             string            `json:"text,omitempty"`
	Thought          bool              `json:"thought,omitempty"`
	ThoughtSignature string            `json:"thoughtSignature,omitempty"`
	InlineData       *blob             `json:"inlineData,omitempty"`
	FunctionCall     *functionCall     `json:"functionCall,omitempty"`
	FunctionResponse *functionResponse `json:"functionResponse,omitempty"`
}

// functionCall is a model's call of a function, with its arguments, and
// its identifier, when the API gives it one
type functionCall struct {
	ID   string          `json:"id,omitempty"`
	Name string          `json:"name"`
	Args json.RawMessage `json:"args,omitempty"`
}

// functionResponse is the response of a function to a model's call of
// it, which is an object, with the call's identifier, when it has one
type functionResponse struct {
	ID       string          `json:"id,omitempty"`
	Name     string          `json:"name"`
	Response json.RawMessage `json:"response"`
}

// generateRequest is a request to POST
// /v1beta/models/<model>:streamGenerateContent, with the conversation, the
// system prompt, the functions the model can call, and how it generates
type generateRequest struct {
	Contents          []content         `json:"contents"`
	SystemInstruction *content          `json:"systemInstruction,omitempty"`
	Tools             []tool            `json:"tools,omitempty"`
	ToolConfig        *toolConfig       `json:"toolConfig,omitempty"`
	GenerationConfig  *generationConfig `json:"generationConfig,omitempty"`
}

// tool is the functions a model can call
type tool struct {
	FunctionDeclarations []functionDeclaration `json:"functionDeclarations"`
}

// functionDeclaration is a function a model can call, with the JSON schema
// of its parameters
type functionDeclaration struct {
	Name                 string          `json:"name"`
	Description          string          `json:"description"`
	ParametersJSONSchema json.RawMessage `json:"parametersJsonSchema,omitempty"`
}

// toolConfig is whether the model calls functions
type toolConfig struct {
	FunctionCallingConfig functionCallingConfig `json:"functionCallingConfig"`
}

// functionCallingConfig is whether the model calls functions, as it
// decides, it must, or it mustn't: AUTO, ANY, or NONE
type functionCallingConfig struct {
	Mode string `json:"mode"`
}

// generationConfig is how a model generates its reply
type generationConfig struct {
	Temperature        *float64        `json:"temperature,omitempty"`
	TopP               *float64        `json:"topP,omitempty"`
	TopK               *uint           `json:"topK,omitempty"`
	MaxOutputTokens    uint            `json:"maxOutputTokens,omitempty"`
	StopSequences      []string        `json:"stopSequences,omitempty"`
	Seed               *uint           `json:"seed,omitempty"`
	PresencePenalty    *float64        `json:"presencePenalty,omitempty"`
	FrequencyPenalty   *float64        `json:"frequencyPenalty,omitempty"`
	ResponseMimeType   string          `json:"responseMimeType,omitempty"`
	ResponseJSONSchema json.RawMessage `json:"responseJsonSchema,omitempty"`
	ThinkingConfig     *thinkingConfig `json:"thinkingConfig,omitempty"`
}

// thinkingConfig is how a model thinks: whether its thoughts are returned,
// as summaries, and how much it thinks, as a budget of tokens, for Gemini
// 2.5, or a level, for later models
type thinkingConfig struct {
	IncludeThoughts bool   `json:"includeThoughts,omitempty"`
	ThinkingBudget  *int   `json:"thinkingBudget,omitempty"`
	ThinkingLevel   string `json:"thinkingLevel,omitempty"`
}

// generateResponse is a piece of a reply, as it's streamed: its candidate,
// why the prompt was blocked, when it was, and the tokens used, which the
// last piece has
type generateResponse struct {
	Candidates     []candidate     `json:"candidates,omitempty"`
	PromptFeedback *promptFeedback `json:"promptFeedback,omitempty"`
	UsageMetadata  *usageMetadata  `json:"usageMetadata,omitempty"`
	Error          *apiError       `json:"error,omitempty"`
}

// candidate is a reply, as its pieces are streamed, and why it finished,
// which the last piece has
type candidate struct {
	Content      *content `json:"content,omitempty"`
	FinishReason string   `json:"finishReason,omitempty"`
}

// promptFeedback is why the prompt was blocked, when it was
type promptFeedback struct {
	BlockReason string `json:"blockReason,omitempty"`
}

// usageMetadata is the tokens a reply used: those of the prompt, of which
// those read from the cache, and of the tools' prompts, and those of the
// reply, and its thoughts
type usageMetadata struct {
	PromptTokenCount        uint `json:"promptTokenCount,omitempty"`
	CachedContentTokenCount uint `json:"cachedContentTokenCount,omitempty"`
	ToolUsePromptTokenCount uint `json:"toolUsePromptTokenCount,omitempty"`
	CandidatesTokenCount    uint `json:"candidatesTokenCount,omitempty"`
	ThoughtsTokenCount      uint `json:"thoughtsTokenCount,omitempty"`
}

// apiError is an error the API returns, with its code, and status, such as
// RESOURCE_EXHAUSTED
type apiError struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
	Status  string `json:"status,omitempty"`
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
