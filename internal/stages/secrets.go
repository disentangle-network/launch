package stages

import (
	"fmt"
	"os"

	"github.com/disentangle-network/launch/internal/config"
	"github.com/disentangle-network/launch/internal/exec"
)

type SecretsResult struct {
	OperatorDeployed bool
	BootstrapApplied bool
}

func RunSecrets(cfg *config.Config, runner *exec.Runner) (*SecretsResult, error) {
	repoPath := cfg.Repos.GenesisOperator
	if repoPath == "" {
		return nil, fmt.Errorf("genesis_operator repo path not configured")
	}

	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("genesis-operator repo not found at %s", repoPath)
	}

	runner.Dir = repoPath
	result := &SecretsResult{}

	// Step 1: Initialize genesis
	fmt.Println("==> Initializing genesis-operator...")
	initArgs := []string{"init", "--provider=oci"}

	// Vault key: prefer config, fall back to env
	keyID := cfg.OCIVaultKeyOCID
	if keyID == "" {
		keyID = os.Getenv("OCI_VAULT_KEY_OCID")
	}
	if keyID != "" {
		initArgs = append(initArgs, "--key-id="+keyID)
	}

	endpoint := cfg.OCIVaultEndpoint
	if endpoint == "" {
		endpoint = os.Getenv("OCI_VAULT_ENDPOINT")
	}
	if endpoint != "" {
		initArgs = append(initArgs, "--endpoint="+endpoint)
	}

	if _, err := runner.Run("genesis", initArgs...); err != nil {
		return result, fmt.Errorf("genesis init failed: %w", err)
	}

	// Step 2: Deploy operator via Helm
	fmt.Println("==> Deploying genesis-operator via Helm...")
	helmArgs := []string{
		"upgrade", "--install", "genesis-operator",
		"./charts/genesis-operator",
		"--namespace", "genesis-system",
		"--create-namespace",
	}
	if cfg.GenesisImage != "" {
		helmArgs = append(helmArgs, "--set", fmt.Sprintf("image.repository=%s", cfg.GenesisImage))
	}
	if cfg.GenesisVersion != "" {
		helmArgs = append(helmArgs, "--set", fmt.Sprintf("image.tag=%s", cfg.GenesisVersion))
	}

	if _, err := runner.Run("helm", helmArgs...); err != nil {
		return result, fmt.Errorf("helm install genesis-operator failed: %w", err)
	}
	result.OperatorDeployed = true

	// Step 3: Verify
	fmt.Println("==> Verifying genesis-operator deployment...")
	if _, err := runner.Run("kubectl", "get", "pods", "-n", "genesis-system"); err != nil {
		fmt.Println("WARNING: could not verify genesis-operator pods")
	}

	result.BootstrapApplied = true
	return result, nil
}
