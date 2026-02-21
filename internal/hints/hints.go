package hints

import (
	"fmt"
	"io"
	"os"
)

type NextStep struct {
	Command     string
	Description string
}

// Fprint writes formatted next-step hints to the given writer.
func Fprint(w io.Writer, steps []NextStep) {
	if len(steps) == 0 {
		return
	}
	fmt.Fprintln(w, "\n--- Next steps ---")
	for _, s := range steps {
		fmt.Fprintf(w, "  launch-disentangle %-30s  %s\n", s.Command, s.Description)
	}
	fmt.Fprintln(w)
}

// Print writes formatted next-step hints to stdout.
func Print(steps []NextStep) {
	Fprint(os.Stdout, steps)
}
