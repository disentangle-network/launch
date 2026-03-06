package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/disentangle-network/launch/internal/config"
	"github.com/disentangle-network/launch/internal/exec"
	"github.com/disentangle-network/launch/internal/paths"
	"github.com/spf13/cobra"
)

var statusFleetDir string

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show fleet health across clusters",
	Long:  "Checks FluxCD reconciliation status and pod health for each cluster in the fleet.",
	Example: `  launch-disentangle status
  launch-disentangle status --fleet-dir ~/custom-fleet`,
	RunE: runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
	statusCmd.Flags().StringVar(&statusFleetDir, "fleet-dir", "", "Path to fleet repository root")
}

// StatusParams holds all dependencies for Status.
type StatusParams struct {
	Exec     exec.Executor
	Paths    *paths.Resolver
	Stdout   io.Writer
	FleetDir string
}

// Status checks FluxCD reconciliation status and pod health for each cluster.
func Status(p StatusParams) error {
	fleetDir := p.Paths.FleetDir(p.FleetDir)
	clustersDir := filepath.Join(fleetDir, "clusters")

	entries, err := os.ReadDir(clustersDir)
	if err != nil {
		return fmt.Errorf("no clusters/ directory found -- are you in a fleet repo?")
	}

	if len(entries) == 0 {
		fmt.Fprintln(p.Stdout, "No clusters configured. Run 'launch cluster add <name>' to add one.")
		return nil
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		fmt.Fprintf(p.Stdout, "=== Cluster: %s ===\n", name)

		// Try to get flux status
		if _, err := p.Exec.RunSilent("kubectl", "--context", name, "get", "ns", "flux-system"); err == nil {
			fmt.Fprintln(p.Stdout, "  FluxCD:")
			if out, err := p.Exec.RunSilent("flux", "--context", name, "get", "all", "-A", "--no-header"); err == nil {
				for _, line := range splitLines(out.Stdout) {
					if line != "" {
						fmt.Fprintf(p.Stdout, "    %s\n", line)
					}
				}
			} else {
				fmt.Fprintln(p.Stdout, "    Could not query flux status")
			}

			fmt.Fprintln(p.Stdout, "  Disentangle pods:")
			if out, err := p.Exec.RunSilent("kubectl", "--context", name, "get", "pods", "-n", "disentangle", "-o", "wide", "--no-headers"); err == nil {
				if out.Stdout == "" {
					fmt.Fprintln(p.Stdout, "    No pods in disentangle namespace")
				} else {
					for _, line := range splitLines(out.Stdout) {
						if line != "" {
							fmt.Fprintf(p.Stdout, "    %s\n", line)
						}
					}
				}
			}
		} else {
			fmt.Fprintln(p.Stdout, "  FluxCD not installed (or cluster unreachable)")
		}
		fmt.Fprintln(p.Stdout)
	}

	return nil
}

func runStatus(cmd *cobra.Command, args []string) error {
	runner := exec.NewRunner()
	cfg, _ := config.Load(cfgFile)
	p := paths.NewWithHome("", cfg)
	if home, err := os.UserHomeDir(); err == nil {
		p = paths.NewWithHome(home, cfg)
	}

	return Status(StatusParams{
		Exec:     runner,
		Paths:    p,
		Stdout:   os.Stdout,
		FleetDir: statusFleetDir,
	})
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
