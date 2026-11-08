package llm_test

import (
	"testing"

	// Packages
	llm "github.com/mutablelogic/go-llm"
	assert "github.com/stretchr/testify/assert"
)

func Test_Effort(t *testing.T) {
	t.Run("Option", func(t *testing.T) {
		// Each of the efforts can be asked for, and no other
		for _, effort := range llm.Efforts {
			o, err := llm.ApplyOpts(llm.WithThinkingEffort(effort))
			if assert.NoError(t, err, effort) {
				value, err := llm.GetOpt[string](o, llm.ThinkingEffortKey)
				assert.NoError(t, err)
				assert.Equal(t, effort, value)
			}
		}
		_, err := llm.ApplyOpts(llm.WithThinkingEffort("huge"))
		assert.ErrorIs(t, err, llm.ErrBadParameter)
	})

	t.Run("Nearest", func(t *testing.T) {
		// An effort is one of the levels, or the nearest, and the greater,
		// when two are as near
		levels := []string{"low", "high", "max"}
		for effort, want := range map[string]string{
			"low": "low", "medium": "high", "high": "high", "xhigh": "max", "max": "max",
			"none": "low", "minimal": "low", "huge": "huge",
		} {
			assert.Equal(t, want, llm.NearestEffort(effort, levels...), effort)
		}
		assert.Equal(t, "high", llm.NearestEffort("max", "low", "medium", "high"))
		assert.Equal(t, "max", llm.NearestEffort("max"), "with no levels, it's the effort")
	})

	t.Run("Most", func(t *testing.T) {
		// An effort is no more than the most
		for effort, want := range map[string]string{"none": "none", "low": "low", "high": "high", "xhigh": "high", "max": "high", "huge": "huge"} {
			assert.Equal(t, want, llm.MostEffort(effort, "high"), effort)
		}
	})
}
