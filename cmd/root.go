package cmd

import (
	"fmt"

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
	Long: `launch manages Disentangle Network deployments across any Kubernetes cluster.

Fleet management:
  init           Scaffold a fleet monorepo
  cluster add    Add a cluster to the fleet
  secrets init   Bootstrap secrets for a cluster
  bootstrap      FluxCD bootstrap + genesis unseal
  mesh           Nebula-PQ overlay mesh management
  status         Health check across clusters

Legacy pipeline (OCI-specific):
  discover       OCI resource discovery
  infra          Provision OKE cluster
  deploy         Deploy via FluxCD`,
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
	_, _ = fmt.Scanln(&response)
	return response == "y" || response == "Y" || response == "yes"
}

