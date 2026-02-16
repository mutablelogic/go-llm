package main

import "fmt"

// TableSummary returns a human-readable summary of the rows displayed.
// length is the number of rows shown, offset is the starting row index (0-based),
// and total is the total number of matching rows.
func TableSummary(length, offset, total int) string {
	if total == 0 {
		return "No results"
	}
	if offset == 0 && length >= total {
		return fmt.Sprintf("All %d rows displayed", total)
	}
	return fmt.Sprintf("Displaying rows %d-%d of %d", offset+1, offset+length, total)
}
