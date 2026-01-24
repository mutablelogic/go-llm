package gemini

import (
	"encoding/json"
	"fmt"

	opt "github.com/mutablelogic/go-llm/pkg/opt"
	schema "github.com/mutablelogic/go-llm/pkg/schema"
)

////////////////////////////////////////////////////////////////////////////////
// GENERATE CONTENT OPTIONS

// WithSystemPrompt sets the system instruction for the request
func WithSystemPrompt(value string) opt.Opt {
	return opt.SetString("system", value)
}

// WithTemperature sets the temperature for the request (0.0 to 2.0)
func WithTemperature(value float64) opt.Opt {
	if value < 0 || value > 2 {
		return opt.Error(fmt.Errorf("temperature must be between 0.0 and 2.0"))
	}
	return opt.SetFloat64("temperature", value)
}

// WithMaxTokens sets the maximum number of tokens to generate (minimum 1)
func WithMaxTokens(value uint) opt.Opt {
	if value < 1 {
		return opt.Error(fmt.Errorf("max_tokens must be at least 1"))
	}
	return opt.SetUint("max_tokens", value)
}

// WithTopK sets the top K sampling parameter (minimum 1)
func WithTopK(value uint) opt.Opt {
	if value < 1 {
		return opt.Error(fmt.Errorf("top_k must be at least 1"))
	}
	return opt.SetUint("top_k", value)
}

// WithTopP sets the nucleus sampling parameter (0.0 to 1.0)
func WithTopP(value float64) opt.Opt {
	if value < 0 || value > 1 {
		return opt.Error(fmt.Errorf("top_p must be between 0.0 and 1.0"))
	}
	return opt.SetFloat64("top_p", value)
}

// WithStopSequences sets custom stop sequences for the request
func WithStopSequences(values ...string) opt.Opt {
	if len(values) == 0 {
		return opt.Error(fmt.Errorf("at least one stop sequence is required"))
	}
	return opt.AddString("stop_sequences", values...)
}

// WithTool adds a tool definition to the request (Gemini function-style tools).
// Multiple calls append additional tools.
func WithTool(def schema.ToolDefinition) opt.Opt {
	if def.Name == "" {
		return opt.Error(fmt.Errorf("tool name is required"))
	}
	if def.InputSchema == nil {
		return opt.Error(fmt.Errorf("tool schema is required"))
	}

	data, err := json.Marshal(def)
	if err != nil {
		return opt.Error(fmt.Errorf("failed to serialize tool: %w", err))
	}
	return opt.AddString("tools", string(data))
}

////////////////////////////////////////////////////////////////////////////////
// EMBEDDING OPTIONS

// TaskType represents the type of task for which embeddings will be used
type TaskType string

const (
	TaskTypeUnspecified       TaskType = "TASK_TYPE_UNSPECIFIED"
	TaskTypeRetrievalQuery    TaskType = "RETRIEVAL_QUERY"
	TaskTypeRetrievalDocument TaskType = "RETRIEVAL_DOCUMENT"
	TaskTypeSemantic          TaskType = "SEMANTIC_SIMILARITY"
	TaskTypeClassification    TaskType = "CLASSIFICATION"
	TaskTypeClustering        TaskType = "CLUSTERING"
	TaskTypeQuestionAnswering TaskType = "QUESTION_ANSWERING"
	TaskTypeFactVerification  TaskType = "FACT_VERIFICATION"
	TaskTypeCodeRetrieval     TaskType = "CODE_RETRIEVAL_QUERY"
)

// WithTaskType sets the task type for embedding generation
func WithTaskType(value TaskType) opt.Opt {
	return opt.SetString("task_type", string(value))
}

// WithOutputDimensionality sets the output dimensionality for embeddings
// (only supported on newer models, not embedding-001)
func WithOutputDimensionality(value uint) opt.Opt {
	if value < 1 {
		return opt.Error(fmt.Errorf("output_dimensionality must be at least 1"))
	}
	return opt.SetUint("output_dimensionality", value)
}

// WithTitle sets the title for the document (only for RETRIEVAL_DOCUMENT task type)
func WithTitle(value string) opt.Opt {
	return opt.SetString("title", value)
}
