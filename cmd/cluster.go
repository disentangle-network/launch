package cmd

import (
	"fmt"
	"os"

	"github.com/disentangle-network/launch/internal/fleet"
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
	RunE:  runClusterList,
}

func init() {
	rootCmd.AddCommand(clusterCmd)
	clusterCmd.AddCommand(clusterAddCmd)
	clusterCmd.AddCommand(clusterListCmd)

	clusterAddCmd.Flags().StringVar(&clusterArch, "arch", "arm64", "CPU architecture (arm64, amd64)")
	clusterAddCmd.Flags().StringVar(&clusterInfra, "infra", "cloud", "Infrastructure type (cloud, bare-metal, local)")
	clusterAddCmd.Flags().IntVar(&clusterNodes, "nodes", 3, "Number of Disentangle nodes")
	clusterAddCmd.Flags().StringVar(&clusterResources, "resources", "medium", "Resource preset (small, medium, large)")
	clusterAddCmd.Flags().StringVar(&clusterStorageClass, "storage-class", "", "Kubernetes storage class (default: cluster default)")
	clusterAddCmd.Flags().StringVar(&clusterNebulaMode, "nebula", "disabled", "Nebula mesh mode (lighthouse, node, disabled)")
	clusterAddCmd.Flags().StringVar(&clusterNebulaPrefix, "nebula-prefix", "10.42.0", "Nebula overlay IP prefix")
	clusterAddCmd.Flags().StringVar(&clusterLHAddr, "nebula-lighthouse", "", "Lighthouse address for node mode (ip:port)")
	clusterCmd.PersistentFlags().StringVar(&clusterFleetDir, "fleet-dir", ".", "Path to fleet repository root")
}

func runClusterAdd(cmd *cobra.Command, args []string) error {
	name := args[0]

	// Validate fleet dir exists
	if _, err := os.Stat(clusterFleetDir); os.IsNotExist(err) {
		return fmt.Errorf("fleet directory not found: %s (run 'launch init' first)", clusterFleetDir)
	}

	// Check for apps/base as a fleet repo indicator
	if _, err := os.Stat(fmt.Sprintf("%s/apps/base", clusterFleetDir)); os.IsNotExist(err) {
		return fmt.Errorf("%s does not appear to be a fleet repo (missing apps/base)", clusterFleetDir)
	}

	cfg := fleet.ClusterConfig{
		Name:           name,
		Arch:           clusterArch,
		Infra:          clusterInfra,
		Nodes:          clusterNodes,
		Resources:      clusterResources,
		StorageClass:   clusterStorageClass,
		NebulaMode:     clusterNebulaMode,
		NebulaPrefix:   clusterNebulaPrefix,
		LighthouseAddr: clusterLHAddr,
	}

	fmt.Printf("Adding cluster '%s' to fleet:\n", name)
	fmt.Printf("  arch: %s, infra: %s, nodes: %d, resources: %s\n", cfg.Arch, cfg.Infra, cfg.Nodes, cfg.Resources)
	if cfg.NebulaMode != "disabled" {
		fmt.Printf("  nebula: %s, prefix: %s\n", cfg.NebulaMode, cfg.NebulaPrefix)
	}

	if err := fleet.AddCluster(clusterFleetDir, cfg); err != nil {
		return fmt.Errorf("failed to add cluster: %w", err)
	}

	fmt.Printf("\nCluster '%s' added.\n", name)
	fmt.Println("\nNext steps:")
	fmt.Printf("  launch secrets init --cluster %s --provider <provider>\n", name)
	if cfg.NebulaMode != "disabled" {
		if cfg.NebulaMode == "lighthouse" {
			fmt.Printf("  launch mesh add --cluster %s --lighthouse\n", name)
		} else {
			fmt.Printf("  launch mesh add --cluster %s --lighthouse-addr <ip:port>\n", name)
		}
	}
	fmt.Printf("  launch bootstrap --cluster %s\n", name)

	return nil
}

func runClusterList(cmd *cobra.Command, args []string) error {
	fleetDir := clusterFleetDir
	clustersDir := fmt.Sprintf("%s/clusters", fleetDir)

	entries, err := os.ReadDir(clustersDir)
	if err != nil {
		return fmt.Errorf("failed to read clusters directory: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("No clusters configured. Run 'launch cluster add <name>' to add one.")
		return nil
	}

	fmt.Println("Clusters:")
	for _, e := range entries {
		if e.IsDir() {
			fmt.Printf("  %s\n", e.Name())
		}
	}

	return nil
}
