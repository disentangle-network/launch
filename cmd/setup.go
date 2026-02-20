package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/disentangle-network/launch/internal/cloudflare"
	"github.com/disentangle-network/launch/internal/config"
	"github.com/disentangle-network/launch/internal/exec"
	"github.com/disentangle-network/launch/internal/hints"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Configure credentials for the deployment pipeline",
	Long: `Interactively configures credentials needed for fleet management.
Supports: OCI CLI, Cloudflare, GitHub CLI, and SOPS age keys.`,
	RunE: runSetup,
}

func init() {
	rootCmd.AddCommand(setupCmd)
}

func runSetup(cmd *cobra.Command, args []string) error {
	runner := exec.NewRunner()
	cfg, err := config.Load(cfgFile)
	if err != nil {
		cfg = &config.Config{}
	}
	configChanged := false

	fmt.Println("==> Credential Setup")
	fmt.Println()

	// 1. OCI CLI
	fmt.Println("--- OCI CLI ---")
	if _, err := runner.RunSilent("oci", "iam", "region", "list", "--output", "json"); err == nil {
		fmt.Println("  OCI CLI configured and authenticated.")
		// Auto-discover compartment ID if not set
		if cfg.OCICompartmentID == "" {
			if out, err := runner.RunSilent("oci", "iam", "compartment", "list",
				"--query", "data[0].\"compartment-id\"", "--raw-output"); err == nil {
				tenancy := strings.TrimSpace(out.Stdout)
				if tenancy != "" {
					cfg.OCICompartmentID = tenancy
					configChanged = true
					fmt.Printf("  Auto-detected tenancy: %s\n", tenancy)
				}
			}
		}
	} else {
		fmt.Println("  OCI CLI not configured.")
		if confirm("Run 'oci setup config' to configure OCI credentials?") {
			if _, err := runner.Run("oci", "setup", "config"); err != nil {
				fmt.Printf("  WARNING: oci setup config failed: %v\n", err)
			}
		}
	}
	fmt.Println()

	// 2. Cloudflare via wrangler
	fmt.Println("--- Cloudflare ---")
	if token, source, err := cloudflare.ResolveToken(); err == nil {
		fmt.Printf("  Cloudflare token resolved via %s.\n", source)
		_ = token // token is used at runtime, not persisted
		// Extract account ID if not already set
		if cfg.CloudflareAccountID == "" {
			if acctID := cloudflare.ResolveAccountID(); acctID != "" {
				cfg.CloudflareAccountID = acctID
				configChanged = true
				fmt.Printf("  Auto-detected account ID: %s\n", acctID)
			}
		} else {
			fmt.Printf("  Account ID: %s\n", cfg.CloudflareAccountID)
		}
	} else if exec.CommandExists("wrangler") {
		fmt.Println("  Wrangler installed but not authenticated.")
		if confirm("Run 'wrangler login' to authenticate with Cloudflare?") {
			if _, err := runner.Run("wrangler", "login"); err != nil {
				fmt.Printf("  WARNING: wrangler login failed: %v\n", err)
			}
		}
	} else {
		fmt.Println("  Cloudflare not configured: " + err.Error())
		fmt.Println("  Install wrangler: npm install -g wrangler")
	}
	fmt.Println()

	// 3. GitHub CLI
	fmt.Println("--- GitHub CLI ---")
	if out, err := runner.RunSilent("gh", "auth", "status"); err == nil {
		output := out.Stdout + out.Stderr
		for _, line := range strings.Split(output, "\n") {
			if strings.Contains(line, "Logged in") {
				fmt.Printf("  %s\n", strings.TrimSpace(line))
			}
		}
		if !strings.Contains(output, "repo") {
			fmt.Println("  WARNING: 'repo' scope may be missing. FluxCD bootstrap requires it.")
			fmt.Println("  Run: gh auth refresh -s repo")
		}
	} else {
		fmt.Println("  GitHub CLI not authenticated.")
		if confirm("Run 'gh auth login' to authenticate?") {
			if _, err := runner.Run("gh", "auth", "login"); err != nil {
				fmt.Printf("  WARNING: gh auth login failed: %v\n", err)
			}
		}
	}
	fmt.Println()

	// 4. SOPS Age Key
	fmt.Println("--- SOPS Age Key ---")
	keyFile := os.Getenv("SOPS_AGE_KEY_FILE")
	if keyFile == "" {
		home, _ := os.UserHomeDir()
		keyFile = home + "/.config/sops/age/keys.txt"
	}
	if _, err := os.Stat(keyFile); err == nil {
		fmt.Printf("  Age key found at %s\n", keyFile)
	} else {
		fmt.Printf("  No age key at %s\n", keyFile)
		if exec.CommandExists("age-keygen") {
			if confirm("Generate a new age key?") {
				if err := os.MkdirAll(keyFile[:strings.LastIndex(keyFile, "/")], 0o700); err != nil {
					return fmt.Errorf("creating key directory: %w", err)
				}
				if _, err := runner.Run("sh", "-c", "age-keygen -o "+keyFile); err != nil {
					fmt.Printf("  WARNING: age-keygen failed: %v\n", err)
				} else {
					fmt.Printf("  Age key generated at %s\n", keyFile)
				}
			}
		} else {
			fmt.Println("  Install age: brew install age")
		}
	}
	fmt.Println()

	// 5. OCI Vault -- auto-discover and configure
	fmt.Println("--- OCI Vault ---")
	if cfg.OCIVaultKeyOCID != "" {
		fmt.Printf("  Vault key configured: ...%s\n", cfg.OCIVaultKeyOCID[max(0, len(cfg.OCIVaultKeyOCID)-20):])
	} else if exec.CommandExists("oci") && cfg.OCICompartmentID != "" {
		fmt.Println("  Discovering OCI vaults...")
		if err := discoverVault(runner, cfg); err != nil {
			fmt.Printf("  Could not auto-discover vault: %v\n", err)
			fmt.Println("  To provision manually:")
			fmt.Println("    oci kms management vault create --compartment-id <OCID> \\")
			fmt.Println("      --display-name launch-vault --vault-type DEFAULT")
		} else if cfg.OCIVaultKeyOCID != "" {
			configChanged = true
		}
	} else {
		fmt.Println("  OCI CLI not available or compartment not set; skipping vault discovery.")
	}
	fmt.Println()

	// Save config if anything changed
	if configChanged {
		if err := config.Save(cfg, cfgFile); err != nil {
			fmt.Printf("  WARNING: could not save config: %v\n", err)
		} else {
			path := cfgFile
			if path == "" {
				path, _ = config.DefaultConfigPath()
			}
			fmt.Printf("==> Config saved to %s\n\n", path)
		}
	}

	fmt.Println("Setup complete.")
	hints.Print([]hints.NextStep{
		{Command: "preflight", Description: "Verify all tools and credentials"},
		{Command: "infra plan", Description: "Preview OCI infrastructure"},
	})
	return nil
}

// discoverVault finds existing OCI vaults and keys, prompts for selection if
// multiple exist, and populates the config.
func discoverVault(runner *exec.Runner, cfg *config.Config) error {
	// List vaults
	out, err := runner.RunSilent("oci", "kms", "management", "vault", "list",
		"--all", "--compartment-id", cfg.OCICompartmentID,
		"--output", "json",
		"--query", `data[?\"lifecycle-state\"=='ACTIVE']`)
	if err != nil {
		return fmt.Errorf("listing vaults: %w", err)
	}

	var vaults []struct {
		ID                 string `json:"id"`
		DisplayName        string `json:"display-name"`
		ManagementEndpoint string `json:"management-endpoint"`
	}
	if err := json.Unmarshal([]byte(out.Stdout), &vaults); err != nil {
		return fmt.Errorf("parsing vault list: %w", err)
	}

	if len(vaults) == 0 {
		fmt.Println("  No active vaults found.")
		if confirm("Create a new vault named 'launch-vault'?") {
			createOut, err := runner.RunSilent("oci", "kms", "management", "vault", "create",
				"--compartment-id", cfg.OCICompartmentID,
				"--display-name", "launch-vault",
				"--vault-type", "DEFAULT",
				"--output", "json",
				"--wait-for-state", "ACTIVE")
			if err != nil {
				return fmt.Errorf("creating vault: %w", err)
			}
			var created struct {
				Data struct {
					ID                 string `json:"id"`
					ManagementEndpoint string `json:"management-endpoint"`
				} `json:"data"`
			}
			if err := json.Unmarshal([]byte(createOut.Stdout), &created); err != nil {
				return fmt.Errorf("parsing created vault: %w", err)
			}
			vaults = append(vaults, struct {
				ID                 string `json:"id"`
				DisplayName        string `json:"display-name"`
				ManagementEndpoint string `json:"management-endpoint"`
			}{
				ID:                 created.Data.ID,
				DisplayName:        "launch-vault",
				ManagementEndpoint: created.Data.ManagementEndpoint,
			})
			fmt.Println("  Vault created.")
		} else {
			return nil
		}
	}

	// Select vault (use first if only one)
	vault := vaults[0]
	if len(vaults) > 1 {
		fmt.Println("  Available vaults:")
		for i, v := range vaults {
			fmt.Printf("    [%d] %s (%s)\n", i+1, v.DisplayName, v.ID[len(v.ID)-20:])
		}
		fmt.Printf("  Using vault: %s\n", vault.DisplayName)
	} else {
		fmt.Printf("  Found vault: %s\n", vault.DisplayName)
	}

	cfg.OCIVaultID = vault.ID
	cfg.OCIVaultEndpoint = vault.ManagementEndpoint

	// List keys in the vault
	keyOut, err := runner.RunSilent("oci", "kms", "management", "key", "list",
		"--all", "--compartment-id", cfg.OCICompartmentID,
		"--endpoint", vault.ManagementEndpoint,
		"--output", "json",
		"--query", `data[?\"lifecycle-state\"=='ENABLED']`)
	if err != nil {
		return fmt.Errorf("listing keys: %w", err)
	}

	var keys []struct {
		ID          string `json:"id"`
		DisplayName string `json:"display-name"`
		Algorithm   string `json:"algorithm"`
	}
	if err := json.Unmarshal([]byte(keyOut.Stdout), &keys); err != nil {
		return fmt.Errorf("parsing key list: %w", err)
	}

	if len(keys) == 0 {
		fmt.Println("  No enabled keys found in vault.")
		if confirm("Create a new AES-256 master key named 'launch-key'?") {
			createKeyOut, err := runner.RunSilent("oci", "kms", "management", "key", "create",
				"--compartment-id", cfg.OCICompartmentID,
				"--endpoint", vault.ManagementEndpoint,
				"--display-name", "launch-key",
				"--key-shape", `{"algorithm":"AES","length":32}`,
				"--output", "json",
				"--wait-for-state", "ENABLED")
			if err != nil {
				return fmt.Errorf("creating key: %w", err)
			}
			var createdKey struct {
				Data struct {
					ID string `json:"id"`
				} `json:"data"`
			}
			if err := json.Unmarshal([]byte(createKeyOut.Stdout), &createdKey); err != nil {
				return fmt.Errorf("parsing created key: %w", err)
			}
			cfg.OCIVaultKeyOCID = createdKey.Data.ID
			fmt.Printf("  Key created: %s\n", createdKey.Data.ID)
		}
		return nil
	}

	// Select key (use first if only one)
	key := keys[0]
	if len(keys) > 1 {
		fmt.Println("  Available keys:")
		for i, k := range keys {
			fmt.Printf("    [%d] %s (%s, %s)\n", i+1, k.DisplayName, k.Algorithm, k.ID[len(k.ID)-20:])
		}
		fmt.Printf("  Using key: %s\n", key.DisplayName)
	} else {
		fmt.Printf("  Found key: %s (%s)\n", key.DisplayName, key.Algorithm)
	}

	cfg.OCIVaultKeyOCID = key.ID
	fmt.Printf("  Vault key OCID saved to config.\n")
	return nil
}
