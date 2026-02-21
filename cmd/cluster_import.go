package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/disentangle-network/launch/internal/exec"
	"github.com/disentangle-network/launch/internal/hints"
	"github.com/spf13/cobra"
)

var (
	importContext string
)

var clusterImportCmd = &cobra.Command{
	Use:   "import <name> <kubeconfig>",
	Short: "Import a kubeconfig and register cluster",
	Long: `Import a kubeconfig from any source (OCI, Talos, Omni, manual),
merge it into ~/.kube/config, and rename the context to match the
cluster name. Existing config is backed up before modification.`,
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

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not determine home directory: %w", err)
	}

	kubeDir := filepath.Join(home, ".kube")
	if err := os.MkdirAll(kubeDir, 0755); err != nil {
		return fmt.Errorf("failed to create %s: %w", kubeDir, err)
	}
	targetPath := filepath.Join(kubeDir, "config")

	runner := exec.NewRunner()

	// Copy source to a temp file so we never mutate the original
	sourceData, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", sourcePath, err)
	}
	tmpFile, err := os.CreateTemp("", "launch-kubeconfig-*.yaml")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)
	if _, err := tmpFile.Write(sourceData); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	tmpFile.Close()

	// Detect source context if not provided
	if importContext == "" {
		result, err := runner.RunSilent("kubectl", "config", "current-context", "--kubeconfig", tmpPath)
		if err != nil {
			return fmt.Errorf("could not determine source context: %w", err)
		}
		importContext = strings.TrimSpace(result.Stdout)
	}

	fmt.Printf("Importing cluster '%s' from %s (context: %s)\n", name, sourcePath, importContext)

	// Rename context in the temp copy
	if importContext != name {
		fmt.Printf("Renaming context '%s' → '%s'\n", importContext, name)
		if _, err := runner.RunSilent("kubectl", "config", "rename-context", importContext, name, "--kubeconfig", tmpPath); err != nil {
			return fmt.Errorf("failed to rename context: %w", err)
		}
	}

	// Back up existing config if present
	if info, err := os.Stat(targetPath); err == nil && info.Size() > 0 {
		backupPath := targetPath + "." + time.Now().Format("20060102-150405") + ".bak"
		existing, err := os.ReadFile(targetPath)
		if err != nil {
			return fmt.Errorf("failed to read existing config for backup: %w", err)
		}
		if err := os.WriteFile(backupPath, existing, 0600); err != nil {
			return fmt.Errorf("failed to write backup: %w", err)
		}
		fmt.Printf("Backed up existing config to %s\n", backupPath)
	}

	// Merge using KUBECONFIG env var (the only way colon-separated paths work)
	if info, err := os.Stat(targetPath); err == nil && info.Size() > 0 {
		mergeRunner := exec.NewRunner()
		mergeRunner.Env = []string{
			fmt.Sprintf("KUBECONFIG=%s:%s", targetPath, tmpPath),
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
		// No existing config — just copy the temp file
		data, err := os.ReadFile(tmpPath)
		if err != nil {
			return fmt.Errorf("failed to read temp kubeconfig: %w", err)
		}
		if err := os.WriteFile(targetPath, data, 0600); err != nil {
			return fmt.Errorf("failed to write config: %w", err)
		}
	}

	// Set the imported context as current
	if _, err := runner.RunSilent("kubectl", "config", "use-context", name, "--kubeconfig", targetPath); err != nil {
		fmt.Printf("Warning: could not set current context: %v\n", err)
	}

	fmt.Printf("\nCluster '%s' imported to %s\n", name, targetPath)

	hints.Print([]hints.NextStep{
		{Command: "cluster add " + name, Description: "Generate fleet overlays"},
		{Command: "secrets init --cluster " + name, Description: "Bootstrap SOPS secrets"},
		{Command: "bootstrap --cluster " + name, Description: "Bootstrap FluxCD"},
	})

	return nil
}
