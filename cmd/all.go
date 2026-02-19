package cmd

import (
	"fmt"

	"github.com/disentangle-network/launch/internal/config"
	"github.com/disentangle-network/launch/internal/exec"
	"github.com/disentangle-network/launch/internal/stages"
	"github.com/disentangle-network/launch/internal/state"
	"github.com/spf13/cobra"
)

var (
	freshStart bool
)

var allCmd = &cobra.Command{
	Use:   "all",
	Short: "Run all pipeline stages sequentially",
	Long:  "Runs discover, infra, secrets, and deploy stages in order. Resumes from last failure unless --fresh is set.",
	RunE:  runAll,
}

func init() {
	allCmd.Flags().BoolVar(&freshStart, "fresh", false, "reset state and start from scratch")
	rootCmd.AddCommand(allCmd)
}

func runAll(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	runner := exec.NewRunner()
	runner.Verbose = verbose
	runner.DryRun = dryRun

	var ps *state.PipelineState
	if !freshStart {
		ps, _ = state.Load("")
	}
	if ps == nil {
		ps = state.New()
	}

	if ps.AllCompleted() && !freshStart {
		fmt.Println("All stages already completed. Use --fresh to start over.")
		return nil
	}

	if !dryRun && !confirm("This will run the full deployment pipeline. Continue?") {
		fmt.Println("Aborted.")
		return nil
	}

	type stageFunc struct {
		name string
		run  func() error
	}

	stageFuncs := []stageFunc{
		{"discover", func() error {
			_, err := stages.RunDiscover(cfg, runner)
			return err
		}},
		{"infra", func() error {
			_, err := stages.RunInfra(cfg, runner, stages.InfraApply)
			return err
		}},
		{"secrets", func() error {
			_, err := stages.RunSecrets(cfg, runner)
			return err
		}},
		{"deploy", func() error {
			_, err := stages.RunDeploy(cfg, runner)
			return err
		}},
	}

	for _, sf := range stageFuncs {
		s := ps.Stages[sf.name]
		if s.Status == state.StatusCompleted {
			fmt.Printf("\n==> Stage '%s' already completed, skipping.\n", sf.name)
			continue
		}

		fmt.Printf("\n========================================\n")
		fmt.Printf("==> Stage: %s\n", sf.name)
		fmt.Printf("========================================\n\n")

		ps.StartStage(sf.name)
		if err := state.Save(ps, ""); err != nil {
			return fmt.Errorf("saving state: %w", err)
		}

		if err := sf.run(); err != nil {
			ps.FailStage(sf.name, err)
			_ = state.Save(ps, "")
			return fmt.Errorf("stage '%s' failed: %w", sf.name, err)
		}

		ps.CompleteStage(sf.name, nil)
		if err := state.Save(ps, ""); err != nil {
			return fmt.Errorf("saving state: %w", err)
		}
	}

	fmt.Println("\n========================================")
	fmt.Println("==> All stages completed successfully!")
	fmt.Println("========================================")
	return nil
}
