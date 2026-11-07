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
// tool has its own timeout to return, or else the timeout, unless it's
// zero, after which its context is done, and its result is that it timed
// out, without waiting for it to return, so a tool which doesn't return
// when its context is done can't keep the others waiting. Returns the context's error, when it's done,
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
			results[i] = runTool(ctx, tool, call, toolTimeout(tool, timeout), session)
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

// toolTimeout returns the most time the tool has to return: its own, unless
// it's zero, when it's the timeout
func toolTimeout(tool llm.Tool, timeout time.Duration) time.Duration {
	if own := tool.Timeout(); own > 0 {
		return own
	}
	return timeout
}

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
	// The call's session, whichever it is, which counts the tokens the tool
	// says it used
	var s Session = discard{}
	if session != nil {
		if s = session(call); s == nil {
			s = discard{}
		}
	}
	counted := &usageSession{Session: s}
	ctx = WithSession(ctx, counted)

	// Its result has how long it took, and the tokens it used, when it
	// says, even when it fails
	start := time.Now()
	block := runCall(parent, ctx, tool, call, timeout)
	if block.ToolResult != nil {
		usage := counted.usage()
		usage.Duration = time.Since(start)
		block.ToolResult.Usage = &usage
	}
	return block
}

// runCall runs the tool for the call, in the context, and returns its
// result, or its error, unless the context is done first, or it times out
func runCall(parent, ctx context.Context, tool llm.Tool, call llm.ToolCall, timeout time.Duration) providerschema.ContentBlock {
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

// usageSession is the session of a tool call, which counts the tokens the
// tool says it used, for its result, as well as telling its session, so
// they're counted whichever session it is
type usageSession struct {
	Session
	mu   sync.Mutex
	used llm.Usage
}

// Usage counts the tokens the tool used, and tells its session
func (s *usageSession) Usage(usage llm.Usage) {
	s.mu.Lock()
	s.used = s.used.Add(usage)
	s.mu.Unlock()
	s.Session.Usage(usage)
}

// usage returns the tokens the tool said it used
func (s *usageSession) usage() llm.Usage {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.used
}
