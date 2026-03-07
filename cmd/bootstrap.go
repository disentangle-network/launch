package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/disentangle-network/launch/internal/config"
	"github.com/disentangle-network/launch/internal/exec"
	"github.com/disentangle-network/launch/internal/hints"
	"github.com/disentangle-network/launch/internal/paths"
	"github.com/spf13/cobra"
)

var (
	bootstrapCluster  string
	bootstrapFleetDir string
	bootstrapRepo     string
	bootstrapOwner    string
	bootstrapBranch   string
	bootstrapContext  string
)

var bootstrapCmd = &cobra.Command{
	Use:   "bootstrap",
	Short: "Bootstrap FluxCD and genesis secrets on a cluster",
	Long: `Bootstrap a cluster by installing FluxCD (pointing at the fleet repo)
and provisioning genesis secrets.

Requires:
  - kubectl configured for the target cluster
  - GitHub token (GITHUB_TOKEN env var or gh CLI auth)
  - Fleet repo pushed to GitHub`,
	RunE: runBootstrap,
}

func init() {
	rootCmd.AddCommand(bootstrapCmd)

	bootstrapCmd.Flags().StringVar(&bootstrapCluster, "cluster", "", "Cluster name (required)")
	bootstrapCmd.Flags().StringVar(&bootstrapFleetDir, "fleet-dir", ".", "Path to fleet repository root")
	bootstrapCmd.Flags().StringVar(&bootstrapRepo, "repo", "", "GitHub repository name (default: derived from fleet dir)")
	bootstrapCmd.Flags().StringVar(&bootstrapOwner, "owner", "", "GitHub owner/org (default: derived from git remote)")
	bootstrapCmd.Flags().StringVar(&bootstrapBranch, "branch", "main", "Git branch")
	bootstrapCmd.Flags().StringVar(&bootstrapContext, "context", "", "Kubernetes context to use for kubectl and flux commands")
	_ = bootstrapCmd.MarkFlagRequired("cluster")
}

// BootstrapParams holds all dependencies for Bootstrap.
type BootstrapParams struct {
	Exec     exec.Executor
	Paths    *paths.Resolver
	Stdout   io.Writer
	Cluster  string
	FleetDir string
	Repo     string
	Owner    string
	Branch   string
	Context  string // Kubernetes context override
	Verbose  bool
}

// Bootstrap installs FluxCD and provisions genesis secrets on a cluster.
func Bootstrap(p BootstrapParams) error {
	// Verify cluster exists in fleet
	clusterPath := filepath.Join(p.FleetDir, "clusters", p.Cluster)
	if _, err := os.Stat(clusterPath); os.IsNotExist(err) {
		return fmt.Errorf("cluster '%s' not found in fleet repo", p.Cluster)
	}

	// Step 1: Verify kubectl context
	fmt.Fprintln(p.Stdout, "Step 1: Verifying kubectl access...")
	kubectlArgs := []string{"cluster-info", "--request-timeout=5s"}
	if p.Context != "" {
		kubectlArgs = append(kubectlArgs, "--context", p.Context)
	}
	result, err := p.Exec.Run("kubectl", kubectlArgs...)
	if err != nil {
		return fmt.Errorf("kubectl not configured for a cluster: %w\nSet your kubeconfig context for the target cluster first", err)
	}
	if p.Verbose && result != nil {
		fmt.Fprintln(p.Stdout, result.Stdout)
	}

	// Step 2: Verify flux CLI
	fmt.Fprintln(p.Stdout, "Step 2: Checking flux CLI...")
	if _, err := p.Exec.Run("flux", "version", "--client"); err != nil {
		return fmt.Errorf("flux CLI not found: %w\nInstall: https://fluxcd.io/flux/installation/", err)
	}

	// Step 3: Derive repo info from git remote if not provided via flags
	owner := p.Owner
	repo := p.Repo
	if owner == "" || repo == "" {
		fmt.Fprintln(p.Stdout, "Step 3: Detecting repository info from git remote...")
		result, err := p.Exec.RunSilent("git", "-C", p.FleetDir, "remote", "get-url", "origin")
		if err != nil {
			return fmt.Errorf("could not detect git remote (set --owner and --repo manually): %w", err)
		}
		remoteURL := strings.TrimSpace(result.Stdout)
		fmt.Fprintf(p.Stdout, "  Remote: %s\n", remoteURL)
		parsedOwner, parsedRepo := parseGitRemote(remoteURL)
		if owner == "" {
			if parsedOwner == "" {
				return fmt.Errorf("could not parse owner from remote %q (set --owner manually)", remoteURL)
			}
			owner = parsedOwner
			fmt.Fprintf(p.Stdout, "  Detected owner: %s\n", owner)
		}
		if repo == "" {
			if parsedRepo == "" {
				return fmt.Errorf("could not parse repo from remote %q (set --repo manually)", remoteURL)
			}
			repo = parsedRepo
			fmt.Fprintf(p.Stdout, "  Detected repo: %s\n", repo)
		}
	}

	// Step 4: Flux bootstrap
	fmt.Fprintf(p.Stdout, "\nStep 4: Bootstrapping FluxCD for cluster '%s'...\n", p.Cluster)
	fluxPath := fmt.Sprintf("clusters/%s", p.Cluster)

	fluxArgs := []string{"bootstrap", "github",
		"--owner", owner,
		"--repository", repo,
		"--branch", p.Branch,
		"--path", fluxPath,
		"--personal",
	}
	if p.Context != "" {
		fluxArgs = append(fluxArgs, "--context", p.Context)
	}

	_, err = p.Exec.Run("flux", fluxArgs...)
	if err != nil {
		return fmt.Errorf("flux bootstrap failed: %w", err)
	}

	// Step 5: Genesis secrets (if configured)
	genesisConfig := filepath.Join(p.FleetDir, "secrets", p.Cluster, "genesis-config.yaml")
	if _, err := os.Stat(genesisConfig); err == nil {
		fmt.Fprintln(p.Stdout, "\nStep 5: Provisioning genesis secrets...")

		secretsDir := filepath.Join(p.FleetDir, "secrets", p.Cluster)

		// Find age identity: prefer genesis identity file, fall back to legacy names
		var ageKeyPath string
		var keySource string
		candidates := []struct {
			filename string
			source   string
		}{
			{"genesis-identity.key", "genesis PQ hybrid"},
			{"age-identity.txt", "genesis PQ hybrid"},
			{"age.key", "age (classical)"},
		}
		for _, c := range candidates {
			path := filepath.Join(secretsDir, c.filename)
			if _, err := os.Stat(path); err == nil {
				ageKeyPath = path
				keySource = c.source
				break
			}
		}

		if ageKeyPath != "" {
			fmt.Fprintf(p.Stdout, "  Using %s identity from %s\n", keySource, ageKeyPath)
			// Create the sops-age secret in the cluster
			kubectlSecretArgs := []string{"create", "secret", "generic",
				"sops-age", "-n", "flux-system",
				"--from-file=age.agekey=" + ageKeyPath,
			}
			if p.Context != "" {
				kubectlSecretArgs = append(kubectlSecretArgs, "--context", p.Context)
			}
			_, err = p.Exec.Run("kubectl", kubectlSecretArgs...)
			if err != nil {
				fmt.Fprintf(p.Stdout, "  Warning: could not create sops-age secret (may already exist): %v\n", err)
			} else {
				fmt.Fprintln(p.Stdout, "  sops-age secret created in flux-system namespace")
			}
		} else {
			fmt.Fprintln(p.Stdout, "  No age identity found, skipping sops-age secret creation")
			fmt.Fprintf(p.Stdout, "  Run 'launch secrets init --cluster %s' to configure secrets\n", p.Cluster)
		}
	} else {
		fmt.Fprintln(p.Stdout, "\nStep 5: No genesis config found, skipping secrets provisioning")
		fmt.Fprintf(p.Stdout, "  Run 'launch secrets init --cluster %s' to configure secrets\n", p.Cluster)
	}

	fmt.Fprintf(p.Stdout, "\nBootstrap complete for cluster '%s'\n", p.Cluster)
	fmt.Fprintln(p.Stdout, "FluxCD will now reconcile the fleet repo automatically.")
	hints.Fprint(p.Stdout, []hints.NextStep{
		{Command: "status", Description: "Check deployment health"},
		{Command: "mesh init", Description: "Generate Nebula-PQ CA certificate"},
		{Command: "mesh add --cluster " + p.Cluster, Description: "Add cluster to PQ mesh"},
	})

	return nil
}

func runBootstrap(cmd *cobra.Command, args []string) error {
	runner := exec.NewRunner()
	cfg, _ := config.Load(cfgFile)
	p := paths.NewWithHome("", cfg)
	if home, err := os.UserHomeDir(); err == nil {
		p = paths.NewWithHome(home, cfg)
	}

	return Bootstrap(BootstrapParams{
		Exec:     runner,
		Paths:    p,
		Stdout:   os.Stdout,
		Cluster:  bootstrapCluster,
		FleetDir: bootstrapFleetDir,
		Repo:     bootstrapRepo,
		Owner:    bootstrapOwner,
		Branch:   bootstrapBranch,
		Context:  bootstrapContext,
		Verbose:  verbose,
	})
}

// parseGitRemote extracts owner and repo from a git remote URL.
// Supports https://github.com/owner/repo.git and git@github.com:owner/repo.git.
func parseGitRemote(url string) (owner, repo string) {
	// Strip trailing .git
	url = strings.TrimSuffix(url, ".git")

	// SSH: git@github.com:owner/repo
	if strings.Contains(url, ":") && !strings.Contains(url, "://") {
		parts := strings.SplitN(url, ":", 2)
		if len(parts) == 2 {
			segments := strings.Split(parts[1], "/")
			if len(segments) >= 2 {
				return segments[len(segments)-2], segments[len(segments)-1]
			}
		}
		return "", ""
	}

	// HTTPS: https://github.com/owner/repo
	parts := strings.Split(url, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2], parts[len(parts)-1]
	}
	return "", ""
}
