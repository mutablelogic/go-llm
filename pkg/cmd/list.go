package cmd

import (
	"fmt"
	"os"

	// Packages
	tui "github.com/mutablelogic/go-llm/pkg/tui"
)

///////////////////////////////////////////////////////////////////////////////
// PUBLIC FUNCTIONS

func writeListTable[T tui.TableRow](rows []T, offset uint64, total uint64, options ...tui.Opt) error {
	if len(rows) > 0 {
		if _, err := tui.TableFor[T](options...).Write(os.Stdout, rows...); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(os.Stdout); err != nil {
			return err
		}
	}

	_, err := fmt.Fprintln(os.Stdout, listSummary(offset, len(rows), total))
	return err
}

func listSummary(offset uint64, shown int, total uint64) string {
	if shown <= 0 || total == 0 {
		return "No results"
	}

	start := offset + 1
	end := offset + uint64(shown)
	if end > total {
		end = total
	}

	return fmt.Sprintf("Showing %d-%d of %d items", start, end, total)
}
