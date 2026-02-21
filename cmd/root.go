package cmd

import (
	"fmt"
	"runtime/debug"

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

// versionInfo returns a human-readable version string.
//
// When built via GoReleaser (ldflags inject version != "dev"), the injected
// values are used verbatim.  For plain `go build` or `go install` the
// function falls back to runtime/debug.ReadBuildInfo so that VCS metadata
// embedded by the toolchain is surfaced instead of the useless default
// "dev (commit: none, built: unknown)".
func versionInfo() string {
	if version != "dev" {
		// GoReleaser path – use the injected ldflags values.
		return fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, buildDate)
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev (unknown commit)"
	}

	// `go install module@vX.Y.Z` embeds the module version in Main.Version.
	if info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}

	// Plain `go build` – extract VCS settings stamped by the toolchain.
	var rev, ts string
	modified := false
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.time":
			ts = s.Value
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}

	if rev == "" {
		return "dev (unknown commit)"
	}

	short := rev
	if len(rev) > 7 {
		short = rev[:7]
	}

	if modified {
		return fmt.Sprintf("dev-%s (dirty)", short)
	}
	if ts != "" {
		return fmt.Sprintf("dev-%s (%s)", short, ts)
	}
	return fmt.Sprintf("dev-%s", short)
}

var rootCmd = &cobra.Command{
	Use:   "launch-disentangle",
	Short: "Deployment orchestrator for the Disentangle Network",
	Long: `launch-disentangle manages Disentangle Network fleet deployments across any Kubernetes cluster.

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
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.Version = versionInfo()
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
