package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/disentangle-network/launch/internal/cloudflare"
	"github.com/disentangle-network/launch/internal/config"
	"github.com/disentangle-network/launch/internal/exec"
	"github.com/disentangle-network/launch/internal/hints"
	"github.com/spf13/cobra"
)

var (
	infraEnv string
	infraDir string
)

var infraCmd = &cobra.Command{
	Use:   "infra",
	Short: "Manage cluster infrastructure via OpenTofu",
	Long:  `Direct OpenTofu operations with automatic secret resolution from 1Password.`,
}

var infraInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Terraform providers",
	RunE:  runInfraInit,
}

var infraPlanCmd = &cobra.Command{
	Use:   "plan",
	Short: "Plan infrastructure changes",
	RunE:  runInfraPlan,
}

var infraApplyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply infrastructure changes",
	RunE:  runInfraApply,
}

var infraDestroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroy infrastructure (DANGEROUS)",
	RunE:  runInfraDestroy,
}

var infraOutputCmd = &cobra.Command{
	Use:   "output",
	Short: "Show Terraform outputs",
	RunE:  runInfraOutput,
}

var infraKubeconfigCmd = &cobra.Command{
	Use:   "kubeconfig",
	Short: "Fetch kubeconfig from OCI",
	RunE:  runInfraKubeconfig,
}

func init() {
	rootCmd.AddCommand(infraCmd)
	infraCmd.AddCommand(infraInitCmd)
	infraCmd.AddCommand(infraPlanCmd)
	infraCmd.AddCommand(infraApplyCmd)
	infraCmd.AddCommand(infraDestroyCmd)
	infraCmd.AddCommand(infraOutputCmd)
	infraCmd.AddCommand(infraKubeconfigCmd)

	infraCmd.PersistentFlags().StringVar(&infraEnv, "env", "dev", "Environment (dev, staging, prod)")
	infraCmd.PersistentFlags().StringVar(&infraDir, "dir", "", "Infrastructure repo root (default: auto-detect)")
}

func resolveInfraDir() (string, error) {
	if infraDir != "" {
		return infraDir, nil
	}

	cfg, _ := config.Load(cfgFile)
	if cfg != nil {
		if cfg.InfraDir != "" {
			return cfg.InfraDir, nil
		}
		if cfg.Repos.K8sOCIFoundation != "" {
			return cfg.Repos.K8sOCIFoundation, nil
		}
	}

	return "", fmt.Errorf("could not find k8s-oci-foundation repo (use --dir or set repos.k8s_oci_foundation in config)")
}

func resolveEnvDir() (string, error) {
	root, err := resolveInfraDir()
	if err != nil {
		return "", err
	}
	envDir := filepath.Join(root, "environments", infraEnv)
	if _, err := os.Stat(envDir); os.IsNotExist(err) {
		return "", fmt.Errorf("environment '%s' not found at %s", infraEnv, envDir)
	}
	return envDir, nil
}

func infraRunner() (*exec.Runner, error) {
	envDir, err := resolveEnvDir()
	if err != nil {
		return nil, err
	}

	runner := exec.NewRunner()
	runner.Dir = envDir
	runner.Verbose = verbose

	if dryRun {
		runner.DryRun = true
		runner.Env = append(runner.Env, "TF_VAR_cloudflare_api_token=<resolved-at-runtime>")
		fmt.Println("  [dry-run] Skipping secret resolution")
		return runner, nil
	}

	token, source, err := cloudflare.ResolveToken()
	if err != nil {
		return nil, fmt.Errorf("cloudflare token: %w", err)
	}
	fmt.Printf("  Using Cloudflare token from: %s\n", source)
	runner.Env = append(runner.Env, "TF_VAR_cloudflare_api_token="+token)

	return runner, nil
}

func runInfraInit(cmd *cobra.Command, args []string) error {
	runner, err := infraRunner()
	if err != nil {
		return err
	}
	fmt.Println("Initializing Terraform...")
	_, err = runner.Run("tofu", "init", "-upgrade")
	if err != nil {
		return err
	}
	hints.Print([]hints.NextStep{
		{Command: "infra plan", Description: "Preview infrastructure changes"},
	})
	return nil
}

func runInfraPlan(cmd *cobra.Command, args []string) error {
	runner, err := infraRunner()
	if err != nil {
		return err
	}

	fmt.Println("Initializing...")
	if _, err := runner.Run("tofu", "init", "-upgrade"); err != nil {
		return err
	}

	fmt.Println("\nPlanning...")
	_, err = runner.Run("tofu", "plan", "-var-file=terraform.tfvars", "-out=tfplan")
	if err != nil {
		return err
	}
	hints.Print([]hints.NextStep{
		{Command: "infra apply", Description: "Apply the plan"},
		{Command: "infra destroy", Description: "Tear down infrastructure"},
	})
	return nil
}

func runInfraApply(cmd *cobra.Command, args []string) error {
	runner, err := infraRunner()
	if err != nil {
		return err
	}

	envDir, _ := resolveEnvDir()
	planFile := filepath.Join(envDir, "tfplan")

	if _, err := os.Stat(planFile); os.IsNotExist(err) {
		fmt.Println("No plan found, running plan first...")
		if _, err := runner.Run("tofu", "plan", "-var-file=terraform.tfvars", "-out=tfplan"); err != nil {
			return err
		}
	}

	if !autoYes {
		fmt.Print("\nApply this plan? [y/N] ")
		var confirm string
		_, _ = fmt.Scanln(&confirm)
		if confirm != "y" && confirm != "Y" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	fmt.Println("Applying...")
	if _, err := runner.Run("tofu", "apply", "tfplan"); err != nil {
		return err
	}

	_ = os.Remove(planFile)

	fmt.Println("\nOutputs:")
	_, err = runner.Run("tofu", "output", "-json")
	if err != nil {
		return err
	}
	hints.Print([]hints.NextStep{
		{Command: "infra kubeconfig", Description: "Fetch kubeconfig from OCI"},
		{Command: "cluster add oci-dev", Description: "Add cluster to fleet"},
		{Command: "bootstrap --cluster oci-dev", Description: "Bootstrap FluxCD on cluster"},
	})
	return nil
}

func runInfraDestroy(cmd *cobra.Command, args []string) error {
	runner, err := infraRunner()
	if err != nil {
		return err
	}

	if !autoYes {
		fmt.Print("\n⚠️  DESTROY all infrastructure? This cannot be undone. [y/N] ")
		var confirm string
		_, _ = fmt.Scanln(&confirm)
		if confirm != "y" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	_, err = runner.Run("tofu", "destroy", "-var-file=terraform.tfvars")
	return err
}

func runInfraOutput(cmd *cobra.Command, args []string) error {
	runner, err := infraRunner()
	if err != nil {
		return err
	}
	_, err = runner.Run("tofu", "output", "-json")
	return err
}

func runInfraKubeconfig(cmd *cobra.Command, args []string) error {
	runner, err := infraRunner()
	if err != nil {
		return err
	}

	result, err := runner.RunSilent("tofu", "output", "-raw", "cluster_id")
	if err != nil {
		return fmt.Errorf("no cluster_id output found — has infra been applied? %w", err)
	}

	cfg, _ := config.Load(cfgFile)
	region := "us-phoenix-1"
	if cfg != nil && cfg.OCIRegion != "" {
		region = cfg.OCIRegion
	}

	infraRoot, _ := resolveInfraDir()
	kubeconfigPath := filepath.Join(infraRoot, "kubeconfig")

	_, err = runner.Run("oci", "ce", "cluster", "create-kubeconfig",
		"--cluster-id", result.Stdout,
		"--file", kubeconfigPath,
		"--region", region,
		"--token-version", "2.0.0",
		"--kube-endpoint", "PUBLIC_ENDPOINT",
	)
	if err != nil {
		return err
	}

	_ = os.Chmod(kubeconfigPath, 0600)
	fmt.Printf("Kubeconfig saved to %s\n", kubeconfigPath)
	hints.Print([]hints.NextStep{
		{Command: "cluster add oci-dev", Description: "Add cluster to fleet"},
		{Command: "secrets init --cluster oci-dev", Description: "Bootstrap secrets"},
		{Command: "bootstrap --cluster oci-dev", Description: "Bootstrap FluxCD"},
	})
	return nil
}
