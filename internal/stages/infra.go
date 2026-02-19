package stages

import (
	"fmt"
	"os"

	"github.com/disentangle-network/launch/internal/config"
	"github.com/disentangle-network/launch/internal/exec"
)

type InfraAction string

const (
	InfraPlan  InfraAction = "plan"
	InfraApply InfraAction = "apply"
)

type InfraResult struct {
	Action    InfraAction
	Planned   bool
	Applied   bool
	FluxReady bool
}

func RunInfra(cfg *config.Config, runner *exec.Runner, action InfraAction) (*InfraResult, error) {
	repoPath := cfg.Repos.K8sOCIFoundation
	if repoPath == "" {
		return nil, fmt.Errorf("k8s_oci_foundation repo path not configured")
	}

	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("k8s-oci-foundation repo not found at %s", repoPath)
	}

	runner.Dir = repoPath
	env := cfg.Environment
	if env == "" {
		env = "dev"
	}

	result := &InfraResult{Action: action}

	// Step 1: Initialize (generates age key for SOPS)
	fmt.Println("==> Initializing infrastructure tooling...")
	if _, err := runner.Run("task", "init"); err != nil {
		return result, fmt.Errorf("task init failed: %w", err)
	}

	// Step 2: Plan
	fmt.Printf("==> Running terraform plan for environment '%s'...\n", env)
	if _, err := runner.Run("task", "plan", fmt.Sprintf("ENV=%s", env)); err != nil {
		return result, fmt.Errorf("terraform plan failed: %w", err)
	}
	result.Planned = true

	if action == InfraPlan {
		return result, nil
	}

	// Step 3: Apply
	fmt.Printf("==> Applying terraform for environment '%s'...\n", env)
	if _, err := runner.Run("task", "apply", fmt.Sprintf("ENV=%s", env)); err != nil {
		return result, fmt.Errorf("terraform apply failed: %w", err)
	}
	result.Applied = true

	// Step 4: Bootstrap FluxCD
	fmt.Println("==> Bootstrapping FluxCD...")
	if _, err := runner.Run("task", "bootstrap:flux"); err != nil {
		return result, fmt.Errorf("flux bootstrap failed: %w", err)
	}
	result.FluxReady = true

	// Postcondition: verify nodes
	fmt.Println("==> Verifying cluster nodes...")
	if _, err := runner.Run("kubectl", "get", "nodes"); err != nil {
		fmt.Println("WARNING: kubectl get nodes failed -- cluster may still be provisioning")
	}

	return result, nil
}
