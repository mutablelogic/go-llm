package memory_test

import (
	"testing"
	"time"

	// Packages
	toolkit "github.com/mutablelogic/go-llm/toolkit"
	memory "github.com/mutablelogic/go-llm/toolkit/memory"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
)

func TestNamespace(t *testing.T) {
	// The namespace has its name, a system prompt of the time, and no tools,
	// and can be added to a toolkit
	ns := memory.New()
	assert.Equal(t, memory.Name, ns.Name())
	assert.Equal(t, "memory", ns.Name())
	assert.Contains(t, ns.SystemPrompt(), "The current date and time is ")
	assert.Empty(t, ns.Tools())

	tk, err := toolkit.New(t.Context())
	require.NoError(t, err)
	require.NoError(t, tk.Add(ns))
	tools, err := tk.Tools(t.Context(), "memory:*")
	require.NoError(t, err)
	assert.Empty(t, tools)
}

func TestSystemPrompt(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)
	for name, test := range map[string]struct {
		now    time.Time
		prompt string
	}{
		"Named": {
			time.Date(2026, 11, 4, 18, 42, 7, 0, berlin),
			"The current date and time is Wednesday, 4 November 2026, 18:42, in the Europe/Berlin (CET) time zone, UTC+01:00.",
		},
		"Summer": {
			time.Date(2026, 7, 1, 9, 5, 0, 0, berlin),
			"The current date and time is Wednesday, 1 July 2026, 09:05, in the Europe/Berlin (CEST) time zone, UTC+02:00.",
		},
		"Abbreviation": {
			time.Date(2026, 11, 4, 12, 0, 0, 0, time.FixedZone("EST", -5*60*60)),
			"The current date and time is Wednesday, 4 November 2026, 12:00, in the EST time zone, UTC-05:00.",
		},
		"UTC": {
			time.Date(2026, 1, 31, 23, 59, 0, 0, time.UTC),
			"The current date and time is Saturday, 31 January 2026, 23:59, in the UTC time zone, UTC+00:00.",
		},
	} {
		t.Run(name, func(t *testing.T) {
			ns := memory.NewWithClock(func() time.Time { return test.now })
			assert.Equal(t, test.prompt, ns.SystemPrompt())
		})
	}

	t.Run("Now", func(t *testing.T) {
		// The time is the clock's when the system prompt is asked for, so it
		// changes, and a toolkit has it for the namespace's name and "*"
		now := time.Date(2026, 11, 4, 18, 42, 0, 0, time.UTC)
		ns := memory.NewWithClock(func() time.Time { return now })
		tk, err := toolkit.New(t.Context())
		require.NoError(t, err)
		require.NoError(t, tk.Add(ns))
		prompts, err := tk.SystemPrompts(t.Context(), "memory:*")
		require.NoError(t, err)
		assert.Equal(t, []string{"The current date and time is Wednesday, 4 November 2026, 18:42, in the UTC time zone, UTC+00:00."}, prompts)

		now = now.Add(time.Hour)
		prompts, err = tk.SystemPrompts(t.Context(), "*")
		require.NoError(t, err)
		assert.Equal(t, []string{"The current date and time is Wednesday, 4 November 2026, 19:42, in the UTC time zone, UTC+00:00."}, prompts)
	})
}
