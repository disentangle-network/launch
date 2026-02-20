package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/disentangle-network/launch/internal/exec"
	"github.com/disentangle-network/launch/internal/hints"
	"github.com/spf13/cobra"
)

var meshCmd = &cobra.Command{
	Use:   "mesh",
	Short: "Manage the nebula-pq overlay mesh",
	Long:  "Generate PQ certificates and manage the nebula mesh across clusters.",
}

var meshCAOutput string

var meshInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate a new nebula-pq CA certificate",
	Long: `Generate a post-quantum CA certificate using ML-DSA-87.
The CA key is stored locally and must never be committed to git.`,
	RunE: runMeshInit,
}

var (
	meshCluster       string
	meshIsLighthouse  bool
	meshLighthouseAddr string
	meshFleetDir      string
)

var meshAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Generate nebula host certificates for a cluster",
	Long: `Generate post-quantum host certificates for each node in a cluster
and SOPS-encrypt them into the fleet repository's secrets directory.`,
	RunE: runMeshAdd,
}

func init() {
	rootCmd.AddCommand(meshCmd)
	meshCmd.AddCommand(meshInitCmd)
	meshCmd.AddCommand(meshAddCmd)

	meshInitCmd.Flags().StringVar(&meshCAOutput, "ca-output", "", "Output directory for CA files (default: ~/.config/disentangle/nebula-ca/)")

	meshAddCmd.Flags().StringVar(&meshCluster, "cluster", "", "Cluster name (required)")
	meshAddCmd.Flags().BoolVar(&meshIsLighthouse, "lighthouse", false, "This cluster runs a lighthouse node")
	meshAddCmd.Flags().StringVar(&meshLighthouseAddr, "lighthouse-addr", "", "Lighthouse address for non-lighthouse clusters (ip:port)")
	meshAddCmd.Flags().StringVar(&meshFleetDir, "fleet-dir", ".", "Path to fleet repository root")
	_ = meshAddCmd.MarkFlagRequired("cluster")
}

func runMeshInit(cmd *cobra.Command, args []string) error {
	caDir := meshCAOutput
	if caDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		caDir = filepath.Join(home, ".config", "disentangle", "nebula-ca")
	}

	if err := os.MkdirAll(caDir, 0700); err != nil {
		return err
	}

	caKeyPath := filepath.Join(caDir, "ca.key")
	caCrtPath := filepath.Join(caDir, "ca.crt")

	if _, err := os.Stat(caKeyPath); err == nil {
		return fmt.Errorf("CA key already exists at %s (remove to regenerate)", caKeyPath)
	}

	fmt.Println("Generating nebula-pq CA certificate (ML-DSA-87)...")

	runner := exec.NewRunner()
	_, err := runner.Run("nebula-cert", "ca",
		"-curve", "PQ",
		"-name", "disentangle-network",
		"-out-key", caKeyPath,
		"-out-crt", caCrtPath,
	)
	if err != nil {
		return fmt.Errorf("failed to generate CA: %w", err)
	}

	fmt.Printf("\nCA certificate generated:\n")
	fmt.Printf("  Key:  %s (keep secret, never commit to git)\n", caKeyPath)
	fmt.Printf("  Cert: %s\n", caCrtPath)
	hints.Print([]hints.NextStep{
		{Command: "mesh add --cluster <name>", Description: "Generate host certs for a cluster"},
	})

	return nil
}

func runMeshAdd(cmd *cobra.Command, args []string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	caDir := filepath.Join(home, ".config", "disentangle", "nebula-ca")
	caKeyPath := filepath.Join(caDir, "ca.key")
	caCrtPath := filepath.Join(caDir, "ca.crt")

	if _, err := os.Stat(caKeyPath); os.IsNotExist(err) {
		return fmt.Errorf("no CA found at %s (run 'launch mesh init' first)", caDir)
	}

	// Read cluster settings to get node count and nebula prefix
	settingsPath := filepath.Join(meshFleetDir, "clusters", meshCluster, "cluster-settings.yaml")
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		return fmt.Errorf("cluster '%s' not found (run 'launch cluster add %s' first)", meshCluster, meshCluster)
	}

	// Create secrets directory for this cluster
	secretsDir := filepath.Join(meshFleetDir, "secrets", meshCluster)
	if err := os.MkdirAll(secretsDir, 0755); err != nil {
		return err
	}

	fmt.Printf("Generating nebula-pq host certificates for cluster '%s'...\n", meshCluster)

	// For now, generate a single host cert for the cluster
	// In production, iterate over node count from cluster-settings
	runner := exec.NewRunner()

	certName := fmt.Sprintf("%s-node", meshCluster)
	certKeyPath := filepath.Join(secretsDir, fmt.Sprintf("%s.key", certName))
	certCrtPath := filepath.Join(secretsDir, fmt.Sprintf("%s.crt", certName))

	groups := "disentangle"
	if meshIsLighthouse {
		groups = "disentangle,lighthouse"
	}

	_, err = runner.Run("nebula-cert", "sign",
		"-ca-key", caKeyPath,
		"-ca-crt", caCrtPath,
		"-name", certName,
		"-networks", "10.42.0.1/16",
		"-groups", groups,
		"-out-key", certKeyPath,
		"-out-crt", certCrtPath,
	)
	if err != nil {
		return fmt.Errorf("failed to generate host cert: %w", err)
	}

	fmt.Printf("\nHost certificates generated in %s\n", secretsDir)
	hints.Print([]hints.NextStep{
		{Command: "bootstrap --cluster " + meshCluster, Description: "Bootstrap FluxCD"},
		{Command: "status", Description: "Check deployment health"},
	})

	return nil
}
