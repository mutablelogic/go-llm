package lmstudio

import (
	"encoding/json"
	"time"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// apiModel is a model, as LM Studio's REST API lists it, with all its fields
type apiModel struct {
	Type             string           `json:"type"`
	Key              string           `json:"key"`
	DisplayName      string           `json:"display_name,omitempty"`
	Description      *string          `json:"description,omitempty"`
	MaxContextLength uint             `json:"max_context_length,omitempty"`
	LoadedInstances  []loadedInstance `json:"loaded_instances,omitempty"`
	Capabilities     *capabilities    `json:"capabilities,omitempty"`

	// All the fields of the model, including those above
	Fields map[string]json.RawMessage `json:"-"`
}

// loadedInstance is a model which is loaded, in memory, with the context
// length it was loaded with
type loadedInstance struct {
	ID     string `json:"id"`
	Config struct {
		ContextLength uint `json:"context_length,omitempty"`
	} `json:"config"`
}

// capabilities is what a model can do, as LM Studio reports it
type capabilities struct {
	Vision    bool            `json:"vision"`
	ToolUse   bool            `json:"trained_for_tool_use"`
	Reasoning json.RawMessage `json:"reasoning,omitempty"`
}

// listModelsResponse is the response of listing the models
type listModelsResponse struct {
	Models []apiModel `json:"models"`
}

// loadResponse is the response of loading a model
type loadResponse struct {
	Type       string  `json:"type"`
	InstanceID string  `json:"instance_id"`
	Seconds    float64 `json:"load_time_seconds"`
	Status     string  `json:"status"`
}

// downloadStatus is how a model's download is getting on, as it's started,
// and as its status is asked for
type downloadStatus struct {
	JobID      string    `json:"job_id,omitempty"`
	Status     string    `json:"status"`
	Total      uint64    `json:"total_size_bytes,omitempty"`
	Downloaded uint64    `json:"downloaded_bytes,omitempty"`
	Started    time.Time `json:"started_at,omitzero"`
	Completed  time.Time `json:"completed_at,omitzero"`
}

// The statuses of a download
const (
	downloadDownloading = "downloading"
	downloadCompleted   = "completed"
	downloadFailed      = "failed"
	downloadDone        = "already_downloaded"
)

// The types of models
const (
	typeLLM       = "llm"
	typeEmbedding = "embedding"
)

///////////////////////////////////////////////////////////////////////////////
// JSON

// UnmarshalJSON reads the model's fields, and keeps them all
func (m *apiModel) UnmarshalJSON(data []byte) error {
	type model apiModel
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, (*model)(m)); err != nil {
		return err
	} else if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	m.Fields = fields
	return nil
}
