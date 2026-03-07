package fleet

import (
	"os"
	"path/filepath"
	"testing"
)

func assertGolden(t *testing.T, goldenPath, actual string) {
	t.Helper()
	if os.Getenv("LAUNCH_UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0755); err != nil {
			t.Fatalf("creating golden dir: %v", err)
		}
		if err := os.WriteFile(goldenPath, []byte(actual), 0644); err != nil {
			t.Fatalf("updating golden file: %v", err)
		}
		return
	}
	expected, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("reading golden file %s: %v\nRun with LAUNCH_UPDATE_GOLDEN=1 to create", goldenPath, err)
	}
	if actual != string(expected) {
		t.Errorf("output differs from golden file %s\n--- got ---\n%s\n--- want ---\n%s", goldenPath, actual, string(expected))
	}
}

func addClusterForGolden(t *testing.T, cfg ClusterConfig) string {
	t.Helper()
	tmpDir := t.TempDir()
	fleetDir := filepath.Join(tmpDir, "fleet")
	if err := InitFleetRepo(fleetDir, "golden-test"); err != nil {
		t.Fatalf("InitFleetRepo failed: %v", err)
	}
	if err := AddCluster(fleetDir, cfg); err != nil {
		t.Fatalf("AddCluster failed: %v", err)
	}
	return filepath.Join(fleetDir, "clusters", cfg.Name)
}

func TestAddClusterSmallGolden(t *testing.T) {
	clusterDir := addClusterForGolden(t, ClusterConfig{
		Name:         "test-cluster",
		Arch:         "arm64",
		Infra:        "bare-metal",
		Nodes:        3,
		Resources:    "small",
		NebulaMode:   "disabled",
		NebulaPrefix: "10.42.0",
	})
	data, err := os.ReadFile(filepath.Join(clusterDir, "config", "cluster-settings.yaml"))
	if err != nil {
		t.Fatalf("reading config/cluster-settings.yaml: %v", err)
	}
	assertGolden(t, "testdata/golden/small-cluster-settings.yaml", string(data))
}

func TestAddClusterMediumGolden(t *testing.T) {
	clusterDir := addClusterForGolden(t, ClusterConfig{
		Name:         "test-cluster",
		Arch:         "amd64",
		Infra:        "cloud",
		Nodes:        3,
		Resources:    "medium",
		NebulaMode:   "disabled",
		NebulaPrefix: "10.42.0",
	})
	data, err := os.ReadFile(filepath.Join(clusterDir, "config", "cluster-settings.yaml"))
	if err != nil {
		t.Fatalf("reading config/cluster-settings.yaml: %v", err)
	}
	assertGolden(t, "testdata/golden/medium-cluster-settings.yaml", string(data))
}

func TestAddClusterLargeGolden(t *testing.T) {
	clusterDir := addClusterForGolden(t, ClusterConfig{
		Name:         "test-cluster",
		Arch:         "amd64",
		Infra:        "cloud",
		Nodes:        3,
		Resources:    "large",
		NebulaMode:   "disabled",
		NebulaPrefix: "10.42.0",
	})
	data, err := os.ReadFile(filepath.Join(clusterDir, "config", "cluster-settings.yaml"))
	if err != nil {
		t.Fatalf("reading config/cluster-settings.yaml: %v", err)
	}
	assertGolden(t, "testdata/golden/large-cluster-settings.yaml", string(data))
}

func TestAddClusterInfraGolden(t *testing.T) {
	clusterDir := addClusterForGolden(t, ClusterConfig{
		Name:         "test-cluster",
		Arch:         "arm64",
		Infra:        "bare-metal",
		Nodes:        3,
		Resources:    "small",
		NebulaMode:   "disabled",
		NebulaPrefix: "10.42.0",
	})
	data, err := os.ReadFile(filepath.Join(clusterDir, "infrastructure.yaml"))
	if err != nil {
		t.Fatalf("reading infrastructure.yaml: %v", err)
	}
	assertGolden(t, "testdata/golden/infrastructure.yaml", string(data))
}

func TestAddClusterAppsGolden(t *testing.T) {
	clusterDir := addClusterForGolden(t, ClusterConfig{
		Name:         "test-cluster",
		Arch:         "arm64",
		Infra:        "bare-metal",
		Nodes:        3,
		Resources:    "small",
		NebulaMode:   "disabled",
		NebulaPrefix: "10.42.0",
	})
	data, err := os.ReadFile(filepath.Join(clusterDir, "apps.yaml"))
	if err != nil {
		t.Fatalf("reading apps.yaml: %v", err)
	}
	assertGolden(t, "testdata/golden/apps.yaml", string(data))
}

func TestAddClusterConfigGolden(t *testing.T) {
	clusterDir := addClusterForGolden(t, ClusterConfig{
		Name:         "test-cluster",
		Arch:         "arm64",
		Infra:        "bare-metal",
		Nodes:        3,
		Resources:    "small",
		NebulaMode:   "disabled",
		NebulaPrefix: "10.42.0",
	})
	data, err := os.ReadFile(filepath.Join(clusterDir, "config.yaml"))
	if err != nil {
		t.Fatalf("reading config.yaml: %v", err)
	}
	assertGolden(t, "testdata/golden/config.yaml", string(data))
}
