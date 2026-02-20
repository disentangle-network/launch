package fleet

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitFleetRepo(t *testing.T) {
	tmpDir := t.TempDir()
	fleetDir := filepath.Join(tmpDir, "test-fleet")

	err := InitFleetRepo(fleetDir, "test-fleet")
	if err != nil {
		t.Fatalf("InitFleetRepo failed: %v", err)
	}

	expectedDirs := []string{
		"clusters",
		"apps/base",
		"secrets",
	}

	for _, d := range expectedDirs {
		path := filepath.Join(fleetDir, d)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected directory %s to exist", d)
		}
	}

	expectedFiles := []string{
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

	if err := InitFleetRepo(fleetDir, "test-fleet"); err != nil {
		t.Fatalf("InitFleetRepo failed: %v", err)
	}

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

	expectedFiles := []string{
		"clusters/oci-arm/cluster-settings.yaml",
		"clusters/oci-arm/infrastructure.yaml",
		"clusters/oci-arm/apps.yaml",
	}

	for _, f := range expectedFiles {
		path := filepath.Join(fleetDir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %s to exist", f)
		}
	}

	// Verify cluster-settings has correct content
	data, err := os.ReadFile(filepath.Join(fleetDir, "clusters/oci-arm/cluster-settings.yaml"))
	if err != nil {
		t.Fatalf("failed to read cluster-settings: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, `nodes: "3"`) {
		t.Error("cluster-settings should contain nodes: 3")
	}
	if !strings.Contains(content, `cpu_limit: "500m"`) {
		t.Error("cluster-settings should contain medium preset CPU limit")
	}

	// Verify apps.yaml points to correct path
	data, err = os.ReadFile(filepath.Join(fleetDir, "clusters/oci-arm/apps.yaml"))
	if err != nil {
		t.Fatalf("failed to read apps.yaml: %v", err)
	}
	if !strings.Contains(string(data), "path: ./apps/disentangle") {
		t.Error("apps.yaml should reference ./apps/disentangle")
	}

	// Verify infrastructure.yaml points to correct path
	data, err = os.ReadFile(filepath.Join(fleetDir, "clusters/oci-arm/infrastructure.yaml"))
	if err != nil {
		t.Fatalf("failed to read infrastructure.yaml: %v", err)
	}
	if !strings.Contains(string(data), "path: ./infrastructure/controllers") {
		t.Error("infrastructure.yaml should reference ./infrastructure/controllers")
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
