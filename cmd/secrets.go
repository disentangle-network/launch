package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/disentangle-network/launch/internal/config"
	"github.com/disentangle-network/launch/internal/exec"
	"github.com/disentangle-network/launch/internal/paths"
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

// genesisExistsFunc checks if the genesis CLI is available.
// Override in tests to avoid requiring the binary.
var genesisExistsFunc = exec.CommandExists

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
	secretsInitCmd.Flags().StringVar(&secretsProvider, "provider", "age", "KMS provider (local, age, yubikey, oci-vault, aws-kms, gcp-kms)")
	secretsInitCmd.Flags().StringVar(&secretsKeyARN, "key-arn", "", "KMS key ARN/OCID (for cloud providers)")
	secretsInitCmd.Flags().StringVar(&secretsFleetDir, "fleet-dir", ".", "Path to fleet repository root")
	_ = secretsInitCmd.MarkFlagRequired("cluster")
}

// SecretsInitParams holds all dependencies for SecretsInit.
type SecretsInitParams struct {
	Exec     exec.Executor
	Paths    *paths.Resolver
	Stdout   io.Writer
	Cluster  string
	Provider string
	KeyARN   string
	FleetDir string
}

// SecretsInit initializes secrets for a cluster using the specified KMS provider.
func SecretsInit(p SecretsInitParams) error {
	// Verify cluster exists
	clusterDir := p.Paths.FleetClusterDir(p.FleetDir, p.Cluster)
	if _, err := os.Stat(clusterDir); os.IsNotExist(err) {
		return fmt.Errorf("cluster '%s' not found (run 'launch cluster add %s' first)", p.Cluster, p.Cluster)
	}

	// Create secrets directory
	secretsDir := p.Paths.FleetSecretsDir(p.FleetDir, p.Cluster)
	if err := os.MkdirAll(secretsDir, 0750); err != nil {
		return err
	}

	fmt.Fprintf(p.Stdout, "Initializing secrets for cluster '%s' (provider: %s)\n", p.Cluster, p.Provider)

	switch p.Provider {
	case "age":
		// Generate age keypair
		keyPath := filepath.Join(secretsDir, "age.key")
		if _, err := os.Stat(keyPath); err == nil {
			fmt.Fprintf(p.Stdout, "Age key already exists at %s\n", keyPath)
			return nil
		}

		result, err := p.Exec.Run("age-keygen", "-o", keyPath)
		if err != nil {
			return fmt.Errorf("failed to generate age key: %w", err)
		}

		// Extract public key from output
		fmt.Fprintf(p.Stdout, "Age key generated at %s\n", keyPath)
		if result != nil && len(result.Stdout) > 0 {
			fmt.Fprintf(p.Stdout, "Public key: %s\n", result.Stdout)
		}

		// Update .sops.yaml
		fmt.Fprintf(p.Stdout, "\nAdd this to %s/.sops.yaml:\n", p.FleetDir)
		fmt.Fprintf(p.Stdout, "  - path_regex: secrets/%s/.*\n", p.Cluster)
		fmt.Fprintf(p.Stdout, "    age: <public-key-from-above>\n")

	case "local":
		// PQ hybrid envelope via genesis-operator CLI
		if !genesisExistsFunc("genesis") {
			return fmt.Errorf("genesis CLI not found in PATH (required for --provider=local)\n" +
				"Install: https://github.com/LarsenClose/genesis-operator/releases")
		}

		envelopePath := filepath.Join(secretsDir, "master-key.enc")

		// Check if already initialized
		bootstrapPath := filepath.Join(secretsDir, "genesis-bootstrap.yaml")
		if _, err := os.Stat(bootstrapPath); err == nil {
			fmt.Fprintf(p.Stdout, "Genesis already initialized at %s\n", secretsDir)
			return nil
		}

		// genesis init --provider=local --envelope-path=<path> --output=<secretsDir>
		result, err := p.Exec.Run("genesis", "init",
			"--provider", "local",
			"--envelope-path", envelopePath,
			"--output", secretsDir,
		)
		if err != nil {
			return fmt.Errorf("genesis init failed: %w", err)
		}

		fmt.Fprintf(p.Stdout, "Genesis PQ initialized for cluster '%s'\n", p.Cluster)
		if result != nil && len(result.Stdout) > 0 {
			fmt.Fprintln(p.Stdout, result.Stdout)
		}

		// Read the generated .sops.yaml to extract the age public key
		clusterSopsPath := filepath.Join(secretsDir, ".sops.yaml")
		sopsData, err := os.ReadFile(filepath.Clean(clusterSopsPath))
		if err != nil {
			fmt.Fprintf(p.Stdout, "Warning: could not read generated .sops.yaml: %v\n", err)
			fmt.Fprintf(p.Stdout, "Manually add the age public key to %s/.sops.yaml\n", p.FleetDir)
		} else {
			fmt.Fprintf(p.Stdout, "\nGenerated SOPS config:\n%s\n", string(sopsData))
			fmt.Fprintf(p.Stdout, "Merge the age key into %s/.sops.yaml with path_regex: secrets/%s/.*\n",
				p.FleetDir, p.Cluster)
		}

	case "yubikey":
		fmt.Fprintln(p.Stdout, "YubiKey provider: genesis init will be called during bootstrap")
		fmt.Fprintln(p.Stdout, "Ensure your YubiKey is connected when running 'launch bootstrap'")

	case "oci-vault", "aws-kms", "gcp-kms":
		if p.KeyARN == "" {
			return fmt.Errorf("--key-arn is required for provider %s", p.Provider)
		}
		fmt.Fprintf(p.Stdout, "KMS key: %s\n", p.KeyARN)
		fmt.Fprintln(p.Stdout, "Genesis will use this key during bootstrap to envelope-encrypt the age key")

	default:
		return fmt.Errorf("unknown provider: %s (valid: local, age, yubikey, oci-vault, aws-kms, gcp-kms)", p.Provider)
	}

	// Write provider config for bootstrap to use later
	providerCfg := fmt.Sprintf("provider: %s\nkey_arn: %s\ncluster: %s\n", p.Provider, p.KeyARN, p.Cluster)
	cfgPath := filepath.Join(secretsDir, "genesis-config.yaml")
	if err := os.WriteFile(cfgPath, []byte(providerCfg), 0600); err != nil {
		return err
	}

	fmt.Fprintf(p.Stdout, "\nSecrets initialized for cluster '%s'\n", p.Cluster)
	fmt.Fprintf(p.Stdout, "Next: launch bootstrap --cluster %s\n", p.Cluster)
	return nil
}

func runSecretsInit(cmd *cobra.Command, args []string) error {
	runner := exec.NewRunner()
	cfg, _ := config.Load(cfgFile)
	p := paths.NewWithHome("", cfg)
	if home, err := os.UserHomeDir(); err == nil {
		p = paths.NewWithHome(home, cfg)
	}

	return SecretsInit(SecretsInitParams{
		Exec:     runner,
		Paths:    p,
		Stdout:   os.Stdout,
		Cluster:  secretsCluster,
		Provider: secretsProvider,
		KeyARN:   secretsKeyARN,
		FleetDir: secretsFleetDir,
	})
}
