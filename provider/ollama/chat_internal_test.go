package ollama

import (
	"testing"

	// Packages
	schema "github.com/mutablelogic/go-llm/provider/schema"
	assert "github.com/stretchr/testify/assert"
)

// A reply which was truncated is so even when it called tools, whose calls
// may be incomplete
func Test_ChatResult(t *testing.T) {
	calls := []chatToolCall{{}}
	for name, test := range map[string]struct {
		response chatResponse
		result   schema.ResultType
	}{
		"Stop":            {chatResponse{DoneReason: "stop"}, schema.ResultStop},
		"ToolCalls":       {chatResponse{DoneReason: "stop", Message: chatMessage{ToolCalls: calls}}, schema.ResultToolCall},
		"Length":          {chatResponse{DoneReason: "length"}, schema.ResultMaxTokens},
		"LengthToolCalls": {chatResponse{DoneReason: "length", Message: chatMessage{ToolCalls: calls}}, schema.ResultMaxTokens},
		"Other":           {chatResponse{}, schema.ResultOther},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, test.result, chatResult(test.response))
		})
	}
}
