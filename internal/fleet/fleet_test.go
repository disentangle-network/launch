package fleet

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Error-path tests for InitFleetRepo ---

func TestInitFleetRepo_MkdirAllError(t *testing.T) {
	// Create a file where the output directory would be, so MkdirAll fails
	// when trying to create subdirectories.
	tmpDir := t.TempDir()
	blocker := filepath.Join(tmpDir, "blocked")
	if err := os.WriteFile(blocker, []byte("not a dir"), 0600); err != nil {
		t.Fatal(err)
	}
	// "blocked" is a file, so "blocked/clusters" will fail in MkdirAll.
	err := InitFleetRepo(blocker, "test")
	if err == nil {
		t.Fatal("expected error when output path is a file, got nil")
	}
	if !strings.Contains(err.Error(), "failed to create directory") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestInitFleetRepo_ReadmeWriteError(t *testing.T) {
	tmpDir := t.TempDir()
	fleetDir := filepath.Join(tmpDir, "fleet")

	if err := InitFleetRepo(fleetDir, "test"); err != nil {
		t.Fatalf("initial InitFleetRepo failed: %v", err)
	}

	// Make README.md a directory so the write fails on the second call.
	readmePath := filepath.Join(fleetDir, "README.md")
	if err := os.Remove(readmePath); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(readmePath, 0750); err != nil {
		t.Fatal(err)
	}

	err := InitFleetRepo(fleetDir, "test")
	if err == nil {
		t.Fatal("expected error when README.md is a directory, got nil")
	}
	if !strings.Contains(err.Error(), "failed to write README") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestInitFleetRepo_TemplateFileCopy(t *testing.T) {
	tmpDir := t.TempDir()
	fleetDir := filepath.Join(tmpDir, "fleet")

	if err := InitFleetRepo(fleetDir, "template-test"); err != nil {
		t.Fatalf("InitFleetRepo failed: %v", err)
	}

	// Verify dot-prefixed files were renamed correctly.
	dotFiles := []string{".sops.yaml", ".gitignore"}
	for _, f := range dotFiles {
		path := filepath.Join(fleetDir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected dot-renamed file %s to exist", f)
		}
	}

	// Verify template subdirectory files were copied.
	templateFiles := []string{
		"infrastructure/base/kustomization.yaml",
		"apps/base/kustomization.yaml",
		"apps/base/helmrelease.yaml",
		"apps/base/namespace.yaml",
		"apps/base/helm-repository.yaml",
	}
	for _, f := range templateFiles {
		path := filepath.Join(fleetDir, f)
		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			t.Errorf("expected template file %s to exist", f)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("template file %s is empty", f)
		}
	}

	// Verify README contains the fleet name.
	data, err := os.ReadFile(filepath.Join(fleetDir, "README.md"))
	if err != nil {
		t.Fatalf("failed to read README: %v", err)
	}
	if !strings.Contains(string(data), "# template-test") {
		t.Error("README should contain the fleet name as heading")
	}
}

func TestInitFleetRepo_TemplateWriteError(t *testing.T) {
	tmpDir := t.TempDir()
	fleetDir := filepath.Join(tmpDir, "fleet")

	// Pre-create the fleet dir, then place a directory where a template file
	// would be written (e.g., .gitignore as a directory) so os.WriteFile fails
	// during the WalkDir copy.
	if err := os.MkdirAll(fleetDir, 0750); err != nil {
		t.Fatal(err)
	}
	// .gitignore will be written from "dot-gitignore" template. Make it a dir.
	if err := os.MkdirAll(filepath.Join(fleetDir, ".gitignore"), 0750); err != nil {
		t.Fatal(err)
	}

	err := InitFleetRepo(fleetDir, "test")
	if err == nil {
		t.Fatal("expected error when template dest is a directory, got nil")
	}
	if !strings.Contains(err.Error(), "failed to copy templates") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// --- Error-path tests for AddCluster ---

func TestAddCluster_MkdirAllError(t *testing.T) {
	tmpDir := t.TempDir()
	fleetDir := filepath.Join(tmpDir, "fleet")

	// Place a file where "clusters" directory should be so MkdirAll fails
	// when creating the cluster subdirectory.
	if err := os.MkdirAll(fleetDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fleetDir, "clusters"), []byte("blocker"), 0600); err != nil {
		t.Fatal(err)
	}

	cfg := ClusterConfig{
		Name:      "fail-cluster",
		Resources: "small",
	}
	err := AddCluster(fleetDir, cfg)
	if err == nil {
		t.Fatal("expected error when clusters path is a file, got nil")
	}
}

func TestAddCluster_RenderTemplateError(t *testing.T) {
	tmpDir := t.TempDir()
	fleetDir := filepath.Join(tmpDir, "fleet")

	if err := InitFleetRepo(fleetDir, "test"); err != nil {
		t.Fatal(err)
	}

	// Create the cluster directory, then make it read-only so renderTemplate
	// cannot create the cluster-settings.yaml file.
	clusterDir := filepath.Join(fleetDir, "clusters", "readonly-cluster")
	if err := os.MkdirAll(clusterDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(clusterDir, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(clusterDir, 0750) })

	cfg := ClusterConfig{
		Name:      "readonly-cluster",
		Resources: "small",
	}
	err := AddCluster(fleetDir, cfg)
	if err == nil {
		t.Fatal("expected error when cluster dir is read-only, got nil")
	}
}

func TestAddCluster_InfraWriteError(t *testing.T) {
	tmpDir := t.TempDir()
	fleetDir := filepath.Join(tmpDir, "fleet")

	if err := InitFleetRepo(fleetDir, "test"); err != nil {
		t.Fatal(err)
	}

	// Place a directory where infrastructure.yaml would be written so
	// os.WriteFile fails.
	clusterDir := filepath.Join(fleetDir, "clusters", "infra-fail")
	if err := os.MkdirAll(clusterDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(clusterDir, "infrastructure.yaml"), 0750); err != nil {
		t.Fatal(err)
	}

	cfg := ClusterConfig{
		Name:      "infra-fail",
		Resources: "small",
	}
	err := AddCluster(fleetDir, cfg)
	if err == nil {
		t.Fatal("expected error when infrastructure.yaml is a directory, got nil")
	}
}

func TestAddCluster_AppsWriteError(t *testing.T) {
	tmpDir := t.TempDir()
	fleetDir := filepath.Join(tmpDir, "fleet")

	if err := InitFleetRepo(fleetDir, "test"); err != nil {
		t.Fatal(err)
	}

	// Place a directory where apps.yaml would be written so os.WriteFile fails.
	clusterDir := filepath.Join(fleetDir, "clusters", "apps-fail")
	if err := os.MkdirAll(clusterDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(clusterDir, "apps.yaml"), 0750); err != nil {
		t.Fatal(err)
	}

	cfg := ClusterConfig{
		Name:      "apps-fail",
		Resources: "small",
	}
	err := AddCluster(fleetDir, cfg)
	if err == nil {
		t.Fatal("expected error when apps.yaml is a directory, got nil")
	}
}

// --- Error-path tests for renderTemplate ---

func TestRenderTemplate_ParseError(t *testing.T) {
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "out.yaml")

	// Malformed template should trigger a parse error.
	err := renderTemplate(outPath, "{{.Unclosed", nil)
	if err == nil {
		t.Fatal("expected template parse error, got nil")
	}
	if !strings.Contains(err.Error(), "template parse error") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRenderTemplate_CreateError(t *testing.T) {
	// Use a path inside a nonexistent directory so os.Create fails.
	err := renderTemplate("/nonexistent-dir-abc123/file.yaml", "{{.Foo}}", struct{ Foo string }{"bar"})
	if err == nil {
		t.Fatal("expected os.Create error, got nil")
	}
}

// --- Additional content verification tests ---

func TestAddCluster_AllPresets(t *testing.T) {
	presets := []string{"small", "medium", "large"}
	expectedCPU := map[string]string{
		"small":  "250m",
		"medium": "500m",
		"large":  "2",
	}

	for _, preset := range presets {
		t.Run(preset, func(t *testing.T) {
			tmpDir := t.TempDir()
			fleetDir := filepath.Join(tmpDir, "fleet")
			if err := InitFleetRepo(fleetDir, "test"); err != nil {
				t.Fatal(err)
			}

			cfg := ClusterConfig{
				Name:         "test-" + preset,
				Arch:         "arm64",
				Infra:        "bare-metal",
				Nodes:        5,
				Resources:    preset,
				StorageClass: "local-path",
				NebulaMode:   "lighthouse",
				NebulaPrefix: "10.42.0",
			}
			if err := AddCluster(fleetDir, cfg); err != nil {
				t.Fatalf("AddCluster(%s) failed: %v", preset, err)
			}

			data, err := os.ReadFile(filepath.Join(fleetDir, "clusters", "test-"+preset, "cluster-settings.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			content := string(data)
			if !strings.Contains(content, `cpu_limit: "`+expectedCPU[preset]+`"`) {
				t.Errorf("expected cpu_limit %s for preset %s", expectedCPU[preset], preset)
			}
			if !strings.Contains(content, `nebula_mode: "lighthouse"`) {
				t.Error("expected nebula_mode lighthouse")
			}
			if !strings.Contains(content, `nodes: "5"`) {
				t.Error("expected nodes 5")
			}
		})
	}
}

func TestAddCluster_InvalidPresetMessage(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := ClusterConfig{
		Name:      "bad",
		Resources: "xxxl",
	}
	err := AddCluster(tmpDir, cfg)
	if err == nil {
		t.Fatal("expected error for invalid preset")
	}
	if !strings.Contains(err.Error(), "xxxl") {
		t.Error("error message should contain the invalid preset name")
	}
	if !strings.Contains(err.Error(), "small") {
		t.Error("error message should list valid presets")
	}
}

func TestInitFleetRepo_AllDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	fleetDir := filepath.Join(tmpDir, "fleet")

	if err := InitFleetRepo(fleetDir, "test"); err != nil {
		t.Fatal(err)
	}

	expectedDirs := []string{
		"clusters",
		"infrastructure/base",
		"infrastructure/controllers",
		"infrastructure/overlays/cloud",
		"infrastructure/overlays/bare-metal",
		"infrastructure/overlays/local",
		"apps/base",
		"apps/disentangle",
		"apps/overlays",
		"secrets",
	}

	for _, d := range expectedDirs {
		path := filepath.Join(fleetDir, d)
		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			t.Errorf("expected directory %s to exist", d)
			continue
		}
		if !info.IsDir() {
			t.Errorf("expected %s to be a directory", d)
		}
	}
}

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

func TestAddCluster_KustomizationPathsExistAfterScaffold(t *testing.T) {
	tmpDir := t.TempDir()
	fleetDir := filepath.Join(tmpDir, "fleet")

	if err := InitFleetRepo(fleetDir, "invariant-test"); err != nil {
		t.Fatalf("InitFleetRepo failed: %v", err)
	}

	cfg := ClusterConfig{
		Name:         "path-check",
		Arch:         "amd64",
		Infra:        "cloud",
		Nodes:        3,
		Resources:    "small",
		StorageClass: "local-path",
		NebulaMode:   "node",
		NebulaPrefix: "10.42.0",
	}

	if err := AddCluster(fleetDir, cfg); err != nil {
		t.Fatalf("AddCluster failed: %v", err)
	}

	// Read infrastructure.yaml and apps.yaml, extract path: values,
	// and verify those paths exist in the scaffold.
	crFiles := []string{
		filepath.Join(fleetDir, "clusters", cfg.Name, "infrastructure.yaml"),
		filepath.Join(fleetDir, "clusters", cfg.Name, "apps.yaml"),
	}

	for _, crFile := range crFiles {
		data, err := os.ReadFile(crFile)
		if err != nil {
			t.Fatalf("failed to read %s: %v", filepath.Base(crFile), err)
		}

		var crPath string
		for _, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "path: ./") {
				crPath = strings.TrimPrefix(trimmed, "path: ./")
				break
			}
		}
		if crPath == "" {
			t.Fatalf("no 'path: ./' found in %s", filepath.Base(crFile))
		}

		absPath := filepath.Join(fleetDir, crPath)
		info, err := os.Stat(absPath)
		if os.IsNotExist(err) {
			t.Errorf("Kustomization CR %s references path %q but it does not exist in the scaffold",
				filepath.Base(crFile), crPath)
			continue
		}
		if err != nil {
			t.Fatalf("failed to stat %s: %v", crPath, err)
		}
		if !info.IsDir() {
			t.Errorf("Kustomization CR %s references path %q but it is not a directory",
				filepath.Base(crFile), crPath)
		}
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
