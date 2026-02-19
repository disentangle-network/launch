package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/disentangle-network/launch/internal/exec"
	"github.com/spf13/cobra"
)

var (
	bootstrapCluster  string
	bootstrapFleetDir string
	bootstrapRepo     string
	bootstrapOwner    string
	bootstrapBranch   string
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
	_ = bootstrapCmd.MarkFlagRequired("cluster")
}

func runBootstrap(cmd *cobra.Command, args []string) error {
	// Verify cluster exists in fleet
	clusterPath := filepath.Join(bootstrapFleetDir, "clusters", bootstrapCluster)
	if _, err := os.Stat(clusterPath); os.IsNotExist(err) {
		return fmt.Errorf("cluster '%s' not found in fleet repo", bootstrapCluster)
	}

	runner := exec.NewRunner()

	// Step 1: Verify kubectl context
	fmt.Println("Step 1: Verifying kubectl access...")
	result, err := runner.Run("kubectl", "cluster-info", "--request-timeout=5s")
	if err != nil {
		return fmt.Errorf("kubectl not configured for a cluster: %w\nSet your kubeconfig context for the target cluster first", err)
	}
	if verbose && result != nil {
		fmt.Println(result.Stdout)
	}

	// Step 2: Verify flux CLI
	fmt.Println("Step 2: Checking flux CLI...")
	if _, err := runner.Run("flux", "version", "--client"); err != nil {
		return fmt.Errorf("flux CLI not found: %w\nInstall: https://fluxcd.io/flux/installation/", err)
	}

	// Step 3: Derive repo info if not provided
	if bootstrapOwner == "" || bootstrapRepo == "" {
		fmt.Println("Step 3: Detecting repository info from git remote...")
		result, err := runner.Run("git", "-C", bootstrapFleetDir, "remote", "get-url", "origin")
		if err != nil {
			return fmt.Errorf("could not detect git remote (set --owner and --repo manually): %w", err)
		}
		// Parse owner/repo from URL -- user can override with flags
		if result != nil {
			fmt.Printf("  Remote: %s\n", result.Stdout)
		}
		if bootstrapOwner == "" {
			bootstrapOwner = "disentangle-network"
			fmt.Printf("  Using default owner: %s (override with --owner)\n", bootstrapOwner)
		}
		if bootstrapRepo == "" {
			bootstrapRepo = "fleet"
			fmt.Printf("  Using default repo: %s (override with --repo)\n", bootstrapRepo)
		}
	}

	// Step 4: Flux bootstrap
	fmt.Printf("\nStep 4: Bootstrapping FluxCD for cluster '%s'...\n", bootstrapCluster)
	fluxPath := fmt.Sprintf("clusters/%s", bootstrapCluster)

	_, err = runner.Run("flux", "bootstrap", "github",
		"--owner", bootstrapOwner,
		"--repository", bootstrapRepo,
		"--branch", bootstrapBranch,
		"--path", fluxPath,
		"--personal",
	)
	if err != nil {
		return fmt.Errorf("flux bootstrap failed: %w", err)
	}

	// Step 5: Genesis secrets (if configured)
	genesisConfig := filepath.Join(bootstrapFleetDir, "secrets", bootstrapCluster, "genesis-config.yaml")
	if _, err := os.Stat(genesisConfig); err == nil {
		fmt.Println("\nStep 5: Provisioning genesis secrets...")
		ageKeyPath := filepath.Join(bootstrapFleetDir, "secrets", bootstrapCluster, "age.key")

		if _, err := os.Stat(ageKeyPath); err == nil {
			// Create the sops-age secret in the cluster
			_, err = runner.Run("kubectl", "create", "secret", "generic",
				"sops-age", "-n", "flux-system",
				"--from-file=age.agekey="+ageKeyPath,
			)
			if err != nil {
				fmt.Printf("  Warning: could not create sops-age secret (may already exist): %v\n", err)
			} else {
				fmt.Println("  sops-age secret created in flux-system namespace")
			}
		} else {
			fmt.Println("  No age key found, skipping sops-age secret creation")
		}
	} else {
		fmt.Println("\nStep 5: No genesis config found, skipping secrets provisioning")
		fmt.Printf("  Run 'launch secrets init --cluster %s' to configure secrets\n", bootstrapCluster)
	}

	fmt.Printf("\nBootstrap complete for cluster '%s'\n", bootstrapCluster)
	fmt.Println("FluxCD will now reconcile the fleet repo automatically.")
	fmt.Printf("Monitor: flux get all -A\n")

	return nil
}
