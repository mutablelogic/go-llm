package toolkit_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"uuid"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	eventschema "github.com/mutablelogic/go-llm/broadcaster/schema"
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	toolschema "github.com/mutablelogic/go-llm/toolkit/schema"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func Test_SystemPrompts(t *testing.T) {
	ctx := context.Background()
	tk, err := toolkit.New(ctx)
	require.NoError(t, err)

	// Namespaces with prompts, one of which is the same as another's, and
	// namespaces without one
	require.NoError(t, tk.Add(toolkit.NewNamespace("weather", "  Use metric units.  ", tool{name: "current"}, tool{name: "forecast"})))
	require.NoError(t, tk.Add(toolkit.NewNamespace("news", "Cite the source.", tool{name: "headlines"})))
	require.NoError(t, tk.Add(toolkit.NewNamespace("climate", "Use metric units.", tool{name: "history"})))
	require.NoError(t, tk.Add(toolkit.NewNamespace("plain", "", tool{name: "echo"})))
	require.NoError(t, tk.Add(toolkit.NewNamespace("blank", " ", tool{name: "blank"})))

	for name, test := range map[string]struct {
		names   []string
		prompts []string
	}{
		"None":          {nil, nil},
		"Tool":          {[]string{"weather:current"}, []string{"Use metric units."}},
		"OncePerSpace":  {[]string{"weather:current", "weather:forecast"}, []string{"Use metric units."}},
		"Namespace":     {[]string{"news:*"}, []string{"Cite the source."}},
		"SamePrompt":    {[]string{"weather:*", "climate:history"}, []string{"Use metric units."}},
		"NoPrompt":      {[]string{"plain:echo", "blank:blank"}, nil},
		"WithoutPrefix": {[]string{"headlines"}, []string{"Cite the source."}},
		"Order":         {[]string{"news:headlines", "weather:current"}, []string{"Cite the source.", "Use metric units."}},
		"All":           {[]string{"*"}, []string{"Use metric units.", "Cite the source."}},
	} {
		t.Run(name, func(t *testing.T) {
			prompts, err := tk.SystemPrompts(ctx, test.names...)
			require.NoError(t, err)
			assert.Equal(t, test.prompts, prompts)
		})
	}

	t.Run("Errors", func(t *testing.T) {
		// The same errors as Tools
		_, err := tk.SystemPrompts(ctx, "weather:missing")
		assert.ErrorIs(t, err, llm.ErrNotFound)
		_, err = tk.SystemPrompts(ctx, "weather:current", "current")
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})
}

func Test_SystemPromptsWithoutTools(t *testing.T) {
	// A namespace without tools has its system prompt, when it's named by its
	// name and "*", or by "*", but not when only other tools are named
	ctx := context.Background()
	tk, err := toolkit.New(ctx)
	require.NoError(t, err)
	require.NoError(t, tk.Add(toolkit.NewNamespace("clock", "It's noon.")))
	require.NoError(t, tk.Add(toolkit.NewNamespace("weather", "Use metric units.", tool{name: "current"})))

	for name, test := range map[string]struct {
		names   []string
		prompts []string
	}{
		"Namespace":  {[]string{"clock:*"}, []string{"It's noon."}},
		"All":        {[]string{"*"}, []string{"It's noon.", "Use metric units."}},
		"OtherTools": {[]string{"weather:current"}, []string{"Use metric units."}},
		"NameOrder":  {[]string{"weather:current", "clock:*"}, []string{"Use metric units.", "It's noon."}},
	} {
		t.Run(name, func(t *testing.T) {
			prompts, err := tk.SystemPrompts(ctx, test.names...)
			require.NoError(t, err)
			assert.Equal(t, test.prompts, prompts)
		})
	}

	// A namespace which doesn't exist is an error, as it is for Tools
	_, err = tk.SystemPrompts(ctx, "missing:*")
	assert.ErrorIs(t, err, llm.ErrNotFound)
}

func Test_AddNamespace(t *testing.T) {
	// A namespace is needed, whose name is an identifier
	ctx := context.Background()
	tk, err := toolkit.New(ctx)
	require.NoError(t, err)
	assert.ErrorIs(t, tk.Add(nil), llm.ErrBadParameter)
	assert.ErrorIs(t, tk.Add(toolkit.NewNamespace("not valid", "prompt", tool{name: "a"})), llm.ErrBadParameter)

	ns := toolkit.NewNamespace("weather", "Use metric units.")
	assert.Equal(t, "weather", ns.Name())
	assert.Equal(t, "Use metric units.", ns.SystemPrompt())
}

// countingNamespace is a namespace whose system prompt changes each time
// it's read, as one which has the time can, and counts how many times
type countingNamespace struct {
	name  string
	tools []llm.Tool
	reads *atomic.Int32
}

func (n countingNamespace) Name() string      { return n.name }
func (n countingNamespace) Tools() []llm.Tool { return n.tools }
func (n countingNamespace) SystemPrompt() string {
	return fmt.Sprint("Read ", n.reads.Add(1), " times.")
}

// Notify does nothing, as the namespace's tools don't change
func (countingNamespace) Description() string { return "" }

// Type returns that the namespace's tools are built in
func (countingNamespace) Type() string { return toolschema.NamespaceBuiltin }

func (countingNamespace) Notify(func(eventschema.EventAction, string)) {}

func Test_SystemPromptsReadOnce(t *testing.T) {
	// A namespace's system prompt is read once, however many names select
	// it, so a prompt which changes is in the system prompt once
	ctx := context.Background()
	for name, test := range map[string]struct {
		tools []llm.Tool
		names []string
	}{
		"Tools":        {[]llm.Tool{tool{name: "now"}, tool{name: "alarm"}}, []string{"clock:now", "clock:alarm"}},
		"WithoutTools": {nil, []string{"clock:*", "*"}},
	} {
		t.Run(name, func(t *testing.T) {
			tk, err := toolkit.New(ctx)
			require.NoError(t, err)
			var reads atomic.Int32
			require.NoError(t, tk.Add(countingNamespace{"clock", test.tools, &reads}))
			prompts, err := tk.SystemPrompts(ctx, test.names...)
			require.NoError(t, err)
			assert.Equal(t, []string{"Read 1 times."}, prompts)
			assert.Equal(t, int32(1), reads.Load())
		})
	}
}

// sessional is a namespace which says more of the session, which it was
// asked about
type sessional struct {
	toolkit.Namespace
	asked []uuid.UUID
}

func (s *sessional) SessionPrompt(_ context.Context, session uuid.UUID) string {
	s.asked = append(s.asked, session)
	if session.String() == connected.String() {
		return "  This conversation is connected.  "
	}
	return ""
}

var connected = uuid.New()

func Test_SessionPrompt(t *testing.T) {
	ctx := context.Background()
	tk, err := toolkit.New(ctx)
	require.NoError(t, err)
	ns := &sessional{Namespace: toolkit.NewNamespace("messaging", "Chats can be connected.", tool{name: "send"})}
	require.NoError(t, tk.Add(ns))

	// Without a session, the namespace's prompt, as it is
	prompts, err := tk.SystemPrompts(ctx, "messaging:*")
	require.NoError(t, err)
	assert.Equal(t, []string{"Chats can be connected."}, prompts)
	assert.Empty(t, ns.asked)

	// With one, what it says of the session is added, or not, when it says
	// nothing
	prompts, err = tk.SystemPrompts(toolkit.WithPromptSession(ctx, connected), "messaging:send")
	require.NoError(t, err)
	assert.Equal(t, []string{"Chats can be connected.\n\nThis conversation is connected."}, prompts)
	prompts, err = tk.SystemPrompts(toolkit.WithPromptSession(ctx, uuid.New()), "*")
	require.NoError(t, err)
	assert.Equal(t, []string{"Chats can be connected."}, prompts)
	assert.Len(t, ns.asked, 2)
}
