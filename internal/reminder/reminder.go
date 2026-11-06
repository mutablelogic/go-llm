// Package reminder has the block of what the system says to a model, in a
// message, which it reads as information, rather than what the person said,
// and which is taken out of what the system doesn't say, so it can't be
// faked, such as by the person, or what a model, or a tool, wrote
package reminder

import "strings"

///////////////////////////////////////////////////////////////////////////////
// GLOBALS

// Tag is the tag of the block, as <system-reminder>...</system-reminder>
const Tag = "system-reminder"

///////////////////////////////////////////////////////////////////////////////
// PUBLIC FUNCTIONS

// Strip returns the text without the block's tags. They're taken out until
// there are none, so a tag which taking one out makes, such as
// <sys<system-reminder>tem-reminder>, is taken out too.
func Strip(text string) string {
	for {
		stripped := text
		for _, tag := range []string{"<" + Tag + ">", "</" + Tag + ">"} {
			stripped = strings.ReplaceAll(stripped, tag, "")
		}
		if stripped == text {
			return text
		}
		text = stripped
	}
}
