package toolkit

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	providerschema "github.com/mutablelogic/go-llm/provider/schema"
)

////////////////////////////////////////////////////////////////////////////////
// TYPES

// outcome is what a tool returned
type outcome struct {
	result any
	err    error
}

////////////////////////////////////////////////////////////////////////////////
// PUBLIC FUNCTIONS

// RunTools runs the tools a model called, which are found in the tools by
// the name the model called them by, in parallel, each in the session the
// function returns for its call, unless the function is nil, and returns
// their results, in the order of the calls, as content blocks of a message
// of the results. A call of a tool which isn't one of the tools, or which
// fails, has its error as its result, which the model can correct. Each
// tool has the timeout to return, unless it's zero, after which its context
// is done, and its result is that it timed out, without waiting for it to
// return, so a tool which doesn't return when its context is done can't
// keep the others waiting. Returns the context's error, when it's done,
// without waiting for the tools to return.
func RunTools(ctx context.Context, tools []llm.Tool, calls []llm.ToolCall, timeout time.Duration, session func(llm.ToolCall) Session) ([]providerschema.ContentBlock, error) {
	results := make([]providerschema.ContentBlock, len(calls))
	var wg sync.WaitGroup
	for i, call := range calls {
		index := slices.IndexFunc(tools, func(tool llm.Tool) bool { return tool.Name() == call.Name })
		if index < 0 {
			results[i] = providerschema.NewToolError(call.ID, call.Name, llm.ErrNotFound.Withf("no tool %q", call.Name))
			continue
		}
		tool := tools[index]
		wg.Go(func() {
			results[i] = runTool(ctx, tool, call, timeout, session)
		})
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

////////////////////////////////////////////////////////////////////////////////
// PRIVATE FUNCTIONS

// runTool runs the tool for the call, in the call's session, with the
// timeout, unless it's zero, and returns its result, as toolResult returns
// it, such as with a resource's data, or its error, or that it timed out,
// or the context's error, when the context is done, without waiting for the
// tool to return. A tool isn't run when the context is done already, and a
// tool which returns once its time is up timed out, whatever it returns.
func runTool(parent context.Context, tool llm.Tool, call llm.ToolCall, timeout time.Duration, session func(llm.ToolCall) Session) providerschema.ContentBlock {
	if err := parent.Err(); err != nil {
		return providerschema.NewToolError(call.ID, call.Name, context.Cause(parent))
	}
	var ctx context.Context
	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(parent, timeout)
	} else {
		ctx, cancel = context.WithCancel(parent)
	}
	defer cancel()
	if session != nil {
		ctx = WithSession(ctx, session(call))
	}

	// The tool returns its result, unless its context is done first
	done := make(chan outcome, 1)
	go func() {
		result, err := tool.Run(ctx, call.Input)
		if err == nil {
			result, err = toolResult(ctx, call.Name, result)
		}
		done <- outcome{result, err}
	}()
	select {
	case outcome := <-done:
		if timedOutAt(parent, ctx) {
			return timedOut(call, timeout)
		} else if outcome.err != nil {
			return providerschema.NewToolError(call.ID, call.Name, outcome.err)
		}
		return providerschema.NewToolResult(call.ID, call.Name, outcome.result)
	case <-ctx.Done():
		if timedOutAt(parent, ctx) {
			return timedOut(call, timeout)
		}
		return providerschema.NewToolError(call.ID, call.Name, context.Cause(ctx))
	}
}

// timedOutAt returns true when the tool's context is done, as its time is
// up, rather than as the context it's run in is done
func timedOutAt(parent, ctx context.Context) bool {
	return parent.Err() == nil && errors.Is(context.Cause(ctx), context.DeadlineExceeded)
}

// timedOut returns the result of the call of a tool which timed out
func timedOut(call llm.ToolCall, timeout time.Duration) providerschema.ContentBlock {
	return providerschema.NewToolError(call.ID, call.Name, fmt.Errorf("timed out after %v", timeout))
}
