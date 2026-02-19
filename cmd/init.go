package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/disentangle-network/launch/internal/fleet"
	"github.com/spf13/cobra"
)

var initFleetName string
var initFleetOutput string

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Scaffold a new fleet monorepo",
	Long: `Initialize a new Disentangle fleet repository with the standard
monorepo structure: clusters/, infrastructure/, apps/, secrets/.

This creates the base Kustomize and Helm templates that FluxCD uses
to deploy Disentangle nodes across one or more Kubernetes clusters.`,
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().StringVar(&initFleetName, "name", "disentangle-fleet", "Name of the fleet repository")
	initCmd.Flags().StringVarP(&initFleetOutput, "output", "o", "", "Output directory (default: ./<name>)")
}

func runInit(cmd *cobra.Command, args []string) error {
	outputDir := initFleetOutput
	if outputDir == "" {
		outputDir = initFleetName
	}

	absDir, err := filepath.Abs(outputDir)
	if err != nil {
		return err
	}

	if _, err := os.Stat(absDir); err == nil {
		return fmt.Errorf("directory already exists: %s", absDir)
	}

	fmt.Printf("Scaffolding fleet repo: %s\n", absDir)

	if err := fleet.InitFleetRepo(absDir, initFleetName); err != nil {
		return fmt.Errorf("failed to scaffold fleet repo: %w", err)
	}

	fmt.Printf("\nFleet repo created at %s\n", absDir)
	fmt.Println("\nNext steps:")
	fmt.Println("  cd", outputDir)
	fmt.Println("  launch cluster add <name> --nodes 3")
	fmt.Println("  launch secrets init --cluster <name> --provider yubikey")
	fmt.Println("  git init && git add -A && git commit -m 'initial fleet'")
	fmt.Println("  launch bootstrap --cluster <name>")

	return nil
}
