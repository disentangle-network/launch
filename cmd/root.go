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
	Long: `launch manages Disentangle Network fleet deployments across any Kubernetes cluster.

Use with the fleet template: github.com/disentangle-network/fleet

Commands:
  setup          Configure credentials (GitHub, Cloudflare, SOPS)
  preflight      Validate tools and credentials
  cluster add    Add a cluster to the fleet
  cluster list   List clusters in the fleet
  secrets init   Bootstrap secrets for a cluster
  bootstrap      FluxCD bootstrap + genesis secrets
  mesh init      Generate nebula-pq CA certificate
  mesh add       Generate host certificates for a cluster
  status         Health check across clusters`,
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
