package registry

import (
	// Packages
	schema "github.com/mutablelogic/go-llm/registry/schema"
)

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// tracedAsk returns the request, as it's traced, without its messages, which
// are the person's
func tracedAsk(req schema.AskRequest) schema.AskRequest {
	req.Messages = nil
	return req
}

// tracedRunAgent returns the request, as it's traced, without the agent's
// input, which can have anything, such as what a person, or a service, sent
func tracedRunAgent(req schema.RunAgentRequest) schema.RunAgentRequest {
	req.Input = nil
	return req
}

// tracedRunEvent returns the request, as it's traced, without the agent's
// input, as tracedRunAgent does
func tracedRunEvent(req schema.RunEventRequest) schema.RunEventRequest {
	req.Input = nil
	return req
}

// tracedEvent returns the request, as it's traced, without its prompt, and
// its data, which are the agent's input
func tracedEvent(req schema.EventRequest) schema.EventRequest {
	req.Prompt, req.Data = "", nil
	return req
}
