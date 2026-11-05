package memory

import (
	"context"
	"encoding/json"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	schema "github.com/mutablelogic/go-llm/memory/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	jsonschema "github.com/mutablelogic/go-server/pkg/jsonschema"
	types "github.com/mutablelogic/go-server/pkg/types"
)

///////////////////////////////////////////////////////////////////////////////
// TYPES

// memories is the tool which remembers, recalls, searches and forgets
// memories, as its operation says
type memories struct {
	toolkit.Base
	memory *Memory
}

var _ llm.Tool = (*memories)(nil)

// memoriesInput is the input of the memories tool: its operation, and what
// the operation needs
type memoriesInput struct {
	Operation   string `json:"operation" enum:"remember,recall,search,forget" help:"remember a memory, under its key, with its description and body, recall the memory with the key, search the memories for the words, or forget the memory with the key"`
	Key         string `json:"key,omitempty" help:"Key of the memory, which is an identifier, such as kitchen_lights, to remember, recall or forget" optional:""`
	Description string `json:"description,omitempty" help:"A line which describes what's remembered, to remember" optional:""`
	Body        string `json:"body,omitempty" help:"What's remembered, as Markdown, to remember, which replaces all of what was remembered under the key" optional:""`
	Version     string `json:"version,omitempty" help:"Version of the memory, which recall, or search, returns, to remember a change to it, or none, to remember a new memory" optional:""`
	Line        string `json:"line,omitempty" help:"A line of the memory's body, to forget only the line, rather than the whole memory" optional:""`
	Q           string `json:"q,omitempty" help:"Words which each memory has, to search, or none, for the most recent memories" optional:""`
	Limit       uint64 `json:"limit,omitempty" help:"Most memories to search for, which is ten when it isn't given" min:"1" max:"100" optional:""`
}

// memoryOutput is a memory, as a model is told it
type memoryOutput struct {
	Key         string    `json:"key"`
	Description string    `json:"description"`
	Body        string    `json:"body"`
	Version     string    `json:"version"`
	Updated     time.Time `json:"updated"`
}

// searchOutput is the memories which are found, and how many match
type searchOutput struct {
	Count    uint           `json:"count"`
	Memories []memoryOutput `json:"memories"`
}

// rememberOutput is the memory which is remembered, and whether it's new
type rememberOutput struct {
	Key     string `json:"key"`
	Version string `json:"version"`
	Created bool   `json:"created"`
}

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// searchLimit is how many memories are found, unless the model says
const searchLimit = 10

///////////////////////////////////////////////////////////////////////////////
// PUBLIC METHODS

func (*memories) Name() string {
	return "memories"
}

func (*memories) Description() string {
	return "What's remembered, from earlier conversations, which every conversation shares: search for memories with words, recall all of a memory, with its version, remember something worth keeping, under a key, which replaces all of what was under it, at its version, or forget a memory, or a line of it."
}

func (*memories) InputSchema() *jsonschema.Schema {
	return jsonschema.MustFor[memoriesInput]()
}

// Run does the operation, saying what it's doing
func (t *memories) Run(ctx context.Context, input json.RawMessage) (any, error) {
	var req memoriesInput
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, llm.ErrBadParameter.With(err.Error())
	}
	switch req.Operation {
	case "remember":
		memory, created, err := t.memory.Remember(ctx, schema.MemoryMeta{Key: req.Key, Description: req.Description, Body: req.Body}, req.Version)
		if err != nil {
			return nil, err
		}
		toolkit.Logf(ctx, "Remembering %s", memory.Description)
		return rememberOutput{Key: memory.Key, Version: memory.Version, Created: created}, nil
	case "recall":
		memory, err := t.memory.GetMemory(ctx, req.Key)
		if err != nil {
			return nil, err
		}
		toolkit.Logf(ctx, "Recalling %s", memory.Description)
		return output(memory), nil
	case "search":
		return t.search(ctx, req)
	case "forget":
		if req.Line != "" {
			memory, err := t.memory.ForgetLine(ctx, req.Key, req.Line)
			if err != nil {
				return nil, err
			}
			toolkit.Logf(ctx, "Forgetting part of %s", memory.Description)
			return output(memory), nil
		}
		memory, err := t.memory.Forget(ctx, req.Key)
		if err != nil {
			return nil, err
		}
		toolkit.Logf(ctx, "Forgetting %s", memory.Description)
		return output(memory), nil
	}
	return nil, llm.ErrBadParameter.Withf("unknown operation %q, which isn't remember, recall, search or forget", req.Operation)
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE METHODS

// search returns the memories with the words, or the most recent, at most
// the limit of them, and how many match
func (t *memories) search(ctx context.Context, req memoriesInput) (searchOutput, error) {
	limit := req.Limit
	if limit == 0 {
		limit = searchLimit
	}
	if req.Q == "" {
		toolkit.Logf(ctx, "Looking through what I remember")
	} else {
		toolkit.Logf(ctx, "Searching what I remember for %q", req.Q)
	}
	list, err := t.memory.ListMemories(ctx, schema.MemoryListRequest{Q: req.Q, Limit: types.Ptr(limit)})
	if err != nil {
		return searchOutput{}, err
	}
	result := searchOutput{Count: list.Count, Memories: make([]memoryOutput, 0, len(list.Body))}
	for _, memory := range list.Body {
		result.Memories = append(result.Memories, output(memory))
	}
	return result, nil
}

///////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// output returns the memory, as a model is told it
func output(memory schema.Memory) memoryOutput {
	result := memoryOutput{Key: memory.Key, Description: memory.Description, Body: memory.Body, Version: memory.Version, Updated: memory.Created}
	if memory.Modified.After(result.Updated) {
		result.Updated = memory.Modified
	}
	return result
}
