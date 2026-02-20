package hints

import "fmt"

type NextStep struct {
	Command     string
	Description string
}

func Print(steps []NextStep) {
	if len(steps) == 0 {
		return
	}
	fmt.Println("\n--- Next steps ---")
	for _, s := range steps {
		fmt.Printf("  launch %-30s  %s\n", s.Command, s.Description)
	}
	fmt.Println()
}
