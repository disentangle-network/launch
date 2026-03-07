package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/disentangle-network/launch/internal/config"
	"github.com/disentangle-network/launch/internal/exec"
	"github.com/disentangle-network/launch/internal/hints"
	"github.com/disentangle-network/launch/internal/paths"
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
	Long: `Show the fleet repository's git status, configured clusters, and apps/base presence.
Useful for verifying the fleet repo is properly initialized and up to date.`,
	Example: `  launch-disentangle fleet status`,
	RunE:    runFleetStatus,
}

func init() {
	rootCmd.AddCommand(fleetCmd)
	fleetCmd.AddCommand(fleetInitCmd)
	fleetCmd.AddCommand(fleetStatusCmd)

	fleetInitCmd.Flags().StringVar(&fleetOutputDir, "dir", "", "Output directory (default: ~/DISENTANGLE-NETWORK/fleet-deploy)")
	fleetInitCmd.Flags().StringVar(&fleetPrivate, "remote", "", "Private git remote URL (default: gh repo create)")
}

// FleetInitParams holds all dependencies for FleetInit.
type FleetInitParams struct {
	Exec    exec.Executor
	Paths   *paths.Resolver
	Stdout  io.Writer
	Dir     string         // flag override for output directory
	Remote  string         // explicit private remote URL
	CfgFile string         // config file path
	DryRun  bool           // show commands without executing
	Config  *config.Config // pre-loaded config (nil loads from CfgFile)
}

// FleetInit clones the fleet template and sets up the private repo.
func FleetInit(p FleetInitParams) error {
	dir := p.Paths.FleetDir(p.Dir)

	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		return fmt.Errorf("fleet-deploy already exists at %s", dir)
	}

	if p.DryRun {
		fmt.Fprintf(p.Stdout, "[dry-run] git clone https://github.com/disentangle-network/fleet.git %s\n", dir)
		fmt.Fprintf(p.Stdout, "[dry-run] git remote remove origin\n")
		fmt.Fprintf(p.Stdout, "[dry-run] git remote add template https://github.com/disentangle-network/fleet.git\n")
		if p.Remote != "" {
			fmt.Fprintf(p.Stdout, "[dry-run] git remote add origin %s\n", p.Remote)
		} else {
			owner := fleetInitOwner(p)
			if owner != "" {
				fmt.Fprintf(p.Stdout, "[dry-run] gh repo create %s/fleet-deploy --private --source %s --push\n", owner, dir)
			} else {
				fmt.Fprintf(p.Stdout, "[dry-run] Would detect owner via gh api user\n")
			}
		}
		fmt.Fprintf(p.Stdout, "[dry-run] Would save fleet_dir=%s to config\n", dir)
		return nil
	}

	fmt.Fprintln(p.Stdout, "Cloning fleet template...")
	_, err := p.Exec.Run("git", "clone", "https://github.com/disentangle-network/fleet.git", dir)
	if err != nil {
		return fmt.Errorf("clone failed: %w", err)
	}

	fmt.Fprintln(p.Stdout, "Removing public origin...")
	p.Exec.SetDir(dir)
	_, _ = p.Exec.Run("git", "remote", "remove", "origin")
	_, _ = p.Exec.Run("git", "remote", "add", "template", "https://github.com/disentangle-network/fleet.git")

	if p.Remote != "" {
		fmt.Fprintf(p.Stdout, "Setting private remote: %s\n", p.Remote)
		_, _ = p.Exec.Run("git", "remote", "add", "origin", p.Remote)
	} else {
		owner := fleetInitOwner(p)
		if owner == "" {
			fmt.Fprintf(p.Stdout, "  Could not detect owner — set remote manually:\n")
			fmt.Fprintf(p.Stdout, "  cd %s && git remote add origin <url> && git push -u origin main\n", dir)
		} else {
			fmt.Fprintf(p.Stdout, "Creating private repo: %s/fleet-deploy\n", owner)
			_, err := p.Exec.Run("gh", "repo", "create", fmt.Sprintf("%s/fleet-deploy", owner),
				"--private", "--source", dir, "--push")
			if err != nil {
				fmt.Fprintf(p.Stdout, "  gh repo create failed — set remote manually:\n")
				fmt.Fprintf(p.Stdout, "  cd %s && git remote add origin <url> && git push -u origin main\n", dir)
			}
		}
	}

	cfg := p.Config
	if cfg == nil {
		cfg, _ = config.Load(p.CfgFile)
	}
	if cfg != nil {
		cfg.FleetDir = dir
		cfgPath, _ := config.DefaultConfigPath()
		if err := config.Save(cfg, cfgPath); err != nil {
			fmt.Fprintf(p.Stdout, "Warning: could not save config: %v\n", err)
		}
		fmt.Fprintf(p.Stdout, "Saved fleet_dir to config: %s\n", dir)
	}

	hints.Fprint(p.Stdout, []hints.NextStep{
		{Command: "cluster import <n> <kubeconfig>", Description: "Import cluster kubeconfig"},
		{Command: "cluster add <n>", Description: "Generate fleet overlays"},
		{Command: "secrets init --cluster <n>", Description: "Bootstrap SOPS secrets"},
		{Command: "bootstrap --cluster <n>", Description: "Bootstrap FluxCD"},
	})

	return nil
}

// fleetInitOwner determines the repo owner: config.github_org first, then gh api user fallback.
func fleetInitOwner(p FleetInitParams) string {
	cfg := p.Config
	if cfg == nil {
		cfg, _ = config.Load(p.CfgFile)
	}
	if cfg != nil && cfg.GitHubOrg != "" {
		return cfg.GitHubOrg
	}
	ghUser, err := p.Exec.RunSilent("gh", "api", "user", "--jq", ".login")
	if err != nil || strings.TrimSpace(ghUser.Stdout) == "" {
		return ""
	}
	return strings.TrimSpace(ghUser.Stdout)
}

// FleetStatusParams holds all dependencies for FleetStatus.
type FleetStatusParams struct {
	Exec   exec.Executor
	Paths  *paths.Resolver
	Stdout io.Writer
	Dir    string // flag override for fleet directory
}

// FleetStatus displays the current fleet repo status.
func FleetStatus(p FleetStatusParams) error {
	dir := p.Paths.FleetDir(p.Dir)

	if _, err := os.Stat(filepath.Join(dir, ".git")); os.IsNotExist(err) {
		fmt.Fprintln(p.Stdout, "No fleet-deploy repo found.")
		hints.Fprint(p.Stdout, []hints.NextStep{
			{Command: "fleet init", Description: "Initialize private fleet repo"},
		})
		return nil
	}

	p.Exec.SetDir(dir)

	fmt.Fprintf(p.Stdout, "Fleet repo: %s\n\n", dir)

	fmt.Fprintln(p.Stdout, "Remotes:")
	_, _ = p.Exec.Run("git", "remote", "-v")

	fmt.Fprintln(p.Stdout, "\nClusters:")
	clustersDir := filepath.Join(dir, "clusters")
	entries, err := os.ReadDir(clustersDir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				fmt.Fprintf(p.Stdout, "  %s\n", e.Name())
			}
		}
	}

	fmt.Fprintln(p.Stdout, "\nStatus:")
	_, _ = p.Exec.Run("git", "status", "--short")

	return nil
}

func runFleetInit(cmd *cobra.Command, args []string) error {
	runner := exec.NewRunner()
	cfg, _ := config.Load(cfgFile)
	p := paths.NewWithHome("", cfg)
	if home, err := os.UserHomeDir(); err == nil {
		p = paths.NewWithHome(home, cfg)
	}

	return FleetInit(FleetInitParams{
		Exec:    runner,
		Paths:   p,
		Stdout:  os.Stdout,
		Dir:     fleetOutputDir,
		Remote:  fleetPrivate,
		CfgFile: cfgFile,
		DryRun:  dryRun,
		Config:  cfg,
	})
}

func runFleetStatus(cmd *cobra.Command, args []string) error {
	runner := exec.NewRunner()
	cfg, _ := config.Load(cfgFile)
	p := paths.NewWithHome("", cfg)
	if home, err := os.UserHomeDir(); err == nil {
		p = paths.NewWithHome(home, cfg)
	}

	return FleetStatus(FleetStatusParams{
		Exec:   runner,
		Paths:  p,
		Stdout: os.Stdout,
	})
}
