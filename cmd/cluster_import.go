package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/disentangle-network/launch/internal/exec"
	"github.com/disentangle-network/launch/internal/hints"
	"github.com/spf13/cobra"
)

var (
	importContext string
)

var clusterImportCmd = &cobra.Command{
	Use:   "import <group> <kubeconfig>",
	Short: "Import a kubeconfig into a named group",
	Long: `Import a kubeconfig from any source (OCI, Talos, Omni, manual) into
a group-specific config file at ~/.kube/<group>/config.

Multiple imports with the same group name merge into the same file.
Context names are preserved from the source kubeconfig unless
--context is specified.

Example:
  launch-disentangle cluster import disentangle ~/kubeconfigs/main.yaml
  launch-disentangle cluster import disentangle ~/kubeconfigs/kv.yaml
  launch-disentangle cluster import disentangle ~/kubeconfigs/zk.yaml
  export KUBECONFIG=~/.kube/disentangle/config`,
	Args: cobra.ExactArgs(2),
	RunE: runClusterImport,
}

func init() {
	clusterCmd.AddCommand(clusterImportCmd)
	clusterImportCmd.Flags().StringVar(&importContext, "context", "", "Rename the imported context (default: keep source context name)")
}

func runClusterImport(cmd *cobra.Command, args []string) error {
	group := args[0]
	sourcePath := args[1]

	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return fmt.Errorf("kubeconfig not found: %s", sourcePath)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not determine home directory: %w", err)
	}

	groupDir := filepath.Join(home, ".kube", group)
	if err := os.MkdirAll(groupDir, 0755); err != nil {
		return fmt.Errorf("failed to create %s: %w", groupDir, err)
	}
	targetPath := filepath.Join(groupDir, "config")

	runner := exec.NewRunner()

	// Detect source context
	result, err := runner.RunSilent("kubectl", "config", "current-context", "--kubeconfig", sourcePath)
	if err != nil {
		return fmt.Errorf("could not determine source context: %w", err)
	}
	sourceCtx := strings.TrimSpace(result.Stdout)

	// Determine final context name
	finalCtx := sourceCtx
	if importContext != "" {
		finalCtx = importContext
	}

	fmt.Printf("Importing from %s (context: %s)\n", sourcePath, sourceCtx)

	// If renaming, work on a temp copy to avoid mutating the source
	importPath := sourcePath
	if finalCtx != sourceCtx {
		tmpFile, err := os.CreateTemp("", "launch-kubeconfig-*.yaml")
		if err != nil {
			return fmt.Errorf("failed to create temp file: %w", err)
		}
		defer os.Remove(tmpFile.Name())

		data, err := os.ReadFile(sourcePath)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", sourcePath, err)
		}
		if _, err := tmpFile.Write(data); err != nil {
			tmpFile.Close()
			return fmt.Errorf("failed to write temp file: %w", err)
		}
		tmpFile.Close()
		importPath = tmpFile.Name()

		fmt.Printf("Renaming context '%s' → '%s'\n", sourceCtx, finalCtx)
		if _, err := runner.RunSilent("kubectl", "config", "rename-context", sourceCtx, finalCtx, "--kubeconfig", importPath); err != nil {
			return fmt.Errorf("failed to rename context: %w", err)
		}
	}

	// Merge or create
	if info, err := os.Stat(targetPath); err == nil && info.Size() > 0 {
		mergeRunner := exec.NewRunner()
		mergeRunner.Env = []string{
			fmt.Sprintf("KUBECONFIG=%s:%s", targetPath, importPath),
		}
		result, err := mergeRunner.RunSilent("kubectl", "config", "view", "--flatten")
		if err != nil {
			return fmt.Errorf("merge failed: %w", err)
		}
		merged := result.Stdout
		if strings.TrimSpace(merged) == "" {
			return fmt.Errorf("merge produced empty output — source kubeconfig may be invalid")
		}
		if err := os.WriteFile(targetPath, []byte(merged), 0600); err != nil {
			return fmt.Errorf("failed to write merged config: %w", err)
		}
	} else {
		data, err := os.ReadFile(importPath)
		if err != nil {
			return fmt.Errorf("failed to read kubeconfig: %w", err)
		}
		if err := os.WriteFile(targetPath, data, 0600); err != nil {
			return fmt.Errorf("failed to write config: %w", err)
		}
	}

	// Set the imported context as current
	if _, err := runner.RunSilent("kubectl", "config", "use-context", finalCtx, "--kubeconfig", targetPath); err != nil {
		fmt.Printf("Warning: could not set current context: %v\n", err)
	}

	// Show what's in the group now
	ctxResult, _ := runner.RunSilent("kubectl", "config", "get-contexts", "-o", "name", "--kubeconfig", targetPath)
	contexts := strings.TrimSpace(ctxResult.Stdout)

	fmt.Printf("\nGroup '%s' → %s\n", group, targetPath)
	fmt.Printf("Contexts: %s\n", strings.ReplaceAll(contexts, "\n", ", "))
	fmt.Printf("\nTo use: export KUBECONFIG=%s\n", targetPath)

	hints.Print([]hints.NextStep{
		{Command: fmt.Sprintf("cluster add %s", group), Description: "Generate fleet overlays"},
		{Command: fmt.Sprintf("secrets init --cluster %s", group), Description: "Bootstrap SOPS secrets"},
		{Command: fmt.Sprintf("bootstrap --cluster %s", group), Description: "Bootstrap FluxCD"},
	})

	return nil
}
