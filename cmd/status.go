package cmd

import (
	"fmt"

	"github.com/disentangle-network/launch/internal/exec"
	"github.com/disentangle-network/launch/internal/state"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show pipeline state and cluster health",
	Long:  "Displays the current pipeline state and optionally checks cluster health.",
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	ps, err := state.Load("")
	if err != nil {
		return fmt.Errorf("loading state: %w", err)
	}

	if ps == nil {
		fmt.Println("No pipeline state found. Run 'launch all' or individual stages to begin.")
		return nil
	}

	fmt.Printf("Pipeline: %s\n", ps.PipelineID)
	fmt.Printf("Started:  %s\n\n", ps.StartedAt.Format("2006-01-02 15:04:05 UTC"))

	fmt.Println("Stages:")
	for _, name := range state.StageOrder {
		s := ps.Stages[name]
		status := string(s.Status)
		detail := ""
		switch s.Status {
		case state.StatusCompleted:
			if s.CompletedAt != nil {
				detail = fmt.Sprintf("  (completed %s)", s.CompletedAt.Format("15:04:05"))
			}
		case state.StatusFailed:
			detail = fmt.Sprintf("  (error: %s)", s.Error)
		case state.StatusInProgress:
			if s.StartedAt != nil {
				detail = fmt.Sprintf("  (started %s)", s.StartedAt.Format("15:04:05"))
			}
		}
		fmt.Printf("  %-12s %s%s\n", name, status, detail)
	}

	// Check cluster health if infra is complete
	infraStage := ps.Stages["infra"]
	if infraStage.Status == state.StatusCompleted {
		fmt.Println("\nCluster health:")
		runner := exec.NewRunner()
		if out, err := runner.RunSilent("kubectl", "get", "nodes", "-o", "wide"); err == nil {
			fmt.Println(out.Stdout)
		} else {
			fmt.Println("  Could not reach cluster (kubectl failed)")
		}
	}

	return nil
}
