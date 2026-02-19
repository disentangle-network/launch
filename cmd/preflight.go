package cmd

import (
	"fmt"

	"github.com/disentangle-network/launch/internal/preflight"
	"github.com/spf13/cobra"
)

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
	fmt.Println("==> Checking required tools...")
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
			fmt.Printf("  %-12s %s  [%s]\n", r.Name, status, r.Version)
		} else {
			fmt.Printf("  %-12s %s\n", r.Name, status)
		}
	}

	if !preflight.HasAllRequired(toolResults) {
		missing := preflight.MissingRequired(toolResults)
		return fmt.Errorf("missing required tools: %v", missing)
	}

	fmt.Println("\n==> Checking credentials...")
	credResults := preflight.CheckCredentials()

	for _, c := range credResults {
		icon := "OK"
		switch c.Status {
		case "missing":
			icon = "MISSING"
		case "invalid":
			icon = "INVALID"
		}
		fmt.Printf("  %-20s %s  %s\n", c.Name, icon, c.Detail)
	}

	if !preflight.AllCredentialsOK(credResults) {
		return fmt.Errorf("some credentials are invalid -- fix them before proceeding")
	}

	fmt.Println("\nPreflight checks passed.")
	return nil
}
