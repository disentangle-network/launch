package stages

import (
	"fmt"
	"os"

	"github.com/disentangle-network/launch/internal/config"
	"github.com/disentangle-network/launch/internal/exec"
)

type DeployResult struct {
	ImageVerified bool
	ConfigApplied bool
	NodesDeployed bool
}

func RunDeploy(cfg *config.Config, runner *exec.Runner) (*DeployResult, error) {
	repoPath := cfg.Repos.Deploy
	if repoPath == "" {
		return nil, fmt.Errorf("deploy repo path not configured")
	}

	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("deploy repo not found at %s", repoPath)
	}

	runner.Dir = repoPath
	result := &DeployResult{}

	// Step 1: Verify container image exists
	image := cfg.ProtocolImage
	version := cfg.ProtocolVersion
	if image == "" {
		image = "ghcr.io/disentangle-network/disentangle-node"
	}
	if version == "" {
		version = "latest"
	}

	fmt.Printf("==> Verifying image %s:%s exists...\n", image, version)
	_, err := runner.RunSilent("docker", "manifest", "inspect", fmt.Sprintf("%s:%s", image, version))
	if err != nil {
		fmt.Printf("WARNING: could not verify image %s:%s -- continuing anyway\n", image, version)
	} else {
		result.ImageVerified = true
	}

	// Step 2: Apply cluster-settings ConfigMap if it exists
	configMapPath := repoPath + "/cluster-settings.yaml"
	if _, err := os.Stat(configMapPath); err == nil {
		fmt.Println("==> Applying cluster-settings ConfigMap...")
		if _, err := runner.Run("kubectl", "apply", "-f", configMapPath); err != nil {
			fmt.Printf("WARNING: could not apply cluster-settings: %v\n", err)
		} else {
			result.ConfigApplied = true
		}
	}

	// Step 3: Deploy via Helm
	fmt.Println("==> Deploying disentangle-node via Helm...")
	helmArgs := []string{
		"upgrade", "--install", "disentangle",
		"./helm/disentangle",
		"--namespace", "disentangle",
		"--create-namespace",
		"--set", fmt.Sprintf("image.repository=%s", image),
		"--set", fmt.Sprintf("image.tag=%s", version),
	}

	if _, err := runner.Run("helm", helmArgs...); err != nil {
		// Fallback: try FluxCD reconciliation
		fmt.Println("==> Helm install failed, triggering FluxCD reconciliation...")
		if _, err := runner.Run("flux", "reconcile", "helmrelease", "disentangle", "-n", "disentangle"); err != nil {
			return result, fmt.Errorf("deploy failed (both helm and flux): %w", err)
		}
	}
	result.NodesDeployed = true

	// Step 4: Verify pods
	fmt.Println("==> Verifying disentangle-node pods...")
	if _, err := runner.Run("kubectl", "get", "pods", "-n", "disentangle"); err != nil {
		fmt.Println("WARNING: could not verify disentangle-node pods")
	}

	return result, nil
}
