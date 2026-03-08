package cmd

import (
	"encoding/json"
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

var (
	discoverProfile    string
	discoverConfigFile string
	discoverRegion     string
)

var infraDiscoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Discover OCI resources and generate terraform.tfvars",
	Long: `Run oci-tf-bootstrap to discover OCI tenancy resources (availability domains,
shapes, images, networking) and generate a terraform.tfvars file for the
target environment. This is the first step in the OCI greenfield flow.

Requires oci-tf-bootstrap to be installed.`,
	Example: `  launch-disentangle infra discover
  launch-disentangle infra discover --env prod
  launch-disentangle infra discover --profile PROD --region us-ashburn-1`,
	RunE: runInfraDiscover,
}

func init() {
	rootCmd.AddCommand(infraCmd)
	infraCmd.AddCommand(infraInitCmd)
	infraCmd.AddCommand(infraPlanCmd)
	infraCmd.AddCommand(infraApplyCmd)
	infraCmd.AddCommand(infraDestroyCmd)
	infraCmd.AddCommand(infraOutputCmd)
	infraCmd.AddCommand(infraKubeconfigCmd)
	infraCmd.AddCommand(infraDiscoverCmd)

	infraCmd.PersistentFlags().StringVar(&infraEnv, "env", "dev", "Environment (dev, staging, prod)")
	infraCmd.PersistentFlags().StringVar(&infraDir, "dir", "", "Infrastructure repo root (default: auto-detect)")

	infraDiscoverCmd.Flags().StringVar(&discoverProfile, "profile", "", "OCI config profile (default: DEFAULT)")
	infraDiscoverCmd.Flags().StringVar(&discoverConfigFile, "oci-config", "", "OCI config file path")
	infraDiscoverCmd.Flags().StringVar(&discoverRegion, "region", "", "Override OCI region")
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
	if p.DryRun {
		fmt.Fprintln(p.Stdout, "[dry-run] tofu init -upgrade")
		return nil
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
	if p.DryRun {
		fmt.Fprintln(p.Stdout, "[dry-run] tofu init -upgrade")
		fmt.Fprintln(p.Stdout, "[dry-run] tofu plan -var-file=terraform.tfvars -out=tfplan")
		return nil
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

	if p.DryRun {
		fmt.Fprintln(p.Stdout, "[dry-run] tofu plan -var-file=terraform.tfvars -out=tfplan")
		fmt.Fprintln(p.Stdout, "[dry-run] tofu apply tfplan")
		fmt.Fprintln(p.Stdout, "[dry-run] tofu output -json")
		return nil
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

	if p.DryRun {
		fmt.Fprintln(p.Stdout, "[dry-run] tofu destroy -var-file=terraform.tfvars")
		return nil
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
	if p.DryRun {
		fmt.Fprintln(p.Stdout, "[dry-run] tofu output -json")
		return nil
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

	if p.DryRun {
		fmt.Fprintln(p.Stdout, "[dry-run] tofu output -raw cluster_id")
		fmt.Fprintf(p.Stdout, "[dry-run] oci ce cluster create-kubeconfig --file %s\n", filepath.Join(infraRoot, "kubeconfig"))
		return nil
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

// InfraDiscoverParams holds dependencies for InfraDiscover.
type InfraDiscoverParams struct {
	Exec        exec.Executor
	Paths       *paths.Resolver
	Stdout      io.Writer
	Env         string
	Dir         string // infra dir override
	DryRun      bool
	Profile     string // OCI config profile
	ConfigFile  string // OCI config file path
	Region      string // OCI region override
	Compartment string // OCI compartment override
	CfgFile     string // launch config file path
}

// discoveryResult is the subset of oci-tf-bootstrap JSON output we need.
type discoveryResult struct {
	CompartmentID string `json:"compartment_id"`
	Tenancy       struct {
		ID         string `json:"id"`
		HomeRegion string `json:"home_region"`
	} `json:"tenancy"`
	AvailabilityDomains []struct {
		Name string `json:"name"`
	} `json:"availability_domains"`
}

// InfraDiscover runs oci-tf-bootstrap to discover OCI resources and generates
// a terraform.tfvars file for the target environment.
func InfraDiscover(p InfraDiscoverParams) error {
	infraRoot := p.Paths.InfraDir(p.Dir)
	if infraRoot == "" {
		return fmt.Errorf("could not find k8s-oci-foundation repo (use --dir or set repos.k8s_oci_foundation in config)")
	}

	envDir := p.Paths.InfraEnvDir(infraRoot, p.Env)
	if err := os.MkdirAll(envDir, 0o750); err != nil {
		return fmt.Errorf("creating environment directory: %w", err)
	}

	// Build oci-tf-bootstrap args
	args := []string{"--json", "--always-free", "--oke"}
	if p.Profile != "" {
		args = append(args, "--profile", p.Profile)
	}
	if p.ConfigFile != "" {
		args = append(args, "--config-file", p.ConfigFile)
	}
	if p.Region != "" {
		args = append(args, "--region", p.Region)
	}
	if p.Compartment != "" {
		args = append(args, "--compartment", p.Compartment)
	}

	if p.DryRun {
		fmt.Fprintf(p.Stdout, "[dry-run] oci-tf-bootstrap %s\n", strings.Join(args, " "))
		fmt.Fprintf(p.Stdout, "[dry-run] Would write terraform.tfvars to %s\n", envDir)
		return nil
	}

	fmt.Fprintln(p.Stdout, "Discovering OCI resources...")
	result, err := p.Exec.RunSilent("oci-tf-bootstrap", args...)
	if err != nil {
		fmt.Fprintf(p.Stdout, "Error: %v\n", err)
		if result != nil && result.Stderr != "" {
			fmt.Fprintf(p.Stdout, "stderr: %s\n", result.Stderr)
		}
		return fmt.Errorf("oci-tf-bootstrap failed: %w", err)
	}

	// oci-tf-bootstrap prints header/progress info before JSON on stdout.
	// Find the JSON object start.
	jsonStart := strings.Index(result.Stdout, "{")
	if jsonStart < 0 {
		fmt.Fprintln(p.Stdout, "Error: no JSON found in oci-tf-bootstrap output")
		return fmt.Errorf("oci-tf-bootstrap produced no JSON output")
	}
	jsonOutput := result.Stdout[jsonStart:]

	var discovery discoveryResult
	if err := json.Unmarshal([]byte(jsonOutput), &discovery); err != nil {
		fmt.Fprintf(p.Stdout, "Error parsing discovery output: %v\n", err)
		return fmt.Errorf("parsing oci-tf-bootstrap output: %w", err)
	}

	tenancyID := discovery.Tenancy.ID
	compartmentID := discovery.CompartmentID
	if compartmentID == "" {
		compartmentID = tenancyID
	}
	region := discovery.Tenancy.HomeRegion
	if p.Region != "" {
		region = p.Region
	}

	// Load launch config for Cloudflare values
	cfg, _ := config.Load(p.CfgFile)
	cfDomain := ""
	cfAccountID := ""
	if cfg != nil {
		cfDomain = cfg.Domain
		cfAccountID = cfg.CloudflareAccountID
	}

	// Generate terraform.tfvars
	tfvars := fmt.Sprintf(`# Generated by launch infra discover
# OCI Configuration (from oci-tf-bootstrap discovery)
tenancy_ocid   = %q
compartment_id = %q
region         = %q

# Environment
environment = %q

# Cloudflare Configuration
cloudflare_account_id  = %q
cloudflare_domain      = %q
cloudflare_create_zone = false

# Load Balancer IP (set after first deployment)
load_balancer_ip = ""

# Kubernetes Configuration
kubernetes_version = "v1.31.10"

# Network Configuration
vcn_cidrs           = ["10.0.0.0/16"]
public_subnet_cidr  = "10.0.0.0/24"
private_subnet_cidr = "10.0.1.0/24"

# ARM Node Pool Configuration (OCI Always Free tier)
# Free tier: 4 OCPUs, 24GB memory total
arm_node_pool_count = 2
arm_node_pool_size  = 1
arm_ocpus           = 2
arm_memory_gbs      = 12

# Resource Tags
tags = {
  project = "disentangle-network"
}
`, tenancyID, compartmentID, region, p.Env, cfAccountID, cfDomain)

	tfvarsPath := filepath.Join(envDir, "terraform.tfvars")
	if err := os.WriteFile(tfvarsPath, []byte(tfvars), 0o600); err != nil {
		return fmt.Errorf("writing terraform.tfvars: %w", err)
	}

	fmt.Fprintf(p.Stdout, "Discovered OCI tenancy: %s\n", tenancyID)
	fmt.Fprintf(p.Stdout, "  Region:      %s\n", region)
	fmt.Fprintf(p.Stdout, "  Compartment: %s\n", compartmentID)
	if len(discovery.AvailabilityDomains) > 0 {
		fmt.Fprintf(p.Stdout, "  ADs:         %d\n", len(discovery.AvailabilityDomains))
	}
	fmt.Fprintf(p.Stdout, "\nGenerated %s\n", tfvarsPath)

	hints.Fprint(p.Stdout, []hints.NextStep{
		{Command: "infra init", Description: "Initialize Terraform providers"},
		{Command: "infra plan", Description: "Preview infrastructure changes"},
	})
	return nil
}

func runInfraDiscover(_ *cobra.Command, _ []string) error {
	cfg, _ := config.Load(cfgFile)
	pr := paths.NewWithHome("", cfg)
	if home, err := os.UserHomeDir(); err == nil {
		pr.HomeDir = home
	}

	compartment := ""
	if cfg != nil && cfg.OCICompartmentID != "" {
		compartment = cfg.OCICompartmentID
	}

	region := discoverRegion
	if region == "" && cfg != nil && cfg.OCIRegion != "" {
		region = cfg.OCIRegion
	}

	return InfraDiscover(InfraDiscoverParams{
		Exec:        exec.NewRunner(),
		Paths:       pr,
		Stdout:      os.Stdout,
		Env:         infraEnv,
		Dir:         infraDir,
		DryRun:      dryRun,
		Profile:     discoverProfile,
		ConfigFile:  discoverConfigFile,
		Region:      region,
		Compartment: compartment,
		CfgFile:     cfgFile,
	})
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
