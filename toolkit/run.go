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
// timeout, unless it's zero, and returns its result, or its error, or that
// it timed out, or the context's error, when the context is done, without
// waiting for the tool to return
func runTool(ctx context.Context, tool llm.Tool, call llm.ToolCall, timeout time.Duration, session func(llm.ToolCall) Session) providerschema.ContentBlock {
	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
	} else {
		ctx, cancel = context.WithCancel(ctx)
	}
	defer cancel()
	if session != nil {
		ctx = WithSession(ctx, session(call))
	}

	// The tool returns, unless its context is done first
	done := make(chan outcome, 1)
	go func() {
		result, err := tool.Run(ctx, call.Input)
		done <- outcome{result, err}
	}()
	select {
	case outcome := <-done:
		if outcome.err != nil {
			return providerschema.NewToolError(call.ID, call.Name, outcome.err)
		}
		return providerschema.NewToolResult(call.ID, call.Name, outcome.result)
	case <-ctx.Done():
		if errors.Is(context.Cause(ctx), context.DeadlineExceeded) {
			return providerschema.NewToolError(call.ID, call.Name, fmt.Errorf("timed out after %v", timeout))
		}
		return providerschema.NewToolError(call.ID, call.Name, context.Cause(ctx))
	}
}
