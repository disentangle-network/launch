package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/disentangle-network/launch/internal/exec"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Configure credentials for the deployment pipeline",
	Long: `Interactively configures credentials needed by the pipeline stages.
Supports: OCI CLI, Cloudflare (via wrangler), GitHub CLI, and SOPS age keys.`,
	RunE: runSetup,
}

func init() {
	rootCmd.AddCommand(setupCmd)
}

func runSetup(cmd *cobra.Command, args []string) error {
	runner := exec.NewRunner()

	fmt.Println("==> Credential Setup")
	fmt.Println()

	// 1. OCI CLI
	fmt.Println("--- OCI CLI ---")
	if out, err := runner.RunSilent("oci", "iam", "region", "list", "--output", "json"); err == nil {
		_ = out
		fmt.Println("  OCI CLI already configured and authenticated.")
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
	if token := os.Getenv("CLOUDFLARE_API_TOKEN"); token != "" {
		fmt.Println("  CLOUDFLARE_API_TOKEN is set.")
	} else if exec.CommandExists("wrangler") {
		// Check if wrangler is already logged in
		out, err := runner.RunSilent("wrangler", "whoami")
		if err == nil && !strings.Contains(out.Stdout, "not authenticated") {
			fmt.Println("  Wrangler authenticated.")
			fmt.Println("  To set CLOUDFLARE_API_TOKEN, run: wrangler config")
			fmt.Println("  Or create an API token at: https://dash.cloudflare.com/profile/api-tokens")
		} else {
			fmt.Println("  Wrangler installed but not authenticated.")
			if confirm("Run 'wrangler login' to authenticate with Cloudflare?") {
				if _, err := runner.Run("wrangler", "login"); err != nil {
					fmt.Printf("  WARNING: wrangler login failed: %v\n", err)
				}
			}
		}
	} else {
		fmt.Println("  CLOUDFLARE_API_TOKEN not set and wrangler not installed.")
		fmt.Println("  Install wrangler: npm install -g wrangler")
		fmt.Println("  Or create an API token at: https://dash.cloudflare.com/profile/api-tokens")
	}
	fmt.Println()

	// 3. GitHub CLI
	fmt.Println("--- GitHub CLI ---")
	if out, err := runner.RunSilent("gh", "auth", "status"); err == nil {
		// Extract account info
		output := out.Stdout + out.Stderr
		for _, line := range strings.Split(output, "\n") {
			if strings.Contains(line, "Logged in") {
				fmt.Printf("  %s\n", strings.TrimSpace(line))
			}
		}
		// Check for repo scope
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

	// 5. OCI Vault (informational)
	fmt.Println("--- OCI Vault ---")
	if vaultOCID := os.Getenv("OCI_VAULT_KEY_OCID"); vaultOCID != "" {
		fmt.Printf("  OCI_VAULT_KEY_OCID is set: %s...%s\n", vaultOCID[:20], vaultOCID[len(vaultOCID)-10:])
	} else {
		fmt.Println("  OCI_VAULT_KEY_OCID not set.")
		fmt.Println("  To provision a vault:")
		fmt.Println("    oci kms management vault create --compartment-id <OCID> \\")
		fmt.Println("      --display-name launch-vault --vault-type DEFAULT")
		fmt.Println("  Then create a master key and set OCI_VAULT_KEY_OCID.")
		if exec.CommandExists("oci") {
			if confirm("List existing vaults in your compartment?") {
				// Try to get compartment from OCI config
				if _, err := runner.Run("oci", "kms", "management", "vault", "list", "--all", "--output", "table", "--query", "data[].{name:\"display-name\",state:\"lifecycle-state\",id:id}"); err != nil {
					fmt.Println("  Could not list vaults. Set --compartment-id or configure OCI CLI default compartment.")
				}
			}
		}
	}
	fmt.Println()

	fmt.Println("Setup complete. Run 'launch preflight' to verify all credentials.")
	return nil
}
