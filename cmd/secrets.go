package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/disentangle-network/launch/internal/config"
	"github.com/disentangle-network/launch/internal/exec"
	"github.com/disentangle-network/launch/internal/paths"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
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
	Example: `  launch-disentangle secrets init --cluster edge-1
  launch-disentangle secrets init --cluster prod-1 --provider oci-vault --key-arn <ocid>`,
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
	// Warn if cluster directory doesn't exist yet (secrets init can run before cluster add)
	clusterDir := p.Paths.FleetClusterDir(p.FleetDir, p.Cluster)
	if _, err := os.Stat(clusterDir); os.IsNotExist(err) {
		fmt.Fprintf(p.Stdout, "Note: cluster '%s' not yet added to fleet (run 'launch cluster add %s' when ready)\n", p.Cluster, p.Cluster)
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

		// Auto-update .sops.yaml
		pubKey := extractAgePublicKey(result)
		if pubKey != "" {
			if err := appendSOPSRule(p.FleetDir, p.Cluster, pubKey); err != nil {
				fmt.Fprintf(p.Stdout, "Warning: could not update .sops.yaml: %v\n", err)
				fmt.Fprintf(p.Stdout, "Manually add to %s/.sops.yaml: path_regex: secrets/%s/.*, age: %s\n",
					p.FleetDir, p.Cluster, pubKey)
			} else {
				fmt.Fprintf(p.Stdout, "Updated %s/.sops.yaml with creation rule for cluster '%s'\n", p.FleetDir, p.Cluster)
			}
		} else {
			fmt.Fprintf(p.Stdout, "\nCould not extract public key from age-keygen output\n")
			fmt.Fprintf(p.Stdout, "Manually add to %s/.sops.yaml: path_regex: secrets/%s/.*\n",
				p.FleetDir, p.Cluster)
		}

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

		// Auto-update fleet root .sops.yaml from genesis output
		clusterSopsPath := filepath.Join(secretsDir, ".sops.yaml")
		sopsData, err := os.ReadFile(filepath.Clean(clusterSopsPath))
		if err != nil {
			fmt.Fprintf(p.Stdout, "Warning: could not read generated .sops.yaml: %v\n", err)
			fmt.Fprintf(p.Stdout, "Manually add the age public key to %s/.sops.yaml\n", p.FleetDir)
		} else {
			// Extract age key from genesis-generated sops config
			var genCfg sopsConfig
			if parseErr := yaml.Unmarshal(sopsData, &genCfg); parseErr == nil {
				for _, rule := range genCfg.CreationRules {
					if rule.Age != "" {
						if appendErr := appendSOPSRule(p.FleetDir, p.Cluster, rule.Age); appendErr != nil {
							fmt.Fprintf(p.Stdout, "Warning: could not update .sops.yaml: %v\n", appendErr)
						} else {
							fmt.Fprintf(p.Stdout, "Updated %s/.sops.yaml with creation rule for cluster '%s'\n",
								p.FleetDir, p.Cluster)
						}
						break
					}
				}
			} else {
				fmt.Fprintf(p.Stdout, "Warning: could not parse generated .sops.yaml: %v\n", parseErr)
				fmt.Fprintf(p.Stdout, "Manually merge into %s/.sops.yaml\n", p.FleetDir)
			}
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

// sopsConfig represents a .sops.yaml file structure.
type sopsConfig struct {
	CreationRules []sopsRule `yaml:"creation_rules"`
}

type sopsRule struct {
	PathRegex string `yaml:"path_regex"`
	Age       string `yaml:"age,omitempty"`
}

// appendSOPSRule adds a creation rule to the fleet root .sops.yaml.
// If the file doesn't exist, it creates one. If a rule with the same
// path_regex already exists, it updates the age key.
func appendSOPSRule(fleetDir, cluster, agePublicKey string) error {
	sopsPath := filepath.Join(fleetDir, ".sops.yaml")

	var cfg sopsConfig
	data, err := os.ReadFile(filepath.Clean(sopsPath))
	if err == nil {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("failed to parse %s: %w", sopsPath, err)
		}
	}

	pathRegex := fmt.Sprintf("secrets/%s/.*", cluster)

	// Check if rule already exists
	found := false
	for i, rule := range cfg.CreationRules {
		if rule.PathRegex == pathRegex {
			cfg.CreationRules[i].Age = agePublicKey
			found = true
			break
		}
	}
	if !found {
		cfg.CreationRules = append(cfg.CreationRules, sopsRule{
			PathRegex: pathRegex,
			Age:       agePublicKey,
		})
	}

	out, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal .sops.yaml: %w", err)
	}

	return os.WriteFile(sopsPath, out, 0600)
}

// extractAgePublicKey parses the age public key from age-keygen output.
// age-keygen outputs: "# public key: age1..." on stderr or stdout.
func extractAgePublicKey(result *exec.Result) string {
	if result == nil {
		return ""
	}
	for _, output := range []string{result.Stdout, result.Stderr} {
		for _, line := range strings.Split(output, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "# public key: ") {
				return strings.TrimPrefix(line, "# public key: ")
			}
			// Also handle bare public key output
			if strings.HasPrefix(line, "age1") && !strings.Contains(line, " ") {
				return line
			}
		}
	}
	return ""
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
