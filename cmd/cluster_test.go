package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/disentangle-network/launch/internal/exec"
	"github.com/disentangle-network/launch/internal/paths"
)

func setupFleetDir(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	fleetDir := filepath.Join(tmp, "fleet")
	for _, d := range []string{
		filepath.Join(fleetDir, "apps", "base"),
		filepath.Join(fleetDir, "clusters"),
		filepath.Join(fleetDir, "infrastructure", "base"),
	} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}
	return fleetDir
}

func TestClusterAddSmall(t *testing.T) {
	fleetDir := setupFleetDir(t)
	mock := exec.NewMockExecutor()
	p := paths.NewWithHome(filepath.Dir(fleetDir), nil)
	var buf bytes.Buffer

	err := ClusterAdd(ClusterAddParams{
		Exec:         mock,
		Paths:        p,
		Stdout:       &buf,
		Name:         "edge-01",
		Arch:         "arm64",
		Infra:        "bare-metal",
		Nodes:        1,
		Resources:    "small",
		NebulaMode:   "disabled",
		NebulaPrefix: "10.42.0",
		FleetDir:     fleetDir,
	})
	if err != nil {
		t.Fatalf("ClusterAdd() returned error: %v", err)
	}

	// Verify cluster-settings.yaml
	settingsPath := filepath.Join(fleetDir, "clusters", "edge-01", "config", "cluster-settings.yaml")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("failed to read cluster-settings.yaml: %v", err)
	}
	settings := string(data)
	if !strings.Contains(settings, `cpu_limit: "250m"`) {
		t.Errorf("expected small preset cpu_limit 250m in settings, got: %s", settings)
	}
	if !strings.Contains(settings, `memory_limit: "256Mi"`) {
		t.Errorf("expected small preset memory_limit 256Mi in settings, got: %s", settings)
	}
	if !strings.Contains(settings, `pow_difficulty: "4"`) {
		t.Errorf("expected small preset pow_difficulty 4 in settings, got: %s", settings)
	}

	// Verify infrastructure.yaml created
	infraPath := filepath.Join(fleetDir, "clusters", "edge-01", "infrastructure.yaml")
	if _, err := os.Stat(infraPath); os.IsNotExist(err) {
		t.Error("infrastructure.yaml not created")
	}

	// Verify apps.yaml created
	appsPath := filepath.Join(fleetDir, "clusters", "edge-01", "apps.yaml")
	if _, err := os.Stat(appsPath); os.IsNotExist(err) {
		t.Error("apps.yaml not created")
	}
}

func TestClusterAddMedium(t *testing.T) {
	fleetDir := setupFleetDir(t)
	mock := exec.NewMockExecutor()
	p := paths.NewWithHome(filepath.Dir(fleetDir), nil)
	var buf bytes.Buffer

	err := ClusterAdd(ClusterAddParams{
		Exec:         mock,
		Paths:        p,
		Stdout:       &buf,
		Name:         "dev",
		Arch:         "amd64",
		Infra:        "cloud",
		Nodes:        3,
		Resources:    "medium",
		NebulaMode:   "disabled",
		NebulaPrefix: "10.42.0",
		FleetDir:     fleetDir,
	})
	if err != nil {
		t.Fatalf("ClusterAdd() returned error: %v", err)
	}

	settingsPath := filepath.Join(fleetDir, "clusters", "dev", "config", "cluster-settings.yaml")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("failed to read cluster-settings.yaml: %v", err)
	}
	settings := string(data)
	if !strings.Contains(settings, `cpu_limit: "500m"`) {
		t.Errorf("expected medium preset cpu_limit 500m, got: %s", settings)
	}
	if !strings.Contains(settings, `memory_limit: "512Mi"`) {
		t.Errorf("expected medium preset memory_limit 512Mi, got: %s", settings)
	}
	if !strings.Contains(settings, `pow_difficulty: "8"`) {
		t.Errorf("expected medium preset pow_difficulty 8, got: %s", settings)
	}
}

func TestClusterAddWithNebula(t *testing.T) {
	fleetDir := setupFleetDir(t)
	mock := exec.NewMockExecutor()
	p := paths.NewWithHome(filepath.Dir(fleetDir), nil)
	var buf bytes.Buffer

	err := ClusterAdd(ClusterAddParams{
		Exec:         mock,
		Paths:        p,
		Stdout:       &buf,
		Name:         "lh-01",
		Arch:         "amd64",
		Infra:        "cloud",
		Nodes:        3,
		Resources:    "medium",
		NebulaMode:   "lighthouse",
		NebulaPrefix: "10.42.0",
		FleetDir:     fleetDir,
	})
	if err != nil {
		t.Fatalf("ClusterAdd() returned error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "nebula: lighthouse") {
		t.Errorf("expected nebula lighthouse in output, got: %s", out)
	}
	if !strings.Contains(out, "mesh add --cluster lh-01 --lighthouse") {
		t.Errorf("expected lighthouse hint in output, got: %s", out)
	}
}

func TestClusterAddInvalidPreset(t *testing.T) {
	fleetDir := setupFleetDir(t)
	mock := exec.NewMockExecutor()
	p := paths.NewWithHome(filepath.Dir(fleetDir), nil)
	var buf bytes.Buffer

	err := ClusterAdd(ClusterAddParams{
		Exec:         mock,
		Paths:        p,
		Stdout:       &buf,
		Name:         "bad",
		Arch:         "amd64",
		Infra:        "cloud",
		Nodes:        3,
		Resources:    "invalid",
		NebulaMode:   "disabled",
		NebulaPrefix: "10.42.0",
		FleetDir:     fleetDir,
	})
	if err == nil {
		t.Fatal("expected error for invalid resource preset, got nil")
	}
	if !strings.Contains(err.Error(), "unknown resource preset") {
		t.Errorf("expected 'unknown resource preset' error, got: %v", err)
	}
}

func TestClusterAddMissingFleetDir(t *testing.T) {
	tmp := t.TempDir()
	nonexistent := filepath.Join(tmp, "does-not-exist")
	mock := exec.NewMockExecutor()
	p := paths.NewWithHome(tmp, nil)
	var buf bytes.Buffer

	err := ClusterAdd(ClusterAddParams{
		Exec:         mock,
		Paths:        p,
		Stdout:       &buf,
		Name:         "test",
		Arch:         "amd64",
		Infra:        "cloud",
		Nodes:        3,
		Resources:    "medium",
		NebulaMode:   "disabled",
		NebulaPrefix: "10.42.0",
		FleetDir:     nonexistent,
	})
	if err == nil {
		t.Fatal("expected error for missing fleet dir, got nil")
	}
	if !strings.Contains(err.Error(), "fleet directory not found") {
		t.Errorf("expected 'fleet directory not found' error, got: %v", err)
	}
}

func TestClusterListNoClusters(t *testing.T) {
	tmp := t.TempDir()
	fleetDir := filepath.Join(tmp, "fleet")
	if err := os.MkdirAll(filepath.Join(fleetDir, "clusters"), 0755); err != nil {
		t.Fatal(err)
	}

	p := paths.NewWithHome(tmp, nil)
	var buf bytes.Buffer

	err := ClusterList(ClusterListParams{
		Paths:    p,
		Stdout:   &buf,
		FleetDir: fleetDir,
	})
	if err != nil {
		t.Fatalf("ClusterList() returned error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "No clusters configured") {
		t.Errorf("expected 'No clusters configured', got: %s", out)
	}
}

func TestClusterListWithClusters(t *testing.T) {
	tmp := t.TempDir()
	fleetDir := filepath.Join(tmp, "fleet")
	for _, name := range []string{"dev", "prod"} {
		if err := os.MkdirAll(filepath.Join(fleetDir, "clusters", name), 0755); err != nil {
			t.Fatal(err)
		}
	}

	p := paths.NewWithHome(tmp, nil)
	var buf bytes.Buffer

	err := ClusterList(ClusterListParams{
		Paths:    p,
		Stdout:   &buf,
		FleetDir: fleetDir,
	})
	if err != nil {
		t.Fatalf("ClusterList() returned error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "dev") {
		t.Errorf("expected 'dev' in output, got: %s", out)
	}
	if !strings.Contains(out, "prod") {
		t.Errorf("expected 'prod' in output, got: %s", out)
	}
}

func TestClusterRemoveSuccess(t *testing.T) {
	tmp := t.TempDir()
	fleetDir := filepath.Join(tmp, "fleet")
	clusterDir := filepath.Join(fleetDir, "clusters", "doomed")
	if err := os.MkdirAll(clusterDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Place a file inside so we verify recursive removal
	if err := os.WriteFile(filepath.Join(clusterDir, "cluster-settings.yaml"), []byte("name: doomed\n"), 0644); err != nil {
		t.Fatal(err)
	}

	p := paths.NewWithHome(tmp, nil)
	var buf bytes.Buffer

	err := ClusterRemove(ClusterRemoveParams{
		Paths:    p,
		Stdout:   &buf,
		Name:     "doomed",
		FleetDir: fleetDir,
	})
	if err != nil {
		t.Fatalf("ClusterRemove() returned error: %v", err)
	}

	if _, err := os.Stat(clusterDir); !os.IsNotExist(err) {
		t.Error("expected cluster directory to be removed, but it still exists")
	}
}

func TestClusterRemoveWithSecrets(t *testing.T) {
	tmp := t.TempDir()
	fleetDir := filepath.Join(tmp, "fleet")
	clusterDir := filepath.Join(fleetDir, "clusters", "doomed")
	secretsDir := filepath.Join(fleetDir, "secrets", "doomed")
	for _, d := range []string{clusterDir, secretsDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}

	p := paths.NewWithHome(tmp, nil)
	var buf bytes.Buffer

	err := ClusterRemove(ClusterRemoveParams{
		Paths:    p,
		Stdout:   &buf,
		Name:     "doomed",
		FleetDir: fleetDir,
	})
	if err != nil {
		t.Fatalf("ClusterRemove() returned error: %v", err)
	}

	if _, err := os.Stat(clusterDir); !os.IsNotExist(err) {
		t.Error("expected cluster directory to be removed, but it still exists")
	}
	if _, err := os.Stat(secretsDir); !os.IsNotExist(err) {
		t.Error("expected secrets directory to be removed, but it still exists")
	}

	out := buf.String()
	if !strings.Contains(out, "Removing secrets for 'doomed'") {
		t.Errorf("expected secrets removal message, got: %s", out)
	}
}

func TestClusterRemoveNotFound(t *testing.T) {
	tmp := t.TempDir()
	fleetDir := filepath.Join(tmp, "fleet")
	if err := os.MkdirAll(filepath.Join(fleetDir, "clusters"), 0755); err != nil {
		t.Fatal(err)
	}

	p := paths.NewWithHome(tmp, nil)
	var buf bytes.Buffer

	err := ClusterRemove(ClusterRemoveParams{
		Paths:    p,
		Stdout:   &buf,
		Name:     "nonexistent",
		FleetDir: fleetDir,
	})
	if err == nil {
		t.Fatal("expected error for nonexistent cluster, got nil")
	}
	if !strings.Contains(err.Error(), "cluster 'nonexistent' not found in fleet") {
		t.Errorf("expected 'not found in fleet' error, got: %v", err)
	}
}

func TestClusterRemoveOutputMessages(t *testing.T) {
	tmp := t.TempDir()
	fleetDir := filepath.Join(tmp, "fleet")
	if err := os.MkdirAll(filepath.Join(fleetDir, "clusters", "test-cluster"), 0755); err != nil {
		t.Fatal(err)
	}

	p := paths.NewWithHome(tmp, nil)
	var buf bytes.Buffer

	err := ClusterRemove(ClusterRemoveParams{
		Paths:    p,
		Stdout:   &buf,
		Name:     "test-cluster",
		FleetDir: fleetDir,
	})
	if err != nil {
		t.Fatalf("ClusterRemove() returned error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Removing cluster 'test-cluster' from fleet...") {
		t.Errorf("expected removing message, got: %s", out)
	}
	if !strings.Contains(out, "Cluster 'test-cluster' removed.") {
		t.Errorf("expected removed confirmation, got: %s", out)
	}
	if !strings.Contains(out, "cluster list") {
		t.Errorf("expected 'cluster list' hint, got: %s", out)
	}
	if !strings.Contains(out, "status") {
		t.Errorf("expected 'status' hint, got: %s", out)
	}
}

func TestClusterListFleetDirResolution(t *testing.T) {
	tmp := t.TempDir()
	// Use default path structure: <home>/DISENTANGLE-NETWORK/fleet-deploy
	resolvedFleet := filepath.Join(tmp, "DISENTANGLE-NETWORK", "fleet-deploy")
	if err := os.MkdirAll(filepath.Join(resolvedFleet, "clusters", "staging"), 0755); err != nil {
		t.Fatal(err)
	}

	p := paths.NewWithHome(tmp, nil)
	var buf bytes.Buffer

	// Empty FleetDir triggers resolution via Paths
	err := ClusterList(ClusterListParams{
		Paths:    p,
		Stdout:   &buf,
		FleetDir: "",
	})
	if err != nil {
		t.Fatalf("ClusterList() returned error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "staging") {
		t.Errorf("expected 'staging' from resolved fleet dir, got: %s", out)
	}
}
