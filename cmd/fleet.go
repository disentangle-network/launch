package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/disentangle-network/launch/internal/config"
	"github.com/disentangle-network/launch/internal/exec"
	"github.com/disentangle-network/launch/internal/hints"
	"github.com/spf13/cobra"
)

var (
	fleetOutputDir string
	fleetPrivate   string
)

var fleetCmd = &cobra.Command{
	Use:   "fleet",
	Short: "Manage the fleet deployment repository",
}

var fleetInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize private fleet repo from template",
	Long: `Clone the public fleet template and push to a private repo for Flux.
The private repo holds cluster overlays, SOPS secrets, and is the
GitOps source of truth that FluxCD watches.`,
	RunE: runFleetInit,
}

var fleetStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show fleet repo status",
	RunE:  runFleetStatus,
}

func init() {
	rootCmd.AddCommand(fleetCmd)
	fleetCmd.AddCommand(fleetInitCmd)
	fleetCmd.AddCommand(fleetStatusCmd)

	fleetInitCmd.Flags().StringVar(&fleetOutputDir, "dir", "", "Output directory (default: ~/DISENTANGLE-NETWORK/fleet-deploy)")
	fleetInitCmd.Flags().StringVar(&fleetPrivate, "remote", "", "Private git remote URL (default: gh repo create)")
}

func resolveFleetDir() string {
	cfg, _ := config.Load(cfgFile)
	if cfg != nil && cfg.FleetDir != "" {
		return cfg.FleetDir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "DISENTANGLE-NETWORK", "fleet-deploy")
}

func runFleetInit(cmd *cobra.Command, args []string) error {
	dir := fleetOutputDir
	if dir == "" {
		dir = resolveFleetDir()
	}

	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		return fmt.Errorf("fleet-deploy already exists at %s", dir)
	}

	runner := exec.NewRunner()

	fmt.Println("Cloning fleet template...")
	_, err := runner.Run("git", "clone", "https://github.com/disentangle-network/fleet.git", dir)
	if err != nil {
		return fmt.Errorf("clone failed: %w", err)
	}

	fmt.Println("Removing public origin...")
	runner.Dir = dir
	_, _ = runner.Run("git", "remote", "remove", "origin")
	_, _ = runner.Run("git", "remote", "add", "template", "https://github.com/disentangle-network/fleet.git")

	if fleetPrivate != "" {
		fmt.Printf("Setting private remote: %s\n", fleetPrivate)
		_, _ = runner.Run("git", "remote", "add", "origin", fleetPrivate)
	} else {
		// Detect the active gh user — the owner must match the authenticated account
		ghUser, err := runner.RunSilent("gh", "api", "user", "--jq", ".login")
		if err != nil || strings.TrimSpace(ghUser.Stdout) == "" {
			fmt.Printf("  Could not detect gh user — set remote manually:\n")
			fmt.Printf("  cd %s && git remote add origin <url> && git push -u origin main\n", dir)
		} else {
			owner := strings.TrimSpace(ghUser.Stdout)
			fmt.Printf("Creating private repo: %s/fleet-deploy\n", owner)
			_, err := runner.Run("gh", "repo", "create", fmt.Sprintf("%s/fleet-deploy", owner),
				"--private", "--source", dir, "--push")
			if err != nil {
				fmt.Printf("  gh repo create failed — set remote manually:\n")
				fmt.Printf("  cd %s && git remote add origin <url> && git push -u origin main\n", dir)
			}
		}
	}

	cfg, _ := config.Load(cfgFile)
	if cfg != nil {
		cfg.FleetDir = dir
		cfgPath, _ := config.DefaultConfigPath()
		if err := config.Save(cfg, cfgPath); err != nil {
			fmt.Printf("Warning: could not save config: %v\n", err)
		}
		fmt.Printf("Saved fleet_dir to config: %s\n", dir)
	}

	hints.Print([]hints.NextStep{
		{Command: "cluster import <n> <kubeconfig>", Description: "Import cluster kubeconfig"},
		{Command: "cluster add <n>", Description: "Generate fleet overlays"},
		{Command: "secrets init --cluster <n>", Description: "Bootstrap SOPS secrets"},
		{Command: "bootstrap --cluster <n>", Description: "Bootstrap FluxCD"},
	})

	return nil
}

func runFleetStatus(cmd *cobra.Command, args []string) error {
	dir := resolveFleetDir()

	if _, err := os.Stat(filepath.Join(dir, ".git")); os.IsNotExist(err) {
		fmt.Println("No fleet-deploy repo found.")
		hints.Print([]hints.NextStep{
			{Command: "fleet init", Description: "Initialize private fleet repo"},
		})
		return nil
	}

	runner := exec.NewRunner()
	runner.Dir = dir

	fmt.Printf("Fleet repo: %s\n\n", dir)

	fmt.Println("Remotes:")
	_, _ = runner.Run("git", "remote", "-v")

	fmt.Println("\nClusters:")
	clustersDir := filepath.Join(dir, "clusters")
	entries, err := os.ReadDir(clustersDir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				fmt.Printf("  %s\n", e.Name())
			}
		}
	}

	fmt.Println("\nStatus:")
	_, _ = runner.Run("git", "status", "--short")

	return nil
}
