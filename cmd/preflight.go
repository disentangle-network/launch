package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/disentangle-network/launch/internal/exec"
	"github.com/disentangle-network/launch/internal/hints"
	"github.com/disentangle-network/launch/internal/preflight"
	"github.com/spf13/cobra"
)

// PreflightParams holds dependencies for the preflight command.
type PreflightParams struct {
	Exec   exec.Executor
	Stdout io.Writer
}

var preflightCmd = &cobra.Command{
	Use:   "preflight",
	Short: "Check required tools and credentials",
	Long:  "Validates that all required CLI tools are installed and credentials are configured.",
	RunE:  runPreflight,
}

func init() {
	rootCmd.AddCommand(preflightCmd)
}

func runPreflight(cmd *cobra.Command, args []string) error {
	runner := exec.NewRunner()
	runner.Verbose = verbose
	runner.DryRun = dryRun
	return Preflight(PreflightParams{
		Exec:   runner,
		Stdout: os.Stdout,
	})
}

// Preflight checks required tools and credentials.
func Preflight(p PreflightParams) error {
	fmt.Fprintln(p.Stdout, "==> Checking required tools...")
	toolResults := preflight.CheckTools()

	for _, r := range toolResults {
		status := "OK"
		if !r.Available {
			if r.Required {
				status = "MISSING (required)"
			} else {
				status = "MISSING (optional)"
			}
		}
		if r.Available && r.Version != "" {
			fmt.Fprintf(p.Stdout, "  %-12s %s  [%s]\n", r.Name, status, r.Version)
		} else {
			fmt.Fprintf(p.Stdout, "  %-12s %s\n", r.Name, status)
		}
	}

	if !preflight.HasAllRequired(toolResults) {
		missing := preflight.MissingRequired(toolResults)
		return fmt.Errorf("missing required tools: %v", missing)
	}

	fmt.Fprintln(p.Stdout, "\n==> Checking credentials...")
	credResults := preflight.CheckCredentials(p.Exec)

	for _, c := range credResults {
		icon := "OK"
		switch c.Status {
		case "missing":
			icon = "MISSING"
		case "invalid":
			icon = "INVALID"
		}
		fmt.Fprintf(p.Stdout, "  %-20s %s  %s\n", c.Name, icon, c.Detail)
	}

	if !preflight.AllCredentialsOK(credResults) {
		return fmt.Errorf("some credentials are invalid -- fix them before proceeding")
	}

	fmt.Fprintln(p.Stdout, "\nPreflight checks passed.")
	hints.Fprint(p.Stdout, []hints.NextStep{
		{Command: "infra plan", Description: "Preview OCI infrastructure"},
		{Command: "cluster add <name>", Description: "Add a cluster to the fleet"},
	})
	return nil
}
