package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/disentangle-network/launch/internal/exec"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show fleet health across clusters",
	Long:  "Checks FluxCD reconciliation status and pod health for each cluster in the fleet.",
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	fleetDir := "."
	clustersDir := filepath.Join(fleetDir, "clusters")

	entries, err := os.ReadDir(clustersDir)
	if err != nil {
		return fmt.Errorf("no clusters/ directory found -- are you in a fleet repo?")
	}

	if len(entries) == 0 {
		fmt.Println("No clusters configured. Run 'launch cluster add <name>' to add one.")
		return nil
	}

	runner := exec.NewRunner()

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		fmt.Printf("=== Cluster: %s ===\n", name)

		// Try to get flux status
		if _, err := runner.RunSilent("kubectl", "--context", name, "get", "ns", "flux-system"); err == nil {
			fmt.Println("  FluxCD:")
			if out, err := runner.RunSilent("flux", "--context", name, "get", "all", "-A", "--no-header"); err == nil {
				for _, line := range splitLines(out.Stdout) {
					if line != "" {
						fmt.Printf("    %s\n", line)
					}
				}
			} else {
				fmt.Println("    Could not query flux status")
			}

			fmt.Println("  Disentangle pods:")
			if out, err := runner.RunSilent("kubectl", "--context", name, "get", "pods", "-n", "disentangle", "-o", "wide", "--no-headers"); err == nil {
				if out.Stdout == "" {
					fmt.Println("    No pods in disentangle namespace")
				} else {
					for _, line := range splitLines(out.Stdout) {
						if line != "" {
							fmt.Printf("    %s\n", line)
						}
					}
				}
			}
		} else {
			fmt.Println("  FluxCD not installed (or cluster unreachable)")
		}
		fmt.Println()
	}

	return nil
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
