package cmd

import (
	"fmt"

	"github.com/disentangle-network/launch/internal/config"
	"github.com/disentangle-network/launch/internal/exec"
	"github.com/disentangle-network/launch/internal/stages"
	"github.com/disentangle-network/launch/internal/state"
	"github.com/spf13/cobra"
)

var discoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Stage 1: OCI resource discovery",
	Long:  "Runs oci-tf-bootstrap to discover OCI Always Free resources and generate Terraform files.",
	RunE:  runDiscover,
}

func init() {
	rootCmd.AddCommand(discoverCmd)
}

func runDiscover(cmd *cobra.Command, args []string) error {
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

	ps.StartStage("discover")
	if err := state.Save(ps, ""); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}

	result, err := stages.RunDiscover(cfg, runner)
	if err != nil {
		ps.FailStage("discover", err)
		_ = state.Save(ps, "")
		return err
	}

	output := map[string]interface{}{
		"output_dir": result.OutputDir,
		"files":      result.Files,
	}
	ps.CompleteStage("discover", output)
	if err := state.Save(ps, ""); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}

	fmt.Printf("\nDiscovery complete. Generated %d files in %s\n", len(result.Files), result.OutputDir)
	return nil
}
