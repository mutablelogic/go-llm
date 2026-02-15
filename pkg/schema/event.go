package schema

///////////////////////////////////////////////////////////////////////////////
// SSE EVENT NAMES

const (
	EventAssistant = "assistant" // Streamed text chunk from the assistant
	EventThinking  = "thinking"  // Streamed thinking/reasoning chunk
	EventTool      = "tool"      // Tool call feedback (name, description)
	EventUsage     = "usage"     // Token usage update
	EventError     = "error"     // Error during processing
	EventResult    = "result"    // Final complete response
)
