package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/disentangle-network/launch/internal/cloudflare"
	"github.com/disentangle-network/launch/internal/config"
	"github.com/disentangle-network/launch/internal/exec"
	"github.com/disentangle-network/launch/internal/hints"
	"github.com/disentangle-network/launch/internal/paths"
	"github.com/spf13/cobra"
)

var (
	infraEnv string
	infraDir string
)

// InfraParams holds all injectable dependencies for testable infra functions.
type InfraParams struct {
	Exec              exec.Executor
	Paths             *paths.Resolver
	Stdout            io.Writer
	Env               string
	Dir               string // flag override for infra dir
	CfgFile           string
	DryRun            bool
	Verbose           bool
	AutoYes           bool
	ConfirmFunc       func(string) bool
	TokenResolverFunc func() (string, string, error)
	Region            string // OCI region for kubeconfig
}

var infraCmd = &cobra.Command{
	Use:   "infra",
	Short: "Manage cluster infrastructure via OpenTofu",
	Long:  `Direct OpenTofu operations with automatic secret resolution from 1Password.`,
}

var infraInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Terraform providers",
	Long: `Download and initialize Terraform/OpenTofu providers for the target environment.
Runs tofu init in the environment's infrastructure directory.`,
	Example: `  launch-disentangle infra init
  launch-disentangle infra init --env prod`,
	RunE: runInfraInit,
}

var infraPlanCmd = &cobra.Command{
	Use:   "plan",
	Short: "Plan infrastructure changes",
	Long: `Generate and display a Terraform execution plan without applying changes.
Automatically resolves secrets from 1Password and sets required environment variables.`,
	Example: `  launch-disentangle infra plan
  launch-disentangle infra plan --env prod`,
	RunE: runInfraPlan,
}

var infraApplyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply infrastructure changes",
	Long: `Apply the Terraform execution plan to create or update infrastructure.
Prompts for confirmation before applying unless --yes is specified.`,
	Example: `  launch-disentangle infra apply
  launch-disentangle infra apply --env prod --yes`,
	RunE: runInfraApply,
}

var infraDestroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroy infrastructure (DANGEROUS)",
	Long: `Destroy all infrastructure managed by Terraform in the target environment.
This is a destructive operation that cannot be undone. Prompts for
confirmation unless --yes is specified.`,
	Example: `  launch-disentangle infra destroy --env dev
  launch-disentangle infra destroy --env dev --yes`,
	RunE: runInfraDestroy,
}

var infraOutputCmd = &cobra.Command{
	Use:   "output",
	Short: "Show Terraform outputs",
	Long: `Display the Terraform output values for the target environment.
Useful for retrieving cluster endpoints, VCN IDs, and other provisioned resource details.`,
	Example: `  launch-disentangle infra output
  launch-disentangle infra output --env prod`,
	RunE: runInfraOutput,
}

var infraKubeconfigCmd = &cobra.Command{
	Use:   "kubeconfig",
	Short: "Fetch kubeconfig from OCI",
	Long: `Retrieve the kubeconfig for an OKE cluster from Oracle Cloud Infrastructure.
Extracts the cluster OCID from Terraform outputs and uses the OCI CLI
to generate the kubeconfig.`,
	Example: `  launch-disentangle infra kubeconfig
  launch-disentangle infra kubeconfig --env prod`,
	RunE: runInfraKubeconfig,
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

// resolveInfraParams resolves the infra and env directories from InfraParams, sets up
// the executor's working directory and environment variables (including Cloudflare token).
func resolveInfraParams(p *InfraParams) (infraRoot, envDir string, err error) {
	infraRoot = p.Paths.InfraDir(p.Dir)
	if infraRoot == "" {
		return "", "", fmt.Errorf("could not find k8s-oci-foundation repo (use --dir or set repos.k8s_oci_foundation in config)")
	}

	envDir = p.Paths.InfraEnvDir(infraRoot, p.Env)
	if _, statErr := os.Stat(envDir); os.IsNotExist(statErr) {
		return "", "", fmt.Errorf("environment '%s' not found at %s", p.Env, envDir)
	}

	p.Exec.SetDir(envDir)

	if p.DryRun {
		p.Exec.SetEnv([]string{"TF_VAR_cloudflare_api_token=<resolved-at-runtime>"})
		fmt.Fprintln(p.Stdout, "  [dry-run] Skipping secret resolution")
		return infraRoot, envDir, nil
	}

	resolveToken := p.TokenResolverFunc
	if resolveToken == nil {
		resolveToken = cloudflare.ResolveToken
	}
	token, source, tokenErr := resolveToken()
	if tokenErr != nil {
		return "", "", fmt.Errorf("cloudflare token: %w", tokenErr)
	}
	fmt.Fprintf(p.Stdout, "  Using Cloudflare token from: %s\n", source)
	p.Exec.SetEnv([]string{"TF_VAR_cloudflare_api_token=" + token})

	return infraRoot, envDir, nil
}

// infraConfirm checks the ConfirmFunc (or defaults to auto-yes if nil).
func infraConfirm(p InfraParams, prompt string) bool {
	if p.ConfirmFunc != nil {
		return p.ConfirmFunc(prompt)
	}
	return true
}

// InfraInit initializes Terraform providers.
func InfraInit(p InfraParams) error {
	if _, _, err := resolveInfraParams(&p); err != nil {
		return err
	}
	fmt.Fprintln(p.Stdout, "Initializing Terraform...")
	if _, err := p.Exec.Run("tofu", "init", "-upgrade"); err != nil {
		return err
	}
	hints.Fprint(p.Stdout, []hints.NextStep{
		{Command: "infra plan", Description: "Preview infrastructure changes"},
	})
	return nil
}

// InfraPlan runs init + plan.
func InfraPlan(p InfraParams) error {
	if _, _, err := resolveInfraParams(&p); err != nil {
		return err
	}
	fmt.Fprintln(p.Stdout, "Initializing...")
	if _, err := p.Exec.Run("tofu", "init", "-upgrade"); err != nil {
		return err
	}
	fmt.Fprintln(p.Stdout, "\nPlanning...")
	if _, err := p.Exec.Run("tofu", "plan", "-var-file=terraform.tfvars", "-out=tfplan"); err != nil {
		return err
	}
	hints.Fprint(p.Stdout, []hints.NextStep{
		{Command: "infra apply", Description: "Apply the plan"},
		{Command: "infra destroy", Description: "Tear down infrastructure"},
	})
	return nil
}

// InfraApply applies a Terraform plan (runs plan first if no tfplan exists).
func InfraApply(p InfraParams) error {
	_, envDir, err := resolveInfraParams(&p)
	if err != nil {
		return err
	}

	planFile := filepath.Join(envDir, "tfplan")
	if _, statErr := os.Stat(planFile); os.IsNotExist(statErr) {
		fmt.Fprintln(p.Stdout, "No plan found, running plan first...")
		if _, planErr := p.Exec.Run("tofu", "plan", "-var-file=terraform.tfvars", "-out=tfplan"); planErr != nil {
			return planErr
		}
	}

	if !infraConfirm(p, "Apply this plan?") {
		fmt.Fprintln(p.Stdout, "Cancelled.")
		return nil
	}

	fmt.Fprintln(p.Stdout, "Applying...")
	if _, applyErr := p.Exec.Run("tofu", "apply", "tfplan"); applyErr != nil {
		return applyErr
	}

	_ = os.Remove(planFile)

	fmt.Fprintln(p.Stdout, "\nOutputs:")
	if _, outErr := p.Exec.Run("tofu", "output", "-json"); outErr != nil {
		return outErr
	}
	hints.Fprint(p.Stdout, []hints.NextStep{
		{Command: "infra kubeconfig", Description: "Fetch kubeconfig from OCI"},
		{Command: "cluster add oci-dev", Description: "Add cluster to fleet"},
		{Command: "bootstrap --cluster oci-dev", Description: "Bootstrap FluxCD on cluster"},
	})
	return nil
}

// InfraDestroy destroys all infrastructure.
func InfraDestroy(p InfraParams) error {
	if _, _, err := resolveInfraParams(&p); err != nil {
		return err
	}

	if !infraConfirm(p, "DESTROY all infrastructure? This cannot be undone.") {
		fmt.Fprintln(p.Stdout, "Cancelled.")
		return nil
	}

	_, err := p.Exec.Run("tofu", "destroy", "-var-file=terraform.tfvars")
	return err
}

// InfraOutput shows Terraform outputs.
func InfraOutput(p InfraParams) error {
	if _, _, err := resolveInfraParams(&p); err != nil {
		return err
	}
	_, err := p.Exec.Run("tofu", "output", "-json")
	return err
}

// InfraKubeconfig fetches the kubeconfig from OCI using the cluster_id output.
func InfraKubeconfig(p InfraParams) error {
	infraRoot, _, err := resolveInfraParams(&p)
	if err != nil {
		return err
	}

	result, err := p.Exec.RunSilent("tofu", "output", "-raw", "cluster_id")
	if err != nil {
		return fmt.Errorf("no cluster_id output found -- has infra been applied? %w", err)
	}

	region := p.Region
	if region == "" {
		region = "us-phoenix-1"
	}

	kubeconfigPath := filepath.Join(infraRoot, "kubeconfig")

	_, err = p.Exec.Run("oci", "ce", "cluster", "create-kubeconfig",
		"--cluster-id", strings.TrimSpace(result.Stdout),
		"--file", kubeconfigPath,
		"--region", region,
		"--token-version", "2.0.0",
		"--kube-endpoint", "PUBLIC_ENDPOINT",
	)
	if err != nil {
		return err
	}

	_ = os.Chmod(kubeconfigPath, 0600)
	fmt.Fprintf(p.Stdout, "Kubeconfig saved to %s\n", kubeconfigPath)
	hints.Fprint(p.Stdout, []hints.NextStep{
		{Command: "cluster add oci-dev", Description: "Add cluster to fleet"},
		{Command: "secrets init --cluster oci-dev", Description: "Bootstrap secrets"},
		{Command: "bootstrap --cluster oci-dev", Description: "Bootstrap FluxCD"},
	})
	return nil
}

// buildInfraParams constructs InfraParams from global Cobra flags.
func buildInfraParams() (InfraParams, error) {
	cfg, _ := config.Load(cfgFile)
	pr := paths.NewWithHome("", cfg)
	// If home is empty, use the real home directory.
	if pr.HomeDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return InfraParams{}, err
		}
		pr.HomeDir = home
	}

	region := "us-phoenix-1"
	if cfg != nil && cfg.OCIRegion != "" {
		region = cfg.OCIRegion
	}

	return InfraParams{
		Exec:    exec.NewRunner(),
		Paths:   pr,
		Stdout:  os.Stdout,
		Env:     infraEnv,
		Dir:     infraDir,
		CfgFile: cfgFile,
		DryRun:  dryRun,
		Verbose: verbose,
		AutoYes: autoYes,
		ConfirmFunc: func(prompt string) bool {
			return confirm(prompt)
		},
		TokenResolverFunc: cloudflare.ResolveToken,
		Region:            region,
	}, nil
}

func runInfraInit(_ *cobra.Command, _ []string) error {
	p, err := buildInfraParams()
	if err != nil {
		return err
	}
	return InfraInit(p)
}

func runInfraPlan(_ *cobra.Command, _ []string) error {
	p, err := buildInfraParams()
	if err != nil {
		return err
	}
	return InfraPlan(p)
}

func runInfraApply(_ *cobra.Command, _ []string) error {
	p, err := buildInfraParams()
	if err != nil {
		return err
	}
	return InfraApply(p)
}

func runInfraDestroy(_ *cobra.Command, _ []string) error {
	p, err := buildInfraParams()
	if err != nil {
		return err
	}
	return InfraDestroy(p)
}

func runInfraOutput(_ *cobra.Command, _ []string) error {
	p, err := buildInfraParams()
	if err != nil {
		return err
	}
	return InfraOutput(p)
}

func runInfraKubeconfig(_ *cobra.Command, _ []string) error {
	p, err := buildInfraParams()
	if err != nil {
		return err
	}
	return InfraKubeconfig(p)
}
