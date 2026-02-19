package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	cfgFile   string
	verbose   bool
	dryRun    bool
	autoYes   bool
	version   = "dev"
	commit    = "none"
	buildDate = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "launch",
	Short: "Deployment orchestrator for the Disentangle Network",
	Long: `launch orchestrates the full deployment pipeline for the Disentangle Network,
from OCI resource discovery through cluster provisioning, secrets bootstrapping,
and node deployment.

Pipeline stages:
  1. discover  - OCI resource discovery (oci-tf-bootstrap)
  2. infra     - Provision OKE cluster (k8s-oci-foundation)
  3. secrets   - Bootstrap secrets (genesis-operator)
  4. deploy    - Deploy nodes (FluxCD/Helm)`,
	SilenceUsage:  true,
	SilenceErrors: true,
	Version:       fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, buildDate),
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ~/.config/launch/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "show commands without executing")
	rootCmd.PersistentFlags().BoolVarP(&autoYes, "yes", "y", false, "non-interactive mode (skip confirmations)")
}

func confirm(prompt string) bool {
	if autoYes {
		return true
	}
	fmt.Printf("%s [y/N]: ", prompt)
	var response string
	fmt.Scanln(&response)
	return response == "y" || response == "Y" || response == "yes"
}

func exitOnError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
