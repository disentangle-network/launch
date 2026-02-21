package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/disentangle-network/launch/internal/config"
	"github.com/disentangle-network/launch/internal/exec"
	"github.com/disentangle-network/launch/internal/hints"
	"github.com/disentangle-network/launch/internal/paths"
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

// ClusterImportParams holds all dependencies for ClusterImport.
type ClusterImportParams struct {
	Exec       exec.Executor
	Paths      *paths.Resolver
	Stdout     io.Writer
	Group      string
	SourcePath string
	Context    string
}

// ClusterImport imports a kubeconfig into a group-specific config file.
func ClusterImport(p ClusterImportParams) error {
	if _, err := os.Stat(p.SourcePath); os.IsNotExist(err) {
		return fmt.Errorf("kubeconfig not found: %s", p.SourcePath)
	}

	groupDir := p.Paths.KubeGroupDir(p.Group)
	if err := os.MkdirAll(groupDir, 0755); err != nil {
		return fmt.Errorf("failed to create %s: %w", groupDir, err)
	}
	targetPath := p.Paths.KubeGroupConfig(p.Group)

	// Detect source context
	result, err := p.Exec.RunSilent("kubectl", "config", "current-context", "--kubeconfig", p.SourcePath)
	if err != nil {
		return fmt.Errorf("could not determine source context: %w", err)
	}
	sourceCtx := strings.TrimSpace(result.Stdout)

	// Determine final context name
	finalCtx := sourceCtx
	if p.Context != "" {
		finalCtx = p.Context
	}

	fmt.Fprintf(p.Stdout, "Importing from %s (context: %s)\n", p.SourcePath, sourceCtx)

	// If renaming, work on a temp copy to avoid mutating the source
	importPath := p.SourcePath
	if finalCtx != sourceCtx {
		tmpFile, err := os.CreateTemp("", "launch-kubeconfig-*.yaml")
		if err != nil {
			return fmt.Errorf("failed to create temp file: %w", err)
		}
		defer func() { _ = os.Remove(tmpFile.Name()) }()

		data, err := os.ReadFile(p.SourcePath)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", p.SourcePath, err)
		}
		if _, err := tmpFile.Write(data); err != nil {
			_ = tmpFile.Close()
			return fmt.Errorf("failed to write temp file: %w", err)
		}
		_ = tmpFile.Close()
		importPath = tmpFile.Name()

		fmt.Fprintf(p.Stdout, "Renaming context '%s' → '%s'\n", sourceCtx, finalCtx)
		if _, err := p.Exec.RunSilent("kubectl", "config", "rename-context", sourceCtx, finalCtx, "--kubeconfig", importPath); err != nil {
			return fmt.Errorf("failed to rename context: %w", err)
		}
	}

	// Merge or create
	if info, err := os.Stat(targetPath); err == nil && info.Size() > 0 {
		p.Exec.SetEnv([]string{
			fmt.Sprintf("KUBECONFIG=%s:%s", targetPath, importPath),
		})
		result, err := p.Exec.RunSilent("kubectl", "config", "view", "--flatten")
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
		// Clear the env override
		p.Exec.SetEnv(nil)
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
	if _, err := p.Exec.RunSilent("kubectl", "config", "use-context", finalCtx, "--kubeconfig", targetPath); err != nil {
		fmt.Fprintf(p.Stdout, "Warning: could not set current context: %v\n", err)
	}

	// Show what's in the group now
	ctxResult, _ := p.Exec.RunSilent("kubectl", "config", "get-contexts", "-o", "name", "--kubeconfig", targetPath)
	contexts := strings.TrimSpace(ctxResult.Stdout)

	fmt.Fprintf(p.Stdout, "\nGroup '%s' → %s\n", p.Group, targetPath)
	fmt.Fprintf(p.Stdout, "Contexts: %s\n", strings.ReplaceAll(contexts, "\n", ", "))
	fmt.Fprintf(p.Stdout, "\nTo use: export KUBECONFIG=%s\n", targetPath)

	hints.Fprint(p.Stdout, []hints.NextStep{
		{Command: fmt.Sprintf("cluster add %s", p.Group), Description: "Generate fleet overlays"},
		{Command: fmt.Sprintf("secrets init --cluster %s", p.Group), Description: "Bootstrap SOPS secrets"},
		{Command: fmt.Sprintf("bootstrap --cluster %s", p.Group), Description: "Bootstrap FluxCD"},
	})

	return nil
}

func runClusterImport(cmd *cobra.Command, args []string) error {
	runner := exec.NewRunner()
	cfg, _ := config.Load(cfgFile)
	p := paths.NewWithHome("", cfg)
	if home, err := os.UserHomeDir(); err == nil {
		p = paths.NewWithHome(home, cfg)
	}

	return ClusterImport(ClusterImportParams{
		Exec:       runner,
		Paths:      p,
		Stdout:     os.Stdout,
		Group:      args[0],
		SourcePath: args[1],
		Context:    importContext,
	})
}
