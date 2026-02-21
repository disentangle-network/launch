package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/disentangle-network/launch/internal/cloudflare"
	"github.com/disentangle-network/launch/internal/config"
	"github.com/disentangle-network/launch/internal/exec"
	"github.com/disentangle-network/launch/internal/hints"
	"github.com/disentangle-network/launch/internal/paths"
	"github.com/spf13/cobra"
)

// SetupParams holds dependencies for the setup command.
type SetupParams struct {
	Exec        exec.Executor
	Paths       *paths.Resolver
	Stdout      io.Writer
	CfgFile     string
	ConfirmFunc func(string) bool
}

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
	runner.Verbose = verbose
	runner.DryRun = dryRun

	resolver, err := paths.New(nil)
	if err != nil {
		return fmt.Errorf("resolving paths: %w", err)
	}

	return Setup(SetupParams{
		Exec:        runner,
		Paths:       resolver,
		Stdout:      os.Stdout,
		CfgFile:     cfgFile,
		ConfirmFunc: confirm,
	})
}

// Setup interactively configures credentials for the deployment pipeline.
func Setup(p SetupParams) error {
	cfg, err := config.Load(p.CfgFile)
	if err != nil {
		cfg = &config.Config{}
	}
	configChanged := false

	fmt.Fprintln(p.Stdout, "==> Credential Setup")
	fmt.Fprintln(p.Stdout)

	// 1. OCI CLI
	fmt.Fprintln(p.Stdout, "--- OCI CLI ---")
	ociAvailable := false
	if _, err := p.Exec.RunSilent("oci", "iam", "region", "list", "--output", "json"); err == nil {
		ociAvailable = true
		fmt.Fprintln(p.Stdout, "  OCI CLI configured and authenticated.")
		// Auto-discover compartment ID if not set
		if cfg.OCICompartmentID == "" {
			if out, err := p.Exec.RunSilent("oci", "iam", "compartment", "list",
				"--query", "data[0].\"compartment-id\"", "--raw-output"); err == nil {
				tenancy := strings.TrimSpace(out.Stdout)
				if tenancy != "" {
					cfg.OCICompartmentID = tenancy
					configChanged = true
					fmt.Fprintf(p.Stdout, "  Auto-detected tenancy: %s\n", tenancy)
				}
			}
		}
	} else {
		fmt.Fprintln(p.Stdout, "  OCI CLI not configured.")
		if p.ConfirmFunc("Run 'oci setup config' to configure OCI credentials?") {
			if _, err := p.Exec.Run("oci", "setup", "config"); err != nil {
				fmt.Fprintf(p.Stdout, "  WARNING: oci setup config failed: %v\n", err)
			}
		}
	}
	fmt.Fprintln(p.Stdout)

	// 2. Cloudflare via wrangler
	fmt.Fprintln(p.Stdout, "--- Cloudflare ---")
	if token, source, err := cloudflare.ResolveToken(); err == nil {
		fmt.Fprintf(p.Stdout, "  Cloudflare token resolved via %s.\n", source)
		_ = token // token is used at runtime, not persisted
		// Extract account ID if not already set
		if cfg.CloudflareAccountID == "" {
			if acctID := cloudflare.ResolveAccountID(); acctID != "" {
				cfg.CloudflareAccountID = acctID
				configChanged = true
				fmt.Fprintf(p.Stdout, "  Auto-detected account ID: %s\n", acctID)
			}
		} else {
			fmt.Fprintf(p.Stdout, "  Account ID: %s\n", cfg.CloudflareAccountID)
		}
	} else if exec.CommandExists("wrangler") {
		fmt.Fprintln(p.Stdout, "  Wrangler installed but not authenticated.")
		if p.ConfirmFunc("Run 'wrangler login' to authenticate with Cloudflare?") {
			if _, err := p.Exec.Run("wrangler", "login"); err != nil {
				fmt.Fprintf(p.Stdout, "  WARNING: wrangler login failed: %v\n", err)
			}
		}
	} else {
		fmt.Fprintln(p.Stdout, "  Cloudflare not configured: "+err.Error())
		fmt.Fprintln(p.Stdout, "  Install wrangler: npm install -g wrangler")
	}
	fmt.Fprintln(p.Stdout)

	// 3. GitHub CLI
	fmt.Fprintln(p.Stdout, "--- GitHub CLI ---")
	if out, err := p.Exec.RunSilent("gh", "auth", "status"); err == nil {
		output := out.Stdout + out.Stderr
		for _, line := range strings.Split(output, "\n") {
			if strings.Contains(line, "Logged in") {
				fmt.Fprintf(p.Stdout, "  %s\n", strings.TrimSpace(line))
			}
		}
		if !strings.Contains(output, "repo") {
			fmt.Fprintln(p.Stdout, "  WARNING: 'repo' scope may be missing. FluxCD bootstrap requires it.")
			fmt.Fprintln(p.Stdout, "  Run: gh auth refresh -s repo")
		}
	} else {
		fmt.Fprintln(p.Stdout, "  GitHub CLI not authenticated.")
		if p.ConfirmFunc("Run 'gh auth login' to authenticate?") {
			if _, err := p.Exec.Run("gh", "auth", "login"); err != nil {
				fmt.Fprintf(p.Stdout, "  WARNING: gh auth login failed: %v\n", err)
			}
		}
	}
	fmt.Fprintln(p.Stdout)

	// 4. SOPS Age Key
	fmt.Fprintln(p.Stdout, "--- SOPS Age Key ---")
	keyFile := p.Paths.AgeKeyFile()
	if _, err := os.Stat(keyFile); err == nil {
		fmt.Fprintf(p.Stdout, "  Age key found at %s\n", keyFile)
	} else {
		fmt.Fprintf(p.Stdout, "  No age key at %s\n", keyFile)
		if exec.CommandExists("age-keygen") {
			if p.ConfirmFunc("Generate a new age key?") {
				if err := os.MkdirAll(keyFile[:strings.LastIndex(keyFile, "/")], 0o700); err != nil {
					return fmt.Errorf("creating key directory: %w", err)
				}
				if _, err := p.Exec.Run("sh", "-c", "age-keygen -o "+keyFile); err != nil {
					fmt.Fprintf(p.Stdout, "  WARNING: age-keygen failed: %v\n", err)
				} else {
					fmt.Fprintf(p.Stdout, "  Age key generated at %s\n", keyFile)
				}
			}
		} else {
			fmt.Fprintln(p.Stdout, "  Install age: brew install age")
		}
	}
	fmt.Fprintln(p.Stdout)

	// 5. OCI Vault -- auto-discover and configure
	fmt.Fprintln(p.Stdout, "--- OCI Vault ---")
	if cfg.OCIVaultKeyOCID != "" {
		fmt.Fprintf(p.Stdout, "  Vault key configured: ...%s\n", cfg.OCIVaultKeyOCID[max(0, len(cfg.OCIVaultKeyOCID)-20):])
	} else if ociAvailable && cfg.OCICompartmentID != "" {
		fmt.Fprintln(p.Stdout, "  Discovering OCI vaults...")
		if err := discoverVault(p.Exec, p.Stdout, p.ConfirmFunc, cfg); err != nil {
			fmt.Fprintf(p.Stdout, "  Could not auto-discover vault: %v\n", err)
			fmt.Fprintln(p.Stdout, "  To provision manually:")
			fmt.Fprintln(p.Stdout, "    oci kms management vault create --compartment-id <OCID> \\")
			fmt.Fprintln(p.Stdout, "      --display-name launch-vault --vault-type DEFAULT")
		} else if cfg.OCIVaultKeyOCID != "" {
			configChanged = true
		}
	} else {
		fmt.Fprintln(p.Stdout, "  OCI CLI not available or compartment not set; skipping vault discovery.")
	}
	fmt.Fprintln(p.Stdout)

	// Save config if anything changed
	if configChanged {
		if err := config.Save(cfg, p.CfgFile); err != nil {
			fmt.Fprintf(p.Stdout, "  WARNING: could not save config: %v\n", err)
		} else {
			path := p.CfgFile
			if path == "" {
				path, _ = config.DefaultConfigPath()
			}
			fmt.Fprintf(p.Stdout, "==> Config saved to %s\n\n", path)
		}
	}

	fmt.Fprintln(p.Stdout, "Setup complete.")
	hints.Fprint(p.Stdout, []hints.NextStep{
		{Command: "preflight", Description: "Verify all tools and credentials"},
		{Command: "infra plan", Description: "Preview OCI infrastructure"},
	})
	return nil
}

// discoverVault finds existing OCI vaults and keys, prompts for selection if
// multiple exist, and populates the config.
func discoverVault(e exec.Executor, w io.Writer, confirmFn func(string) bool, cfg *config.Config) error {
	// List vaults
	out, err := e.RunSilent("oci", "kms", "management", "vault", "list",
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
		fmt.Fprintln(w, "  No active vaults found.")
		if confirmFn("Create a new vault named 'launch-vault'?") {
			createOut, err := e.RunSilent("oci", "kms", "management", "vault", "create",
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
			fmt.Fprintln(w, "  Vault created.")
		} else {
			return nil
		}
	}

	// Select vault (use first if only one)
	vault := vaults[0]
	if len(vaults) > 1 {
		fmt.Fprintln(w, "  Available vaults:")
		for i, v := range vaults {
			fmt.Fprintf(w, "    [%d] %s (%s)\n", i+1, v.DisplayName, v.ID[len(v.ID)-20:])
		}
		fmt.Fprintf(w, "  Using vault: %s\n", vault.DisplayName)
	} else {
		fmt.Fprintf(w, "  Found vault: %s\n", vault.DisplayName)
	}

	cfg.OCIVaultID = vault.ID
	cfg.OCIVaultEndpoint = vault.ManagementEndpoint

	// List keys in the vault
	keyOut, err := e.RunSilent("oci", "kms", "management", "key", "list",
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
		fmt.Fprintln(w, "  No enabled keys found in vault.")
		if confirmFn("Create a new AES-256 master key named 'launch-key'?") {
			createKeyOut, err := e.RunSilent("oci", "kms", "management", "key", "create",
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
			fmt.Fprintf(w, "  Key created: %s\n", createdKey.Data.ID)
		}
		return nil
	}

	// Select key (use first if only one)
	key := keys[0]
	if len(keys) > 1 {
		fmt.Fprintln(w, "  Available keys:")
		for i, k := range keys {
			fmt.Fprintf(w, "    [%d] %s (%s, %s)\n", i+1, k.DisplayName, k.Algorithm, k.ID[len(k.ID)-20:])
		}
		fmt.Fprintf(w, "  Using key: %s\n", key.DisplayName)
	} else {
		fmt.Fprintf(w, "  Found key: %s (%s)\n", key.DisplayName, key.Algorithm)
	}

	cfg.OCIVaultKeyOCID = key.ID
	fmt.Fprintf(w, "  Vault key OCID saved to config.\n")
	return nil
}
