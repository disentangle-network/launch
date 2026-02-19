package cmd

import (
	"fmt"

	"github.com/disentangle-network/launch/internal/config"
	"github.com/disentangle-network/launch/internal/exec"
	"github.com/disentangle-network/launch/internal/stages"
	"github.com/disentangle-network/launch/internal/state"
	"github.com/spf13/cobra"
)

var infraCmd = &cobra.Command{
	Use:   "infra [plan|apply]",
	Short: "Stage 2: Provision OKE cluster",
	Long:  "Provisions the OKE cluster using k8s-oci-foundation with Terraform and bootstraps FluxCD.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runInfra,
}

func init() {
	rootCmd.AddCommand(infraCmd)
}

func runInfra(cmd *cobra.Command, args []string) error {
	action := stages.InfraApply
	if len(args) > 0 {
		switch args[0] {
		case "plan":
			action = stages.InfraPlan
		case "apply":
			action = stages.InfraApply
		default:
			return fmt.Errorf("unknown action %q (use 'plan' or 'apply')", args[0])
		}
	}

	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if action == stages.InfraApply && !dryRun {
		if !confirm("This will provision cloud infrastructure. Continue?") {
			fmt.Println("Aborted.")
			return nil
		}
	}

	runner := exec.NewRunner()
	runner.Verbose = verbose
	runner.DryRun = dryRun

	ps, _ := state.Load("")
	if ps == nil {
		ps = state.New()
	}

	ps.StartStage("infra")
	if err := state.Save(ps, ""); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}

	result, err := stages.RunInfra(cfg, runner, action)
	if err != nil {
		ps.FailStage("infra", err)
		_ = state.Save(ps, "")
		return err
	}

	output := map[string]interface{}{
		"action":     string(result.Action),
		"planned":    result.Planned,
		"applied":    result.Applied,
		"flux_ready": result.FluxReady,
	}
	ps.CompleteStage("infra", output)
	if err := state.Save(ps, ""); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}

	fmt.Println("\nInfrastructure stage complete.")
	return nil
}
