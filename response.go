package llm

import (
	"encoding/json"
	"time"
)

//////////////////////////////////////////////////////////////////
// TYPES

type Response struct {
	Agent    string        `json:"agent,omitempty"`    // The agent name
	Model    string        `json:"model,omitempty"`    // The model name
	Context  []Context     `json:"context,omitempty"`  // The context for the response
	Text     string        `json:"text,omitempty"`     // The response text
	Tokens   uint          `json:"tokens,omitempty"`   // The number of tokens
	Duration time.Duration `json:"duration,omitempty"` // The response duration
}

//////////////////////////////////////////////////////////////////
// STRINGIFY

func (r Response) String() string {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(data)
}
