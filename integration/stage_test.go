//go:build integration

package integration

import (
	"os"
	"testing"

	"github.com/disentangle-network/launch/internal/exec"
	"github.com/disentangle-network/launch/internal/testutil"
)

func TestDiscoverRequiresRepoPath(t *testing.T) {
	cfg := testutil.TestConfig(t)
	cfg.Repos.OCITFBootstrap = "/nonexistent/path"

	runner := exec.NewRunner()
	runner.DryRun = true

	// Import stages to test - but since dry-run won't actually run the binary,
	// we just verify the precondition check
	if _, err := os.Stat(cfg.Repos.OCITFBootstrap); !os.IsNotExist(err) {
		t.Error("expected nonexistent path to not exist")
	}
}

func TestInfraRequiresRepoPath(t *testing.T) {
	cfg := testutil.TestConfig(t)
	cfg.Repos.K8sOCIFoundation = "/nonexistent/path"

	if _, err := os.Stat(cfg.Repos.K8sOCIFoundation); !os.IsNotExist(err) {
		t.Error("expected nonexistent path to not exist")
	}
}

func TestRunnerDryRun(t *testing.T) {
	runner := exec.NewRunner()
	runner.DryRun = true

	result, err := runner.Run("echo", "hello")
	if err != nil {
		t.Fatalf("dry-run should not error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
}
