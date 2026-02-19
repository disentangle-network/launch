package fleet

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitFleetRepo(t *testing.T) {
	tmpDir := t.TempDir()
	fleetDir := filepath.Join(tmpDir, "test-fleet")

	err := InitFleetRepo(fleetDir, "test-fleet")
	if err != nil {
		t.Fatalf("InitFleetRepo failed: %v", err)
	}

	// Verify directory structure
	expectedDirs := []string{
		"clusters",
		"infrastructure/base",
		"infrastructure/overlays/cloud",
		"infrastructure/overlays/bare-metal",
		"infrastructure/overlays/local",
		"apps/base",
		"apps/overlays",
		"secrets",
	}

	for _, d := range expectedDirs {
		path := filepath.Join(fleetDir, d)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected directory %s to exist", d)
		}
	}

	// Verify key files exist
	expectedFiles := []string{
		"apps/base/kustomization.yaml",
		"apps/base/namespace.yaml",
		"apps/base/helmrelease.yaml",
		"apps/base/helm-repository.yaml",
		"infrastructure/base/kustomization.yaml",
		".sops.yaml",
		".gitignore",
		"README.md",
	}

	for _, f := range expectedFiles {
		path := filepath.Join(fleetDir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %s to exist", f)
		}
	}
}

func TestAddCluster(t *testing.T) {
	tmpDir := t.TempDir()
	fleetDir := filepath.Join(tmpDir, "test-fleet")

	// Init first
	if err := InitFleetRepo(fleetDir, "test-fleet"); err != nil {
		t.Fatalf("InitFleetRepo failed: %v", err)
	}

	// Add a cluster
	cfg := ClusterConfig{
		Name:         "oci-arm",
		Arch:         "arm64",
		Infra:        "cloud",
		Nodes:        3,
		Resources:    "medium",
		StorageClass: "oci-bv",
		NebulaMode:   "lighthouse",
		NebulaPrefix: "10.42.0",
	}

	if err := AddCluster(fleetDir, cfg); err != nil {
		t.Fatalf("AddCluster failed: %v", err)
	}

	// Verify cluster files exist
	expectedFiles := []string{
		"clusters/oci-arm/cluster-settings.yaml",
		"clusters/oci-arm/infrastructure.yaml",
		"clusters/oci-arm/apps.yaml",
		"apps/overlays/oci-arm/kustomization.yaml",
		"apps/overlays/oci-arm/values-patch.yaml",
	}

	for _, f := range expectedFiles {
		path := filepath.Join(fleetDir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %s to exist", f)
		}
	}

	// Verify values patch has correct content
	valuesPath := filepath.Join(fleetDir, "apps/overlays/oci-arm/values-patch.yaml")
	data, err := os.ReadFile(valuesPath)
	if err != nil {
		t.Fatalf("failed to read values-patch: %v", err)
	}

	content := string(data)
	if !contains(content, "count: 3") {
		t.Error("values-patch should contain 'count: 3'")
	}
	if !contains(content, `cpu: "500m"`) {
		t.Error("values-patch should contain medium preset CPU limit")
	}
	if !contains(content, "oci-bv") {
		t.Error("values-patch should contain storage class")
	}
}

func TestAddClusterInvalidPreset(t *testing.T) {
	tmpDir := t.TempDir()
	fleetDir := filepath.Join(tmpDir, "test-fleet")

	if err := InitFleetRepo(fleetDir, "test-fleet"); err != nil {
		t.Fatalf("InitFleetRepo failed: %v", err)
	}

	cfg := ClusterConfig{
		Name:      "bad",
		Resources: "xxxl",
	}

	err := AddCluster(fleetDir, cfg)
	if err == nil {
		t.Error("AddCluster should fail with invalid preset")
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && // avoid false positives
		len(s) >= len(substr) &&
		indexOf(s, substr) >= 0
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
