package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/disentangle-network/launch/internal/config"
	"github.com/disentangle-network/launch/internal/exec"
	"github.com/disentangle-network/launch/internal/paths"
)

func TestDoctorNoConfig(t *testing.T) {
	tmp := t.TempDir()
	mock := exec.NewMockExecutor()
	p := paths.NewWithHome(tmp, nil)

	var buf bytes.Buffer
	err := Doctor(DoctorParams{
		Exec:     mock,
		Paths:    p,
		Stdout:   &buf,
		FleetDir: filepath.Join(tmp, "nonexistent-fleet"),
		CfgFile:  filepath.Join(tmp, "nonexistent-config.yaml"),
		Verbose:  false,
	})
	if err == nil {
		t.Fatal("Doctor() should return error when checks fail")
	}
	if !strings.Contains(err.Error(), "failing check") {
		t.Errorf("unexpected error: %v", err)
	}

	output := buf.String()

	// Config should fail (no config file)
	if !strings.Contains(output, "Config") || !strings.Contains(output, "FAIL") {
		t.Error("expected Config FAIL in output")
	}

	// Fleet repo should fail (nonexistent directory)
	if !strings.Contains(output, "Fleet repo") {
		t.Error("expected Fleet repo check in output")
	}

	// Mesh CA should fail (no CA in temp dir)
	if !strings.Contains(output, "Mesh CA") {
		t.Error("expected Mesh CA check in output")
	}

	// Should have failures in summary
	if !strings.Contains(output, "failures") {
		t.Error("expected 'failures' in summary output")
	}
}

func TestDoctorWithFleet(t *testing.T) {
	tmp := t.TempDir()

	// Set up fleet structure
	fleetDir := filepath.Join(tmp, "fleet")
	dirs := []string{
		filepath.Join(fleetDir, ".git"),
		filepath.Join(fleetDir, "apps", "base"),
		filepath.Join(fleetDir, "clusters", "test-cluster"),
		filepath.Join(fleetDir, "secrets", "test-cluster"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// Create cluster-settings.yaml
	settingsContent := "apiVersion: v1\nkind: ConfigMap\ndata:\n  nodes: \"3\"\n"
	if err := os.WriteFile(
		filepath.Join(fleetDir, "clusters", "test-cluster", "cluster-settings.yaml"),
		[]byte(settingsContent), 0o600,
	); err != nil {
		t.Fatal(err)
	}

	// Create genesis-config.yaml for secrets check
	if err := os.WriteFile(
		filepath.Join(fleetDir, "secrets", "test-cluster", "genesis-config.yaml"),
		[]byte("kind: Secret\n"), 0o600,
	); err != nil {
		t.Fatal(err)
	}

	// Create config file
	cfgPath := filepath.Join(tmp, "config.yaml")
	cfg := &config.Config{FleetDir: fleetDir}
	if err := config.Save(cfg, cfgPath); err != nil {
		t.Fatal(err)
	}

	// Create nebula CA key
	nebulaDir := filepath.Join(tmp, ".config", "disentangle", "nebula-ca")
	if err := os.MkdirAll(nebulaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nebulaDir, "ca.key"), []byte("fake-key"), 0o600); err != nil {
		t.Fatal(err)
	}

	mock := exec.NewMockExecutor()
	p := paths.NewWithHome(tmp, cfg)

	var buf bytes.Buffer
	err := Doctor(DoctorParams{
		Exec:     mock,
		Paths:    p,
		Stdout:   &buf,
		FleetDir: fleetDir,
		CfgFile:  cfgPath,
		Verbose:  false,
	})
	if err != nil {
		t.Fatalf("Doctor() returned unexpected error: %v", err)
	}

	output := buf.String()

	// Config should be OK
	if !strings.Contains(output, "Config") {
		t.Error("expected Config check in output")
	}

	// Fleet repo should be OK with 1 cluster
	if !strings.Contains(output, "Fleet repo") || !strings.Contains(output, "1 clusters") {
		t.Errorf("expected Fleet repo OK with 1 cluster, got:\n%s", output)
	}

	// Clusters should be OK (1/1)
	if !strings.Contains(output, "Clusters") || !strings.Contains(output, "1/1") {
		t.Errorf("expected Clusters 1/1, got:\n%s", output)
	}

	// Secrets should be OK (1/1)
	if !strings.Contains(output, "Secrets") || !strings.Contains(output, "1/1") {
		t.Errorf("expected Secrets 1/1, got:\n%s", output)
	}

	// Mesh CA should be OK
	if !strings.Contains(output, "Mesh CA") || !strings.Contains(output, "CA key present") {
		t.Errorf("expected Mesh CA OK, got:\n%s", output)
	}
}

func TestDoctorOutputFormat(t *testing.T) {
	tmp := t.TempDir()
	mock := exec.NewMockExecutor()
	p := paths.NewWithHome(tmp, nil)

	var buf bytes.Buffer
	err := Doctor(DoctorParams{
		Exec:     mock,
		Paths:    p,
		Stdout:   &buf,
		FleetDir: filepath.Join(tmp, "no-fleet"),
		CfgFile:  filepath.Join(tmp, "no-config.yaml"),
		Verbose:  false,
	})
	if err == nil {
		t.Fatal("Doctor() should return error when checks fail")
	}

	output := buf.String()

	// Verify the summary line format: "X checks passed, Y warnings, Z failures."
	if !strings.Contains(output, "checks passed,") ||
		!strings.Contains(output, "warnings,") ||
		!strings.Contains(output, "failures.") {
		t.Errorf("expected summary line 'X checks passed, Y warnings, Z failures.' in output:\n%s", output)
	}

	// Verify the header line
	if !strings.Contains(output, "==> Doctor: checking deployment pipeline health...") {
		t.Errorf("expected header line in output:\n%s", output)
	}

	// Verify all 7 check names appear
	expectedChecks := []string{"Config", "Tools", "Credentials", "Fleet repo", "Clusters", "Secrets", "Mesh CA"}
	for _, name := range expectedChecks {
		if !strings.Contains(output, name) {
			t.Errorf("expected check %q in output:\n%s", name, output)
		}
	}
}

func TestDoctorViaCobra(t *testing.T) {
	tmp := t.TempDir()

	err := execRootCmd([]string{"doctor", "--fleet-dir", tmp})
	// Doctor should return an error when checks fail.
	if err == nil {
		t.Fatal("expected doctor to return error for failing checks")
	}
}

func TestDoctorWarningsOnly(t *testing.T) {
	tmp := t.TempDir()

	// Set up fleet structure with .git and apps/base but no clusters
	// (clusters check returns "warn", secrets check returns "warn")
	fleetDir := filepath.Join(tmp, "fleet")
	dirs := []string{
		filepath.Join(fleetDir, ".git"),
		filepath.Join(fleetDir, "apps", "base"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// Create valid config file
	cfgPath := filepath.Join(tmp, "config.yaml")
	cfg := &config.Config{FleetDir: fleetDir}
	if err := config.Save(cfg, cfgPath); err != nil {
		t.Fatal(err)
	}

	// Create nebula CA key so Mesh CA passes
	nebulaDir := filepath.Join(tmp, ".config", "disentangle", "nebula-ca")
	if err := os.MkdirAll(nebulaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nebulaDir, "ca.key"), []byte("fake-key"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Mock executor returns errors by default, so credentials check => "warn"
	mock := exec.NewMockExecutor()
	p := paths.NewWithHome(tmp, cfg)

	var buf bytes.Buffer
	err := Doctor(DoctorParams{
		Exec:     mock,
		Paths:    p,
		Stdout:   &buf,
		FleetDir: fleetDir,
		CfgFile:  cfgPath,
		Verbose:  false,
	})

	// Warnings only -- Doctor should return nil
	if err != nil {
		t.Fatalf("Doctor() returned unexpected error for warnings-only scenario: %v", err)
	}

	output := buf.String()

	// Should have no FAIL entries
	if strings.Contains(output, "FAIL") {
		t.Errorf("expected no FAIL entries in warnings-only scenario, got:\n%s", output)
	}

	// Should have 0 failures in summary
	if !strings.Contains(output, "0 failures.") {
		t.Errorf("expected '0 failures.' in summary, got:\n%s", output)
	}
}

func TestStatusLabelUnknown(t *testing.T) {
	label := statusLabel("unknown")
	if label != "????" {
		t.Errorf("statusLabel(%q) = %q, want %q", "unknown", label, "????")
	}
}

func TestCheckConfigInvalidYAML(t *testing.T) {
	tmp := t.TempDir()

	// Create a config file with invalid YAML content
	cfgPath := filepath.Join(tmp, "bad-config.yaml")
	if err := os.WriteFile(cfgPath, []byte("!!!invalid\n\t::: yaml"), 0o600); err != nil {
		t.Fatal(err)
	}

	p := paths.NewWithHome(tmp, nil)
	check := checkConfig(DoctorParams{
		Exec:    exec.NewMockExecutor(),
		Paths:   p,
		CfgFile: cfgPath,
	})

	if check.Status != "fail" {
		t.Errorf("checkConfig status = %q, want %q", check.Status, "fail")
	}
	if !strings.Contains(check.Detail, "failed to load") {
		t.Errorf("checkConfig detail = %q, want it to contain %q", check.Detail, "failed to load")
	}
}

func TestCheckFleetRepoNoGit(t *testing.T) {
	tmp := t.TempDir()

	// Create fleet dir without .git
	fleetDir := filepath.Join(tmp, "fleet")
	if err := os.MkdirAll(fleetDir, 0o755); err != nil {
		t.Fatal(err)
	}

	check, clusterNames := checkFleetRepo(fleetDir)

	if check.Status != "fail" {
		t.Errorf("checkFleetRepo status = %q, want %q", check.Status, "fail")
	}
	if !strings.Contains(check.Detail, "not a git repository") {
		t.Errorf("checkFleetRepo detail = %q, want it to contain %q", check.Detail, "not a git repository")
	}
	if clusterNames != nil {
		t.Errorf("checkFleetRepo clusterNames = %v, want nil", clusterNames)
	}
}

func TestCheckFleetRepoNoAppsBase(t *testing.T) {
	tmp := t.TempDir()

	// Create fleet dir with .git but no apps/base
	fleetDir := filepath.Join(tmp, "fleet")
	if err := os.MkdirAll(filepath.Join(fleetDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	check, clusterNames := checkFleetRepo(fleetDir)

	if check.Status != "warn" {
		t.Errorf("checkFleetRepo status = %q, want %q", check.Status, "warn")
	}
	if !strings.Contains(check.Detail, "missing apps/base") {
		t.Errorf("checkFleetRepo detail = %q, want it to contain %q", check.Detail, "missing apps/base")
	}
	if clusterNames != nil {
		t.Errorf("checkFleetRepo clusterNames = %v, want nil", clusterNames)
	}
}

func TestCheckClustersPartialSettings(t *testing.T) {
	tmp := t.TempDir()

	fleetDir := filepath.Join(tmp, "fleet")
	clusterNames := []string{"cluster-a", "cluster-b"}

	// Create cluster dirs
	for _, name := range clusterNames {
		if err := os.MkdirAll(filepath.Join(fleetDir, "clusters", name), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// Only give cluster-a a settings file
	if err := os.WriteFile(
		filepath.Join(fleetDir, "clusters", "cluster-a", "cluster-settings.yaml"),
		[]byte("nodes: 3\n"), 0o600,
	); err != nil {
		t.Fatal(err)
	}

	check := checkClusters(fleetDir, clusterNames)

	if check.Status != "warn" {
		t.Errorf("checkClusters status = %q, want %q", check.Status, "warn")
	}
	if !strings.Contains(check.Detail, "1/2") {
		t.Errorf("checkClusters detail = %q, want it to contain %q", check.Detail, "1/2")
	}
}

func TestCheckSecretsPartial(t *testing.T) {
	tmp := t.TempDir()

	fleetDir := filepath.Join(tmp, "fleet")
	clusterNames := []string{"cluster-a", "cluster-b"}

	// Create secrets dir only for cluster-a
	if err := os.MkdirAll(filepath.Join(fleetDir, "secrets", "cluster-a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(fleetDir, "secrets", "cluster-a", "genesis-config.yaml"),
		[]byte("kind: Secret\n"), 0o600,
	); err != nil {
		t.Fatal(err)
	}

	check := checkSecrets(fleetDir, clusterNames)

	if check.Status != "warn" {
		t.Errorf("checkSecrets status = %q, want %q", check.Status, "warn")
	}
	if !strings.Contains(check.Detail, "1/2") {
		t.Errorf("checkSecrets detail = %q, want it to contain %q", check.Detail, "1/2")
	}
	if !strings.Contains(check.Fix, "cluster-b") {
		t.Errorf("checkSecrets fix = %q, want it to contain %q", check.Fix, "cluster-b")
	}
}
