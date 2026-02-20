package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/disentangle-network/launch/internal/exec"
	"github.com/spf13/cobra"
)

var secretsCmd = &cobra.Command{
	Use:   "secrets",
	Short: "Manage cluster secrets via genesis-operator",
}

var (
	secretsCluster  string
	secretsProvider string
	secretsKeyARN   string
	secretsFleetDir string
)

var secretsInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize secrets for a cluster",
	Long: `Generate an age keypair for the cluster and envelope-encrypt it
using the specified KMS provider (oci-vault, aws-kms, gcp-kms, yubikey, age).

This creates a SOPS configuration entry for the cluster and stores the
encrypted age key in the fleet repo's secrets directory.`,
	RunE: runSecretsInit,
}

func init() {
	rootCmd.AddCommand(secretsCmd)
	secretsCmd.AddCommand(secretsInitCmd)

	secretsInitCmd.Flags().StringVar(&secretsCluster, "cluster", "", "Cluster name (required)")
	secretsInitCmd.Flags().StringVar(&secretsProvider, "provider", "age", "KMS provider (age, yubikey, oci-vault, aws-kms, gcp-kms)")
	secretsInitCmd.Flags().StringVar(&secretsKeyARN, "key-arn", "", "KMS key ARN/OCID (for cloud providers)")
	secretsInitCmd.Flags().StringVar(&secretsFleetDir, "fleet-dir", ".", "Path to fleet repository root")
	_ = secretsInitCmd.MarkFlagRequired("cluster")
}

func runSecretsInit(cmd *cobra.Command, args []string) error {
	// Verify cluster exists
	clusterDir := filepath.Join(secretsFleetDir, "clusters", secretsCluster)
	if _, err := os.Stat(clusterDir); os.IsNotExist(err) {
		return fmt.Errorf("cluster '%s' not found (run 'launch cluster add %s' first)", secretsCluster, secretsCluster)
	}

	// Create secrets directory
	secretsDir := filepath.Join(secretsFleetDir, "secrets", secretsCluster)
	if err := os.MkdirAll(secretsDir, 0755); err != nil {
		return err
	}

	fmt.Printf("Initializing secrets for cluster '%s' (provider: %s)\n", secretsCluster, secretsProvider)

	runner := exec.NewRunner()

	switch secretsProvider {
	case "age":
		// Generate age keypair
		keyPath := filepath.Join(secretsDir, "age.key")
		if _, err := os.Stat(keyPath); err == nil {
			fmt.Printf("Age key already exists at %s\n", keyPath)
			return nil
		}

		result, err := runner.Run("age-keygen", "-o", keyPath)
		if err != nil {
			return fmt.Errorf("failed to generate age key: %w", err)
		}

		// Extract public key from output
		fmt.Printf("Age key generated at %s\n", keyPath)
		if result != nil && len(result.Stdout) > 0 {
			fmt.Printf("Public key: %s\n", result.Stdout)
		}

		// Update .sops.yaml
		fmt.Printf("\nAdd this to %s/.sops.yaml:\n", secretsFleetDir)
		fmt.Printf("  - path_regex: secrets/%s/.*\n", secretsCluster)
		fmt.Printf("    age: <public-key-from-above>\n")

	case "yubikey":
		fmt.Println("YubiKey provider: genesis init will be called during bootstrap")
		fmt.Println("Ensure your YubiKey is connected when running 'launch bootstrap'")

	case "oci-vault", "aws-kms", "gcp-kms":
		if secretsKeyARN == "" {
			return fmt.Errorf("--key-arn is required for provider %s", secretsProvider)
		}
		fmt.Printf("KMS key: %s\n", secretsKeyARN)
		fmt.Println("Genesis will use this key during bootstrap to envelope-encrypt the age key")

	default:
		return fmt.Errorf("unknown provider: %s (valid: age, yubikey, oci-vault, aws-kms, gcp-kms)", secretsProvider)
	}

	// Write provider config for bootstrap to use later
	providerCfg := fmt.Sprintf("provider: %s\nkey_arn: %s\ncluster: %s\n", secretsProvider, secretsKeyARN, secretsCluster)
	cfgPath := filepath.Join(secretsDir, "genesis-config.yaml")
	if err := os.WriteFile(cfgPath, []byte(providerCfg), 0600); err != nil {
		return err
	}

	fmt.Printf("\nSecrets initialized for cluster '%s'\n", secretsCluster)
	fmt.Printf("Next: launch bootstrap --cluster %s\n", secretsCluster)
	return nil
}
