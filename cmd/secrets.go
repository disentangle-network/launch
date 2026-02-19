package cmd

import (
	"fmt"

	"github.com/disentangle-network/launch/internal/config"
	"github.com/disentangle-network/launch/internal/exec"
	"github.com/disentangle-network/launch/internal/stages"
	"github.com/disentangle-network/launch/internal/state"
	"github.com/spf13/cobra"
)

var secretsCmd = &cobra.Command{
	Use:   "secrets",
	Short: "Stage 3: Bootstrap secrets",
	Long:  "Bootstraps cluster secrets using genesis-operator.",
	RunE:  runSecrets,
}

func init() {
	rootCmd.AddCommand(secretsCmd)
}

func runSecrets(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	runner := exec.NewRunner()
	runner.Verbose = verbose
	runner.DryRun = dryRun

	ps, _ := state.Load("")
	if ps == nil {
		ps = state.New()
	}

	ps.StartStage("secrets")
	if err := state.Save(ps, ""); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}

	result, err := stages.RunSecrets(cfg, runner)
	if err != nil {
		ps.FailStage("secrets", err)
		_ = state.Save(ps, "")
		return err
	}

	output := map[string]interface{}{
		"operator_deployed": result.OperatorDeployed,
		"bootstrap_applied": result.BootstrapApplied,
	}
	ps.CompleteStage("secrets", output)
	if err := state.Save(ps, ""); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}

	fmt.Println("\nSecrets stage complete.")
	return nil
}
