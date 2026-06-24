package cli

import (
	"fmt"
	"io"
)

// Execute runs the CLI and translates command errors into a process exit code.
func Execute(args []string, stderr io.Writer) int {
	if err := Run(args); err != nil {
		if !IsRenderedError(err) {
			fmt.Fprintln(stderr, "fabric:", err)
		}
		return 1
	}
	return 0
}
