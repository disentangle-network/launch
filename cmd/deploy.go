package cmd

import (
	"fmt"

	"github.com/disentangle-network/launch/internal/config"
	"github.com/disentangle-network/launch/internal/exec"
	"github.com/disentangle-network/launch/internal/stages"
	"github.com/disentangle-network/launch/internal/state"
	"github.com/spf13/cobra"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Stage 4: Deploy nodes",
	Long:  "Deploys disentangle-node pods via Helm/FluxCD.",
	RunE:  runDeploy,
}

func init() {
	rootCmd.AddCommand(deployCmd)
}

func runDeploy(cmd *cobra.Command, args []string) error {
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

	ps.StartStage("deploy")
	if err := state.Save(ps, ""); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}

	result, err := stages.RunDeploy(cfg, runner)
	if err != nil {
		ps.FailStage("deploy", err)
		state.Save(ps, "")
		return err
	}

	output := map[string]interface{}{
		"image_verified": result.ImageVerified,
		"config_applied": result.ConfigApplied,
		"nodes_deployed": result.NodesDeployed,
	}
	ps.CompleteStage("deploy", output)
	if err := state.Save(ps, ""); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}

	fmt.Println("\nDeploy stage complete.")
	return nil
}
