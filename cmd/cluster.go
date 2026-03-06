package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/disentangle-network/launch/internal/config"
	"github.com/disentangle-network/launch/internal/exec"
	"github.com/disentangle-network/launch/internal/fleet"
	"github.com/disentangle-network/launch/internal/hints"
	"github.com/disentangle-network/launch/internal/paths"
	"github.com/spf13/cobra"
)

var clusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "Manage clusters in the fleet",
	Long:  "Add, list, or remove clusters from the fleet repository.",
}

// Flags for cluster add
var (
	clusterArch         string
	clusterInfra        string
	clusterNodes        int
	clusterResources    string
	clusterStorageClass string
	clusterNebulaMode   string
	clusterNebulaPrefix string
	clusterLHAddr       string
	clusterFleetDir     string
)

var clusterAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add a cluster to the fleet",
	Long: `Generate Kustomize overlays and Helm value patches for a new cluster.

Resource presets:
  small    RPi, edge (250m CPU, 256Mi RAM, PoW 4)
  medium   Cloud free tier, dev (500m CPU, 512Mi RAM, PoW 8)
  large    Dedicated, production (2 CPU, 4Gi RAM, PoW 16)

Nebula modes:
  lighthouse   This cluster runs a lighthouse node (needs public IP)
  node         This cluster connects to an existing lighthouse
  disabled     No nebula mesh (k8s-internal only)`,
	Args: cobra.ExactArgs(1),
	RunE: runClusterAdd,
}

var clusterListCmd = &cobra.Command{
	Use:   "list",
	Short: "List clusters in the fleet",
	Long:  "List all clusters configured in the fleet repository with their settings summary.",
	Example: `  launch-disentangle cluster list
  launch-disentangle cluster list --fleet-dir ~/custom-fleet`,
	RunE: runClusterList,
}

var clusterRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a cluster from the fleet",
	Long: `Remove a cluster's Kustomize overlays, Helm patches, and secrets directory
from the fleet repository. Does not affect running infrastructure.`,
	Example: `  launch-disentangle cluster remove edge-1
  launch-disentangle cluster remove edge-1 --fleet-dir ~/custom-fleet`,
	Args: cobra.ExactArgs(1),
	RunE: runClusterRemove,
}

func init() {
	rootCmd.AddCommand(clusterCmd)
	clusterCmd.AddCommand(clusterAddCmd)
	clusterCmd.AddCommand(clusterListCmd)
	clusterCmd.AddCommand(clusterRemoveCmd)

	clusterAddCmd.Flags().StringVar(&clusterArch, "arch", "arm64", "CPU architecture (arm64, amd64)")
	clusterAddCmd.Flags().StringVar(&clusterInfra, "infra", "cloud", "Infrastructure type (cloud, bare-metal, local)")
	clusterAddCmd.Flags().IntVar(&clusterNodes, "nodes", 3, "Number of Disentangle nodes")
	clusterAddCmd.Flags().StringVar(&clusterResources, "resources", "medium", "Resource preset (small, medium, large)")
	clusterAddCmd.Flags().StringVar(&clusterStorageClass, "storage-class", "", "Kubernetes storage class (default: cluster default)")
	clusterAddCmd.Flags().StringVar(&clusterNebulaMode, "nebula", "disabled", "Nebula mesh mode (lighthouse, node, disabled)")
	clusterAddCmd.Flags().StringVar(&clusterNebulaPrefix, "nebula-prefix", "10.42.0", "Nebula overlay IP prefix")
	clusterAddCmd.Flags().StringVar(&clusterLHAddr, "nebula-lighthouse", "", "Lighthouse address for node mode (ip:port)")
	clusterCmd.PersistentFlags().StringVar(&clusterFleetDir, "fleet-dir", "", "Path to fleet repository root")
}

// ClusterAddParams holds all dependencies for ClusterAdd.
type ClusterAddParams struct {
	Exec         exec.Executor
	Paths        *paths.Resolver
	Stdout       io.Writer
	Name         string
	Arch         string
	Infra        string
	Nodes        int
	Resources    string
	StorageClass string
	NebulaMode   string
	NebulaPrefix string
	LHAddr       string
	FleetDir     string
}

// ClusterAdd generates Kustomize overlays and Helm value patches for a new cluster.
func ClusterAdd(p ClusterAddParams) error {
	fleetDir := p.Paths.FleetDir(p.FleetDir)

	// Validate fleet dir exists
	if _, err := os.Stat(fleetDir); os.IsNotExist(err) {
		return fmt.Errorf("fleet directory not found: %s (run 'launch init' first)", fleetDir)
	}

	// Check for apps/base as a fleet repo indicator
	if _, err := os.Stat(filepath.Join(fleetDir, "apps", "base")); os.IsNotExist(err) {
		return fmt.Errorf("%s does not appear to be a fleet repo (missing apps/base)", fleetDir)
	}

	cfg := fleet.ClusterConfig{
		Name:           p.Name,
		Arch:           p.Arch,
		Infra:          p.Infra,
		Nodes:          p.Nodes,
		Resources:      p.Resources,
		StorageClass:   p.StorageClass,
		NebulaMode:     p.NebulaMode,
		NebulaPrefix:   p.NebulaPrefix,
		LighthouseAddr: p.LHAddr,
	}

	fmt.Fprintf(p.Stdout, "Adding cluster '%s' to fleet:\n", p.Name)
	fmt.Fprintf(p.Stdout, "  arch: %s, infra: %s, nodes: %d, resources: %s\n", cfg.Arch, cfg.Infra, cfg.Nodes, cfg.Resources)
	if cfg.NebulaMode != "disabled" {
		fmt.Fprintf(p.Stdout, "  nebula: %s, prefix: %s\n", cfg.NebulaMode, cfg.NebulaPrefix)
	}

	if err := fleet.AddCluster(fleetDir, cfg); err != nil {
		return fmt.Errorf("failed to add cluster: %w", err)
	}

	fmt.Fprintf(p.Stdout, "\nCluster '%s' added.\n", p.Name)
	steps := []hints.NextStep{
		{Command: "secrets init --cluster " + p.Name, Description: "Bootstrap secrets"},
	}
	switch cfg.NebulaMode {
	case "lighthouse":
		steps = append(steps, hints.NextStep{Command: "mesh add --cluster " + p.Name + " --lighthouse", Description: "Add as mesh lighthouse"})
	case "node":
		steps = append(steps, hints.NextStep{Command: "mesh add --cluster " + p.Name, Description: "Add to mesh"})
	}
	steps = append(steps, hints.NextStep{Command: "bootstrap --cluster " + p.Name, Description: "Bootstrap FluxCD"})
	hints.Fprint(p.Stdout, steps)

	return nil
}

// ClusterListParams holds all dependencies for ClusterList.
type ClusterListParams struct {
	Paths    *paths.Resolver
	Stdout   io.Writer
	FleetDir string
}

// ClusterList lists all clusters in the fleet.
func ClusterList(p ClusterListParams) error {
	fleetDir := p.Paths.FleetDir(p.FleetDir)
	clustersDir := filepath.Join(fleetDir, "clusters")

	entries, err := os.ReadDir(clustersDir)
	if err != nil {
		return fmt.Errorf("failed to read clusters directory: %w", err)
	}

	if len(entries) == 0 {
		fmt.Fprintln(p.Stdout, "No clusters configured. Run 'launch cluster add <name>' to add one.")
		return nil
	}

	fmt.Fprintln(p.Stdout, "Clusters:")
	for _, e := range entries {
		if e.IsDir() {
			fmt.Fprintf(p.Stdout, "  %s\n", e.Name())
		}
	}

	return nil
}

// ClusterRemoveParams holds all dependencies for ClusterRemove.
type ClusterRemoveParams struct {
	Paths    *paths.Resolver
	Stdout   io.Writer
	Name     string
	FleetDir string
}

// ClusterRemove removes a cluster's overlay directory (and associated secrets)
// from the fleet repository.
func ClusterRemove(p ClusterRemoveParams) error {
	fleetDir := p.Paths.FleetDir(p.FleetDir)
	clusterDir := filepath.Join(fleetDir, "clusters", p.Name)

	if _, err := os.Stat(clusterDir); os.IsNotExist(err) {
		return fmt.Errorf("cluster '%s' not found in fleet", p.Name)
	}

	fmt.Fprintf(p.Stdout, "Removing cluster '%s' from fleet...\n", p.Name)

	if err := os.RemoveAll(clusterDir); err != nil {
		return fmt.Errorf("failed to remove cluster directory: %w", err)
	}

	secretsDir := filepath.Join(fleetDir, "secrets", p.Name)
	if _, err := os.Stat(secretsDir); err == nil {
		fmt.Fprintf(p.Stdout, "Removing secrets for '%s'...\n", p.Name)
		if err := os.RemoveAll(secretsDir); err != nil {
			return fmt.Errorf("failed to remove secrets directory: %w", err)
		}
	}

	fmt.Fprintf(p.Stdout, "Cluster '%s' removed.\n", p.Name)
	hints.Fprint(p.Stdout, []hints.NextStep{
		{Command: "cluster list", Description: "Verify cluster was removed"},
		{Command: "status", Description: "Check fleet health"},
	})

	return nil
}

func runClusterRemove(cmd *cobra.Command, args []string) error {
	cfg, _ := config.Load(cfgFile)
	p := paths.NewWithHome("", cfg)
	if home, err := os.UserHomeDir(); err == nil {
		p = paths.NewWithHome(home, cfg)
	}

	return ClusterRemove(ClusterRemoveParams{
		Paths:    p,
		Stdout:   os.Stdout,
		Name:     args[0],
		FleetDir: clusterFleetDir,
	})
}

func runClusterAdd(cmd *cobra.Command, args []string) error {
	runner := exec.NewRunner()
	cfg, _ := config.Load(cfgFile)
	p := paths.NewWithHome("", cfg)
	if home, err := os.UserHomeDir(); err == nil {
		p = paths.NewWithHome(home, cfg)
	}

	return ClusterAdd(ClusterAddParams{
		Exec:         runner,
		Paths:        p,
		Stdout:       os.Stdout,
		Name:         args[0],
		Arch:         clusterArch,
		Infra:        clusterInfra,
		Nodes:        clusterNodes,
		Resources:    clusterResources,
		StorageClass: clusterStorageClass,
		NebulaMode:   clusterNebulaMode,
		NebulaPrefix: clusterNebulaPrefix,
		LHAddr:       clusterLHAddr,
		FleetDir:     clusterFleetDir,
	})
}

func runClusterList(cmd *cobra.Command, args []string) error {
	cfg, _ := config.Load(cfgFile)
	p := paths.NewWithHome("", cfg)
	if home, err := os.UserHomeDir(); err == nil {
		p = paths.NewWithHome(home, cfg)
	}

	return ClusterList(ClusterListParams{
		Paths:    p,
		Stdout:   os.Stdout,
		FleetDir: clusterFleetDir,
	})
}
