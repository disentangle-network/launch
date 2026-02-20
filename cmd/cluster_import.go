package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/disentangle-network/launch/internal/exec"
	"github.com/disentangle-network/launch/internal/hints"
	"github.com/spf13/cobra"
)

var (
	importKubeconfig string
	importContext    string
)

var clusterImportCmd = &cobra.Command{
	Use:   "import <name> <kubeconfig>",
	Short: "Import a kubeconfig and register cluster",
	Long: `Import a kubeconfig from any source (OCI, Talos, Omni, manual),
merge it into the unified launch kubeconfig, and rename the context
to match the cluster name.`,
	Args: cobra.ExactArgs(2),
	RunE: runClusterImport,
}

func init() {
	clusterCmd.AddCommand(clusterImportCmd)
	clusterImportCmd.Flags().StringVar(&importContext, "source-context", "", "Context name in source kubeconfig (default: current-context)")
}

func runClusterImport(cmd *cobra.Command, args []string) error {
	name := args[0]
	sourcePath := args[1]

	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return fmt.Errorf("kubeconfig not found: %s", sourcePath)
	}

	home, _ := os.UserHomeDir()
	mergedPath := filepath.Join(home, ".config", "launch", "kubeconfig")
	os.MkdirAll(filepath.Dir(mergedPath), 0755)

	runner := exec.NewRunner()

	if importContext == "" {
		result, err := runner.RunSilent("kubectl", "config", "current-context", "--kubeconfig", sourcePath)
		if err != nil {
			return fmt.Errorf("could not determine source context: %w", err)
		}
		importContext = result.Stdout
	}

	fmt.Printf("Importing cluster '%s' from %s (context: %s)\n", name, sourcePath, importContext)

	if importContext != name {
		fmt.Printf("Renaming context '%s' → '%s'\n", importContext, name)
		runner.RunSilent("kubectl", "config", "rename-context", importContext, name, "--kubeconfig", sourcePath)
	}

	if _, err := os.Stat(mergedPath); os.IsNotExist(err) {
		data, _ := os.ReadFile(sourcePath)
		if err := os.WriteFile(mergedPath, data, 0600); err != nil {
			return fmt.Errorf("failed to write kubeconfig: %w", err)
		}
	} else {
		merged := fmt.Sprintf("%s:%s", mergedPath, sourcePath)
		result, err := runner.RunSilent("kubectl", "config", "view", "--flatten", "--merge", "--kubeconfig", merged)
		if err != nil {
			return fmt.Errorf("merge failed: %w", err)
		}
		if err := os.WriteFile(mergedPath, []byte(result.Stdout), 0600); err != nil {
			return fmt.Errorf("failed to write merged kubeconfig: %w", err)
		}
	}

	runner.RunSilent("kubectl", "config", "use-context", name, "--kubeconfig", mergedPath)

	fmt.Printf("\nCluster '%s' imported to %s\n", name, mergedPath)
	fmt.Printf("Use: export KUBECONFIG=%s\n", mergedPath)

	hints.Print([]hints.NextStep{
		{Command: "cluster add " + name, Description: "Generate fleet overlays"},
		{Command: "secrets init --cluster " + name, Description: "Bootstrap SOPS secrets"},
		{Command: "bootstrap --cluster " + name, Description: "Bootstrap FluxCD"},
	})

	return nil
}
