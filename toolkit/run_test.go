package toolkit_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

// barrier is a tool which waits for the other tools of the barrier to run,
// so returns an error unless they run in parallel
type barrier struct {
	echoTool
	arrived *sync.WaitGroup
}

func (t barrier) Run(context.Context, json.RawMessage) (any, error) {
	t.arrived.Done()
	waited := make(chan struct{})
	go func() { t.arrived.Wait(); close(waited) }()
	select {
	case <-waited:
		return t.name, nil
	case <-time.After(5 * time.Second):
		return nil, errors.New("the tools didn't run in parallel")
	}
}

// sessionTool returns the name its session's logger logs as
type sessionTool struct{ echoTool }

func (t sessionTool) Run(ctx context.Context, _ json.RawMessage) (any, error) {
	toolkit.SessionFromContext(ctx).Logger().Info("hello")
	return nil, nil
}

func Test_RunTools(t *testing.T) {
	ctx := context.Background()
	calls := func(names ...string) []llm.ToolCall {
		var result []llm.ToolCall
		for _, name := range names {
			result = append(result, llm.ToolCall{ID: "call-" + name, Name: name, Input: json.RawMessage(`{"name":"` + name + `"}`)})
		}
		return result
	}

	t.Run("Results", func(t *testing.T) {
		// Each call has its result, in the order of the calls, which is the
		// error of a tool which fails, or isn't one of the tools
		tools := []llm.Tool{echoTool{name: "a"}, echoTool{name: "b", err: errors.New("it failed")}}
		results, err := toolkit.RunTools(ctx, tools, calls("b", "missing", "a"), 0, nil)
		require.NoError(t, err)
		require.Len(t, results, 3)
		for i, name := range []string{"b", "missing", "a"} {
			require.NotNil(t, results[i].ToolResult)
			assert.Equal(t, "call-"+name, results[i].ToolResult.ID)
			assert.Equal(t, name, results[i].ToolResult.Name)
		}
		assert.True(t, results[0].ToolResult.IsError)
		assert.Contains(t, string(results[0].ToolResult.Content), "it failed")
		assert.True(t, results[1].ToolResult.IsError)
		assert.Contains(t, string(results[1].ToolResult.Content), "missing")
		assert.False(t, results[2].ToolResult.IsError)
		assert.JSONEq(t, `{"name":"a"}`, string(results[2].ToolResult.Content))
	})

	t.Run("None", func(t *testing.T) {
		results, err := toolkit.RunTools(ctx, nil, nil, 0, nil)
		require.NoError(t, err)
		assert.Empty(t, results)
	})

	t.Run("Parallel", func(t *testing.T) {
		// The tools run in parallel
		var arrived sync.WaitGroup
		arrived.Add(3)
		tools := []llm.Tool{barrier{echoTool{name: "a"}, &arrived}, barrier{echoTool{name: "b"}, &arrived}, barrier{echoTool{name: "c"}, &arrived}}
		results, err := toolkit.RunTools(ctx, tools, calls("c", "a", "b"), 0, nil)
		require.NoError(t, err)
		require.Len(t, results, 3)
		for i, name := range []string{"c", "a", "b"} {
			assert.False(t, results[i].ToolResult.IsError, string(results[i].ToolResult.Content))
			assert.JSONEq(t, `"`+name+`"`, string(results[i].ToolResult.Content))
		}
	})

	t.Run("Session", func(t *testing.T) {
		// Each tool runs in the session of its call
		var mu sync.Mutex
		var lines []string
		tools := []llm.Tool{sessionTool{echoTool{name: "a"}}, sessionTool{echoTool{name: "b"}}}
		_, err := toolkit.RunTools(ctx, tools, calls("a", "b"), 0, func(call llm.ToolCall) toolkit.Session {
			return toolkit.NewSession(call.ID, func(line string) {
				mu.Lock()
				defer mu.Unlock()
				lines = append(lines, call.ID+": "+line)
			}, nil)
		})
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{"call-a: hello", "call-b: hello"}, lines)
	})

	t.Run("Cancelled", func(t *testing.T) {
		// The context's error is returned when it's done
		ctx, cancel := context.WithCancel(ctx)
		cancel()
		_, err := toolkit.RunTools(ctx, []llm.Tool{echoTool{name: "a"}}, calls("a"), 0, nil)
		assert.ErrorIs(t, err, context.Canceled)
	})
}

// hangTool is a tool which doesn't return until it's released, or, unless
// it ignores its context, its context is done
type hangTool struct {
	echoTool
	release chan struct{}
	ignore  bool
}

func (t hangTool) Run(ctx context.Context, _ json.RawMessage) (any, error) {
	if t.ignore {
		<-t.release
		return nil, nil
	}
	select {
	case <-t.release:
		return nil, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func Test_RunToolsTimeout(t *testing.T) {
	ctx := context.Background()
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	calls := []llm.ToolCall{{ID: "1", Name: "hang"}, {ID: "2", Name: "a"}}

	for name, ignore := range map[string]bool{"Context": false, "IgnoresContext": true} {
		t.Run(name, func(t *testing.T) {
			// A tool which doesn't return in time has its result that it timed
			// out, even when it ignores its context, and the others' results
			// are theirs
			tools := []llm.Tool{hangTool{echoTool{name: "hang"}, release, ignore}, echoTool{name: "a"}}
			start := time.Now()
			results, err := toolkit.RunTools(ctx, tools, calls, 50*time.Millisecond, nil)
			require.NoError(t, err)
			assert.Less(t, time.Since(start), 2*time.Second)
			require.Len(t, results, 2)
			assert.True(t, results[0].ToolResult.IsError)
			assert.Contains(t, string(results[0].ToolResult.Content), "timed out after 50ms")
			assert.False(t, results[1].ToolResult.IsError)
		})
	}

	t.Run("ReturnsLate", func(t *testing.T) {
		// A tool which returns a result, once its time is up, timed out, every
		// time, whichever the runner sees first
		for range 50 {
			results, err := toolkit.RunTools(ctx, []llm.Tool{lateTool{echoTool{name: "late"}}}, []llm.ToolCall{{ID: "1", Name: "late"}}, 5*time.Millisecond, nil)
			require.NoError(t, err)
			require.Len(t, results, 1)
			assert.Contains(t, string(results[0].ToolResult.Content), "timed out after 5ms")
		}
	})

	t.Run("ReturnsDeadline", func(t *testing.T) {
		// A tool which returns its context's error, once its time is up,
		// timed out, every time, whichever the runner sees first
		for range 50 {
			results, err := toolkit.RunTools(ctx, []llm.Tool{deadlineTool{echoTool{name: "deadline"}}}, []llm.ToolCall{{ID: "1", Name: "deadline"}}, 5*time.Millisecond, nil)
			require.NoError(t, err)
			require.Len(t, results, 1)
			assert.Contains(t, string(results[0].ToolResult.Content), "timed out after 5ms")
		}
	})

	t.Run("Cancelled", func(t *testing.T) {
		// The context's error is returned when it's done, without waiting
		// for a tool which ignores it
		ctx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()
		start := time.Now()
		_, err := toolkit.RunTools(ctx, []llm.Tool{hangTool{echoTool{name: "hang"}, release, true}}, calls[:1], 0, nil)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Less(t, time.Since(start), 2*time.Second)
	})
}

// deadlineTool is a tool which returns its context's error, when it's done
type deadlineTool struct{ echoTool }

func (deadlineTool) Run(ctx context.Context, _ json.RawMessage) (any, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

// lateTool is a tool which returns a result, when its context is done
type lateTool struct{ echoTool }

func (lateTool) Run(ctx context.Context, _ json.RawMessage) (any, error) {
	<-ctx.Done()
	return "partial", nil
}

// countTool is a tool which counts how many times it's run
type countTool struct {
	echoTool
	runs *atomic.Int32
}

func (t countTool) Run(context.Context, json.RawMessage) (any, error) {
	t.runs.Add(1)
	return nil, nil
}

// resourceTool is a tool whose result is a resource, which isn't run by the
// toolkit, so its result isn't read by it
type resourceTool struct{ echoTool }

func (resourceTool) Run(context.Context, json.RawMessage) (any, error) {
	return resource{}, nil
}

func Test_RunToolsNotRunAndResources(t *testing.T) {
	ctx := context.Background()

	t.Run("NotRun", func(t *testing.T) {
		// A tool isn't run when the context is done already
		ctx, cancel := context.WithCancel(ctx)
		cancel()
		var runs atomic.Int32
		_, err := toolkit.RunTools(ctx, []llm.Tool{countTool{echoTool{name: "count"}, &runs}}, []llm.ToolCall{{ID: "1", Name: "count"}, {ID: "2", Name: "count"}}, 0, nil)
		assert.ErrorIs(t, err, context.Canceled)
		assert.Zero(t, runs.Load())
	})

	t.Run("Resource", func(t *testing.T) {
		// A resource which a tool returns is read, with its type and data
		results, err := toolkit.RunTools(ctx, []llm.Tool{resourceTool{echoTool{name: "file"}}}, []llm.ToolCall{{ID: "1", Name: "file"}}, 0, nil)
		require.NoError(t, err)
		require.Len(t, results, 1)
		require.False(t, results[0].ToolResult.IsError, string(results[0].ToolResult.Content))
		var got struct {
			Type string `json:"type"`
			Data []byte `json:"data"`
		}
		require.NoError(t, json.Unmarshal(results[0].ToolResult.Content, &got))
		assert.Equal(t, "image/png", got.Type)
		assert.Equal(t, []byte("png"), got.Data)
	})

	t.Run("Unreadable", func(t *testing.T) {
		// A resource which can't be read is the tool's error
		results, err := toolkit.RunTools(ctx, []llm.Tool{unreadableTool{echoTool{name: "file"}}}, []llm.ToolCall{{ID: "1", Name: "file"}}, 0, nil)
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.True(t, results[0].ToolResult.IsError)
		assert.Contains(t, string(results[0].ToolResult.Content), "can't read")
	})
}

// unreadableTool is a tool whose result is a resource which can't be read
type unreadableTool struct{ echoTool }

func (unreadableTool) Run(context.Context, json.RawMessage) (any, error) {
	return resource{err: errors.New("can't read")}, nil
}
