package google

import "encoding/json"

///////////////////////////////////////////////////////////////////////////////
// TYPES - Gemini REST API wire format
//
// Reference: https://ai.google.dev/api/generate-content
//            https://ai.google.dev/api/caching (Content, Part, Tool types)
//            https://ai.google.dev/api/models
//            https://ai.google.dev/api/embeddings

///////////////////////////////////////////////////////////////////////////////
// CONTENT & PARTS

// geminiContent is the base structured datatype containing multi-part content
// of a message turn. Maps to the REST API "Content" resource.
type geminiContent struct {
	Parts []*geminiPart `json:"parts"`
	Role  string        `json:"role,omitempty"`
}

// geminiPart is a single unit within a Content message.
// Exactly one of the data fields should be set (text, inlineData, fileData,
// functionCall, functionResponse). The thought/thoughtSignature fields are
// orthogonal flags used for extended thinking.
type geminiPart struct {
	// Thinking metadata
	Thought          bool   `json:"thought,omitempty"`
	ThoughtSignature string `json:"thoughtSignature,omitempty"` // base64-encoded

	// Data — exactly one should be populated
	Text             string                `json:"text,omitempty"`
	InlineData       *geminiBlob           `json:"inlineData,omitempty"`
	FileData         *geminiFileData       `json:"fileData,omitempty"`
	FunctionCall     *geminiFunctionCall   `json:"functionCall,omitempty"`
	FunctionResponse *geminiFunctionResult `json:"functionResponse,omitempty"`
}

// geminiBlob carries raw inline media bytes (images, audio, etc.)
type geminiBlob struct {
	MIMEType string `json:"mimeType"`
	Data     string `json:"data"` // base64-encoded
}

// geminiFileData references media by URI (e.g. from the Files API)
type geminiFileData struct {
	MIMEType string `json:"mimeType,omitempty"`
	FileURI  string `json:"fileUri"`
}

// geminiFunctionCall is the model's request to invoke a tool
type geminiFunctionCall struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args,omitempty"`
}

// geminiFunctionResult is the client-supplied result of a tool invocation
type geminiFunctionResult struct {
	Name     string         `json:"name"`
	Response map[string]any `json:"response"`
}

///////////////////////////////////////////////////////////////////////////////
// GENERATE CONTENT — REQUEST

// geminiGenerateRequest is the request body for
// POST /v1beta/{model=models/*}:generateContent  and
// POST /v1beta/{model=models/*}:streamGenerateContent
type geminiGenerateRequest struct {
	Contents          []*geminiContent       `json:"contents"`
	Tools             []*geminiTool          `json:"tools,omitempty"`
	ToolConfig        *geminiToolConfig      `json:"toolConfig,omitempty"`
	SafetySettings    []*geminiSafetySetting `json:"safetySettings,omitempty"`
	SystemInstruction *geminiContent         `json:"systemInstruction,omitempty"`
	GenerationConfig  geminiGenerationConfig `json:"generationConfig,omitzero"`
	CachedContent     string                 `json:"cachedContent,omitempty"`
}

///////////////////////////////////////////////////////////////////////////////
// GENERATE CONTENT — RESPONSE

// geminiGenerateResponse is the response from generateContent and each chunk
// in the streamGenerateContent stream.
type geminiGenerateResponse struct {
	Candidates     []*geminiCandidate    `json:"candidates,omitempty"`
	PromptFeedback *geminiPromptFeedback `json:"promptFeedback,omitempty"`
	UsageMetadata  *geminiUsageMetadata  `json:"usageMetadata,omitempty"`
	ModelVersion   string                `json:"modelVersion,omitempty"`
	ResponseID     string                `json:"responseId,omitempty"`
}

// geminiCandidate is a single response candidate
type geminiCandidate struct {
	Content       *geminiContent        `json:"content,omitempty"`
	FinishReason  string                `json:"finishReason,omitempty"`
	SafetyRatings []*geminiSafetyRating `json:"safetyRatings,omitempty"`
	TokenCount    int                   `json:"tokenCount,omitempty"`
	AvgLogprobs   float64               `json:"avgLogprobs,omitempty"`
	Index         int                   `json:"index,omitempty"`
}

// geminiPromptFeedback reports whether the prompt was blocked
type geminiPromptFeedback struct {
	BlockReason   string                `json:"blockReason,omitempty"`
	SafetyRatings []*geminiSafetyRating `json:"safetyRatings,omitempty"`
}

///////////////////////////////////////////////////////////////////////////////
// GENERATION CONFIG

// geminiGenerationConfig holds all generation parameters
type geminiGenerationConfig struct {
	StopSequences      []string              `json:"stopSequences,omitempty"`
	ResponseMIMEType   string                `json:"responseMimeType,omitempty"`
	ResponseSchema     any                   `json:"responseSchema,omitempty"`
	ResponseJSONSchema any                   `json:"responseJsonSchema,omitempty"`
	CandidateCount     int                   `json:"candidateCount,omitempty"`
	MaxOutputTokens    int                   `json:"maxOutputTokens,omitempty"`
	Temperature        *float64              `json:"temperature,omitempty"`
	TopP               *float64              `json:"topP,omitempty"`
	TopK               *int                  `json:"topK,omitempty"`
	Seed               *int                  `json:"seed,omitempty"`
	PresencePenalty    *float64              `json:"presencePenalty,omitempty"`
	FrequencyPenalty   *float64              `json:"frequencyPenalty,omitempty"`
	ResponseLogprobs   bool                  `json:"responseLogprobs,omitempty"`
	Logprobs           int                   `json:"logprobs,omitempty"`
	ThinkingConfig     *geminiThinkingConfig `json:"thinkingConfig,omitempty"`
}

// geminiThinkingConfig controls the model's extended thinking/reasoning
type geminiThinkingConfig struct {
	IncludeThoughts bool   `json:"includeThoughts,omitempty"`
	ThinkingBudget  int    `json:"thinkingBudget,omitempty"`
	ThinkingLevel   string `json:"thinkingLevel,omitempty"` // MINIMAL, LOW, MEDIUM, HIGH
}

///////////////////////////////////////////////////////////////////////////////
// TOOLS & FUNCTION CALLING

// geminiTool is a tool the model may use (function declarations, google search, etc.)
type geminiTool struct {
	FunctionDeclarations []*geminiFunctionDeclaration `json:"functionDeclarations,omitempty"`
	GoogleSearch         *geminiGoogleSearch          `json:"googleSearch,omitempty"`
	CodeExecution        *geminiCodeExecution         `json:"codeExecution,omitempty"`
}

// geminiFunctionDeclaration describes a callable function
type geminiFunctionDeclaration struct {
	Name                 string         `json:"name"`
	Description          string         `json:"description"`
	Parameters           any            `json:"parameters,omitempty"`           // Schema object
	ParametersJSONSchema map[string]any `json:"parametersJsonSchema,omitempty"` // JSON Schema alternative
}

// geminiToolConfig configures tool behaviour
type geminiToolConfig struct {
	FunctionCallingConfig *geminiFunctionCallingConfig `json:"functionCallingConfig,omitempty"`
}

// geminiFunctionCallingConfig controls how the model calls functions
type geminiFunctionCallingConfig struct {
	Mode                 string   `json:"mode,omitempty"` // AUTO, ANY, NONE, VALIDATED
	AllowedFunctionNames []string `json:"allowedFunctionNames,omitempty"`
}

// geminiGoogleSearch enables Google Search grounding
type geminiGoogleSearch struct{}

// geminiCodeExecution enables code execution
type geminiCodeExecution struct{}

///////////////////////////////////////////////////////////////////////////////
// SAFETY

// geminiSafetySetting controls blocking thresholds per harm category
type geminiSafetySetting struct {
	Category  string `json:"category"`
	Threshold string `json:"threshold"`
}

// geminiSafetyRating is the per-category safety rating of content
type geminiSafetyRating struct {
	Category    string `json:"category"`
	Probability string `json:"probability"`
	Blocked     bool   `json:"blocked,omitempty"`
}

///////////////////////////////////////////////////////////////////////////////
// USAGE METADATA

// geminiUsageMetadata reports token counts for a generation request
type geminiUsageMetadata struct {
	PromptTokenCount        int `json:"promptTokenCount,omitempty"`
	CachedContentTokenCount int `json:"cachedContentTokenCount,omitempty"`
	CandidatesTokenCount    int `json:"candidatesTokenCount,omitempty"`
	ThoughtsTokenCount      int `json:"thoughtsTokenCount,omitempty"`
	TotalTokenCount         int `json:"totalTokenCount,omitempty"`
}

///////////////////////////////////////////////////////////////////////////////
// FINISH REASON CONSTANTS

const (
	geminiFinishReasonStop                   = "STOP"
	geminiFinishReasonMaxTokens              = "MAX_TOKENS"
	geminiFinishReasonSafety                 = "SAFETY"
	geminiFinishReasonRecitation             = "RECITATION"
	geminiFinishReasonLanguage               = "LANGUAGE"
	geminiFinishReasonOther                  = "OTHER"
	geminiFinishReasonBlocklist              = "BLOCKLIST"
	geminiFinishReasonProhibitedContent      = "PROHIBITED_CONTENT"
	geminiFinishReasonSPII                   = "SPII"
	geminiFinishReasonMalformedFunctionCall  = "MALFORMED_FUNCTION_CALL"
	geminiFinishReasonImageSafety            = "IMAGE_SAFETY"
	geminiFinishReasonImageProhibitedContent = "IMAGE_PROHIBITED_CONTENT"
	geminiFinishReasonImageRecitation        = "IMAGE_RECITATION"
	geminiFinishReasonUnexpectedToolCall     = "UNEXPECTED_TOOL_CALL"
	geminiFinishReasonMissingThoughtSig      = "MISSING_THOUGHT_SIGNATURE"
)

///////////////////////////////////////////////////////////////////////////////
// MODELS — GET & LIST

// geminiModel is the "Model" resource returned by GET /v1beta/models/{model}
// and in the list response.
type geminiModel struct {
	Name                       string   `json:"name"` // "models/{model}"
	BaseModelID                string   `json:"baseModelId,omitempty"`
	Version                    string   `json:"version,omitempty"`
	DisplayName                string   `json:"displayName,omitempty"`
	Description                string   `json:"description,omitempty"`
	InputTokenLimit            int      `json:"inputTokenLimit,omitempty"`
	OutputTokenLimit           int      `json:"outputTokenLimit,omitempty"`
	SupportedGenerationMethods []string `json:"supportedGenerationMethods,omitempty"`
	Thinking                   bool     `json:"thinking,omitempty"`
	Temperature                float64  `json:"temperature,omitempty"`
	MaxTemperature             float64  `json:"maxTemperature,omitempty"`
	TopP                       float64  `json:"topP,omitempty"`
	TopK                       int      `json:"topK,omitempty"`
}

// geminiListModelsResponse is returned by GET /v1beta/models
type geminiListModelsResponse struct {
	Models        []*geminiModel `json:"models"`
	NextPageToken string         `json:"nextPageToken,omitempty"`
}

///////////////////////////////////////////////////////////////////////////////
// EMBEDDINGS

// geminiEmbedRequest is the request body for
// POST /v1beta/{model=models/*}:embedContent
type geminiEmbedRequest struct {
	Model                string         `json:"model,omitempty"` // required for batch requests
	Content              *geminiContent `json:"content"`
	TaskType             string         `json:"taskType,omitempty"`
	Title                string         `json:"title,omitempty"`
	OutputDimensionality int            `json:"outputDimensionality,omitempty"`
}

// geminiEmbedResponse is the response from embedContent
type geminiEmbedResponse struct {
	Embedding *geminiContentEmbedding `json:"embedding"`
}

// geminiBatchEmbedRequest is the request body for
// POST /v1beta/{model=models/*}:batchEmbedContents
type geminiBatchEmbedRequest struct {
	Requests []*geminiEmbedRequest `json:"requests"`
}

// geminiBatchEmbedResponse is the response from batchEmbedContents
type geminiBatchEmbedResponse struct {
	Embeddings []*geminiContentEmbedding `json:"embeddings"`
}

// geminiContentEmbedding holds an embedding vector
type geminiContentEmbedding struct {
	Values []float64 `json:"values"`
}

///////////////////////////////////////////////////////////////////////////////
// TASK TYPE CONSTANTS (for embeddings)

const (
	geminiTaskTypeUnspecified        = "TASK_TYPE_UNSPECIFIED"
	geminiTaskTypeRetrievalQuery     = "RETRIEVAL_QUERY"
	geminiTaskTypeRetrievalDocument  = "RETRIEVAL_DOCUMENT"
	geminiTaskTypeSemanticSimilarity = "SEMANTIC_SIMILARITY"
	geminiTaskTypeClassification     = "CLASSIFICATION"
	geminiTaskTypeClustering         = "CLUSTERING"
	geminiTaskTypeQuestionAnswering  = "QUESTION_ANSWERING"
	geminiTaskTypeFactVerification   = "FACT_VERIFICATION"
	geminiTaskTypeCodeRetrievalQuery = "CODE_RETRIEVAL_QUERY"
)

///////////////////////////////////////////////////////////////////////////////
// STREAMING SUPPORT
//
// The streamGenerateContent endpoint returns newline-delimited JSON objects,
// each being a geminiGenerateResponse. go-client's OptTextStreamCallback
// delivers each line as text; we unmarshal into geminiGenerateResponse.

// geminiStreamChunk is a convenience alias — each chunk in the SSE stream
// is a complete geminiGenerateResponse.
type geminiStreamChunk = geminiGenerateResponse

///////////////////////////////////////////////////////////////////////////////
// HARM CATEGORY & BLOCK THRESHOLD CONSTANTS

const (
	geminiHarmCategoryHateSpeech       = "HARM_CATEGORY_HATE_SPEECH"
	geminiHarmCategorySexuallyExplicit = "HARM_CATEGORY_SEXUALLY_EXPLICIT"
	geminiHarmCategoryDangerousContent = "HARM_CATEGORY_DANGEROUS_CONTENT"
	geminiHarmCategoryHarassment       = "HARM_CATEGORY_HARASSMENT"
	geminiHarmCategoryCivicIntegrity   = "HARM_CATEGORY_CIVIC_INTEGRITY"
)

const (
	geminiBlockThresholdNone           = "BLOCK_NONE"
	geminiBlockThresholdOnlyHigh       = "BLOCK_ONLY_HIGH"
	geminiBlockThresholdMediumAndAbove = "BLOCK_MEDIUM_AND_ABOVE"
	geminiBlockThresholdLowAndAbove    = "BLOCK_LOW_AND_ABOVE"
	geminiBlockThresholdOff            = "OFF"
)

///////////////////////////////////////////////////////////////////////////////
// FUNCTION CALLING MODE CONSTANTS

const (
	geminiFunctionCallingModeAuto      = "AUTO"
	geminiFunctionCallingModeAny       = "ANY"
	geminiFunctionCallingModeNone      = "NONE"
	geminiFunctionCallingModeValidated = "VALIDATED"
)

///////////////////////////////////////////////////////////////////////////////
// ERROR RESPONSE

// geminiErrorResponse is the error body returned by the Gemini REST API
type geminiErrorResponse struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

///////////////////////////////////////////////////////////////////////////////
// HELPERS

// geminiNewTextContent creates a Content with a single text Part
func geminiNewTextContent(role, text string) *geminiContent {
	return &geminiContent{
		Role: role,
		Parts: []*geminiPart{
			{Text: text},
		},
	}
}

// geminiNewFunctionCallPart creates a Part for a function call
func geminiNewFunctionCallPart(name string, args map[string]any) *geminiPart {
	return &geminiPart{
		FunctionCall: &geminiFunctionCall{
			Name: name,
			Args: args,
		},
	}
}

// geminiNewFunctionResponsePart creates a Part for a function response
func geminiNewFunctionResponsePart(name string, response map[string]any) *geminiPart {
	return &geminiPart{
		FunctionResponse: &geminiFunctionResult{
			Name:     name,
			Response: response,
		},
	}
}

// geminiNewInlineDataPart creates a Part for inline binary data
func geminiNewInlineDataPart(mimeType, base64Data string) *geminiPart {
	return &geminiPart{
		InlineData: &geminiBlob{
			MIMEType: mimeType,
			Data:     base64Data,
		},
	}
}

// geminiNewFileDataPart creates a Part referencing a file by URI
func geminiNewFileDataPart(mimeType, fileURI string) *geminiPart {
	return &geminiPart{
		FileData: &geminiFileData{
			MIMEType: mimeType,
			FileURI:  fileURI,
		},
	}
}

// Ensure json import is used
var _ = json.Marshal
