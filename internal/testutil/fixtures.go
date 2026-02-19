package testutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/disentangle-network/launch/internal/config"
)

// TestConfig returns a Config pointing to temp directories for all repos.
func TestConfig(t *testing.T) *config.Config {
	t.Helper()
	dir := t.TempDir()
	repos := []string{"oci-tf-bootstrap", "k8s-oci-foundation", "genesis-operator", "deploy"}
	for _, r := range repos {
		_ = os.MkdirAll(filepath.Join(dir, r), 0o755)
	}
	return &config.Config{
		OCIRegion:       "us-test-1",
		Environment:     "test",
		ClusterName:     "test-cluster",
		ProtocolImage:   "ghcr.io/test/node",
		ProtocolVersion: "v0.0.1",
		GenesisImage:    "ghcr.io/test/genesis",
		GenesisVersion:  "v0.0.1",
		Repos: config.Repos{
			OCITFBootstrap:   filepath.Join(dir, "oci-tf-bootstrap"),
			K8sOCIFoundation: filepath.Join(dir, "k8s-oci-foundation"),
			GenesisOperator:  filepath.Join(dir, "genesis-operator"),
			Deploy:           filepath.Join(dir, "deploy"),
		},
	}
}

// WriteFile creates a file with content in the given directory.
func WriteFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
