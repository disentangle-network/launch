package stages

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/disentangle-network/launch/internal/config"
	"github.com/disentangle-network/launch/internal/exec"
)

type DiscoverResult struct {
	OutputDir string
	Files     []string
}

func RunDiscover(cfg *config.Config, runner *exec.Runner) (*DiscoverResult, error) {
	repoPath := cfg.Repos.OCITFBootstrap
	if repoPath == "" {
		return nil, fmt.Errorf("oci_tf_bootstrap repo path not configured")
	}

	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("oci-tf-bootstrap repo not found at %s", repoPath)
	}

	outputDir := filepath.Join(repoPath, "terraform-output")

	// Run oci-tf-bootstrap
	runner.Dir = repoPath
	_, err := runner.Run("go", "run", ".", "--always-free", "--output", outputDir)
	if err != nil {
		return nil, fmt.Errorf("oci-tf-bootstrap failed: %w", err)
	}

	// Verify output files exist
	expectedFiles := []string{"provider.tf", "locals.tf", "data.tf"}
	var found []string
	for _, f := range expectedFiles {
		path := filepath.Join(outputDir, f)
		if _, err := os.Stat(path); err == nil {
			found = append(found, f)
		}
	}

	if len(found) == 0 {
		return nil, fmt.Errorf("no terraform files generated in %s", outputDir)
	}

	return &DiscoverResult{
		OutputDir: outputDir,
		Files:     found,
	}, nil
}
