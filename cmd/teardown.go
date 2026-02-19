package cmd

import (
	"fmt"

	"github.com/disentangle-network/launch/internal/config"
	"github.com/disentangle-network/launch/internal/exec"
	"github.com/spf13/cobra"
)

var teardownCmd = &cobra.Command{
	Use:   "teardown",
	Short: "Destroy infrastructure",
	Long:  "Destroys provisioned infrastructure. Requires confirmation.",
	RunE:  runTeardown,
}

func init() {
	rootCmd.AddCommand(teardownCmd)
}

func runTeardown(cmd *cobra.Command, args []string) error {
	if !confirm("WARNING: This will destroy all provisioned infrastructure. Are you sure?") {
		fmt.Println("Aborted.")
		return nil
	}

	if !confirm("FINAL CONFIRMATION: This action is irreversible. Type 'y' to confirm") {
		fmt.Println("Aborted.")
		return nil
	}

	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	runner := exec.NewRunner()
	runner.Verbose = verbose
	runner.DryRun = dryRun

	repoPath := cfg.Repos.K8sOCIFoundation
	if repoPath == "" {
		return fmt.Errorf("k8s_oci_foundation repo path not configured")
	}

	runner.Dir = repoPath
	env := cfg.Environment
	if env == "" {
		env = "dev"
	}

	fmt.Println("==> Destroying infrastructure...")
	if _, err := runner.Run("task", "destroy", fmt.Sprintf("ENV=%s", env)); err != nil {
		return fmt.Errorf("terraform destroy failed: %w", err)
	}

	fmt.Println("\nInfrastructure destroyed.")
	return nil
}
