package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/disentangle-network/launch/internal/config"
	"github.com/disentangle-network/launch/internal/exec"
	"github.com/disentangle-network/launch/internal/paths"
)

func TestFleetInitCloneSequence(t *testing.T) {
	tmp := t.TempDir()
	fleetDir := filepath.Join(tmp, "fleet-deploy")

	mock := exec.NewMockExecutor()
	mock.ExpectRun("git clone https://github.com/disentangle-network/fleet.git "+fleetDir, "", nil)
	mock.ExpectRun("git remote remove origin", "", nil)
	mock.ExpectRun("git remote add template https://github.com/disentangle-network/fleet.git", "", nil)
	mock.ExpectRun("gh api user --jq .login", "testuser\n", nil)
	mock.ExpectRun("gh repo create testuser/fleet-deploy --private --source "+fleetDir+" --push", "", nil)

	cfgPath := filepath.Join(tmp, "config.yaml")
	var buf bytes.Buffer
	p := FleetInitParams{
		Exec:    mock,
		Paths:   paths.NewWithHome(tmp, nil),
		Stdout:  &buf,
		Dir:     fleetDir,
		CfgFile: cfgPath,
	}

	err := FleetInit(p)
	if err != nil {
		t.Fatalf("FleetInit returned error: %v", err)
	}

	// Verify clone was called
	mock.AssertCalled(t, "git clone https://github.com/disentangle-network/fleet.git "+fleetDir)

	// Verify remote removal and template add
	mock.AssertCalled(t, "git remote remove origin")
	mock.AssertCalled(t, "git remote add template https://github.com/disentangle-network/fleet.git")

	// Verify gh user detection
	mock.AssertCalled(t, "gh api user --jq .login")

	// Verify repo creation
	mock.AssertCalled(t, "gh repo create testuser/fleet-deploy --private --source "+fleetDir+" --push")

	// Verify SetDir was called with the fleet dir
	if mock.Dir != fleetDir {
		t.Errorf("expected SetDir(%q), got Dir=%q", fleetDir, mock.Dir)
	}

	// Verify output contains expected messages
	out := buf.String()
	if !strings.Contains(out, "Cloning fleet template") {
		t.Error("output missing 'Cloning fleet template'")
	}
	if !strings.Contains(out, "Creating private repo: testuser/fleet-deploy") {
		t.Error("output missing repo creation message")
	}
}

func TestFleetInitAlreadyExists(t *testing.T) {
	tmp := t.TempDir()
	fleetDir := filepath.Join(tmp, "fleet-deploy")

	// Create .git dir to simulate existing repo
	if err := os.MkdirAll(filepath.Join(fleetDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	mock := exec.NewMockExecutor()
	var buf bytes.Buffer
	p := FleetInitParams{
		Exec:   mock,
		Paths:  paths.NewWithHome(tmp, nil),
		Stdout: &buf,
		Dir:    fleetDir,
	}

	err := FleetInit(p)
	if err == nil {
		t.Fatal("expected error for existing fleet-deploy, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}

	// No commands should have been called
	mock.AssertCallCount(t, 0)
}

func TestFleetInitWithExplicitRemote(t *testing.T) {
	tmp := t.TempDir()
	fleetDir := filepath.Join(tmp, "fleet-deploy")

	mock := exec.NewMockExecutor()
	mock.ExpectRun("git clone https://github.com/disentangle-network/fleet.git "+fleetDir, "", nil)
	mock.ExpectRun("git remote remove origin", "", nil)
	mock.ExpectRun("git remote add template https://github.com/disentangle-network/fleet.git", "", nil)
	mock.ExpectRun("git remote add origin git@github.com:myorg/my-fleet.git", "", nil)

	cfgPath := filepath.Join(tmp, "config.yaml")
	var buf bytes.Buffer
	p := FleetInitParams{
		Exec:    mock,
		Paths:   paths.NewWithHome(tmp, nil),
		Stdout:  &buf,
		Dir:     fleetDir,
		Remote:  "git@github.com:myorg/my-fleet.git",
		CfgFile: cfgPath,
	}

	err := FleetInit(p)
	if err != nil {
		t.Fatalf("FleetInit returned error: %v", err)
	}

	// Verify explicit remote was added
	mock.AssertCalled(t, "git remote add origin git@github.com:myorg/my-fleet.git")

	// gh repo create should NOT have been called
	for _, c := range mock.Calls {
		if strings.HasPrefix(c.CommandString(), "gh repo create") {
			t.Error("gh repo create should not be called when explicit remote is provided")
		}
	}

	out := buf.String()
	if !strings.Contains(out, "Setting private remote: git@github.com:myorg/my-fleet.git") {
		t.Error("output missing explicit remote message")
	}
}

func TestFleetStatusNoClusters(t *testing.T) {
	tmp := t.TempDir()
	fleetDir := filepath.Join(tmp, "fleet-deploy")

	// Create .git and empty clusters/ dir
	if err := os.MkdirAll(filepath.Join(fleetDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(fleetDir, "clusters"), 0o755); err != nil {
		t.Fatal(err)
	}

	mock := exec.NewMockExecutor()
	var buf bytes.Buffer
	p := FleetStatusParams{
		Exec:   mock,
		Paths:  paths.NewWithHome(tmp, nil),
		Stdout: &buf,
		Dir:    fleetDir,
	}

	err := FleetStatus(p)
	if err != nil {
		t.Fatalf("FleetStatus returned error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Fleet repo:") {
		t.Error("output missing 'Fleet repo:'")
	}
	if !strings.Contains(out, "Clusters:") {
		t.Error("output missing 'Clusters:'")
	}

	// Verify git commands were called
	mock.AssertCalled(t, "git remote -v")
	mock.AssertCalled(t, "git status --short")
}

func TestFleetStatusWithClusters(t *testing.T) {
	tmp := t.TempDir()
	fleetDir := filepath.Join(tmp, "fleet-deploy")

	// Create .git and cluster directories
	if err := os.MkdirAll(filepath.Join(fleetDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(fleetDir, "clusters", "dev"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(fleetDir, "clusters", "prod"), 0o755); err != nil {
		t.Fatal(err)
	}

	mock := exec.NewMockExecutor()
	var buf bytes.Buffer
	p := FleetStatusParams{
		Exec:   mock,
		Paths:  paths.NewWithHome(tmp, nil),
		Stdout: &buf,
		Dir:    fleetDir,
	}

	err := FleetStatus(p)
	if err != nil {
		t.Fatalf("FleetStatus returned error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "dev") {
		t.Error("output missing cluster 'dev'")
	}
	if !strings.Contains(out, "prod") {
		t.Error("output missing cluster 'prod'")
	}
}

// ---------------------------------------------------------------------------
// Additional Fleet coverage tests
// ---------------------------------------------------------------------------

func TestFleetInitCloneFails(t *testing.T) {
	tmp := t.TempDir()
	fleetDir := filepath.Join(tmp, "fleet-deploy")

	mock := exec.NewMockExecutor()
	mock.ExpectRunWithResult(
		"git clone https://github.com/disentangle-network/fleet.git "+fleetDir,
		&exec.Result{Stderr: "clone error"},
		fmt.Errorf("exit status 128"),
	)

	cfgPath := filepath.Join(tmp, "config.yaml")
	var buf bytes.Buffer
	err := FleetInit(FleetInitParams{
		Exec:    mock,
		Paths:   paths.NewWithHome(tmp, nil),
		Stdout:  &buf,
		Dir:     fleetDir,
		CfgFile: cfgPath,
	})
	if err == nil {
		t.Fatal("expected error for clone failure, got nil")
	}
	if !strings.Contains(err.Error(), "clone failed") {
		t.Errorf("expected 'clone failed' error, got: %v", err)
	}
}

func TestFleetInitGhUserDetectionFails(t *testing.T) {
	tmp := t.TempDir()
	fleetDir := filepath.Join(tmp, "fleet-deploy")

	mock := exec.NewMockExecutor()
	mock.ExpectRun("git clone https://github.com/disentangle-network/fleet.git "+fleetDir, "", nil)
	mock.ExpectRun("git remote remove origin", "", nil)
	mock.ExpectRun("git remote add template https://github.com/disentangle-network/fleet.git", "", nil)
	// gh api user fails
	mock.ExpectRunWithResult("gh api user --jq .login",
		&exec.Result{Stderr: "not authenticated"},
		fmt.Errorf("exit status 1"),
	)

	cfgPath := filepath.Join(tmp, "config.yaml")
	var buf bytes.Buffer
	err := FleetInit(FleetInitParams{
		Exec:    mock,
		Paths:   paths.NewWithHome(tmp, nil),
		Stdout:  &buf,
		Dir:     fleetDir,
		CfgFile: cfgPath,
	})
	if err != nil {
		t.Fatalf("FleetInit should not error on gh user failure: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Could not detect owner") {
		t.Errorf("expected gh user detection failure message, got:\n%s", out)
	}
	if !strings.Contains(out, "git remote add origin <url>") {
		t.Errorf("expected manual remote instructions, got:\n%s", out)
	}
}

func TestFleetInitGhUserEmptyStdout(t *testing.T) {
	tmp := t.TempDir()
	fleetDir := filepath.Join(tmp, "fleet-deploy")

	mock := exec.NewMockExecutor()
	mock.ExpectRun("git clone https://github.com/disentangle-network/fleet.git "+fleetDir, "", nil)
	mock.ExpectRun("git remote remove origin", "", nil)
	mock.ExpectRun("git remote add template https://github.com/disentangle-network/fleet.git", "", nil)
	// gh api user returns empty string
	mock.ExpectRun("gh api user --jq .login", "  \n", nil)

	cfgPath := filepath.Join(tmp, "config.yaml")
	var buf bytes.Buffer
	err := FleetInit(FleetInitParams{
		Exec:    mock,
		Paths:   paths.NewWithHome(tmp, nil),
		Stdout:  &buf,
		Dir:     fleetDir,
		CfgFile: cfgPath,
	})
	if err != nil {
		t.Fatalf("FleetInit should not error on empty gh user: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Could not detect owner") {
		t.Errorf("expected gh user detection failure message, got:\n%s", out)
	}
}

func TestFleetInitGhRepoCreateFails(t *testing.T) {
	tmp := t.TempDir()
	fleetDir := filepath.Join(tmp, "fleet-deploy")

	mock := exec.NewMockExecutor()
	mock.ExpectRun("git clone https://github.com/disentangle-network/fleet.git "+fleetDir, "", nil)
	mock.ExpectRun("git remote remove origin", "", nil)
	mock.ExpectRun("git remote add template https://github.com/disentangle-network/fleet.git", "", nil)
	mock.ExpectRun("gh api user --jq .login", "testuser\n", nil)
	// gh repo create fails
	mock.ExpectRunWithResult(
		"gh repo create testuser/fleet-deploy --private --source "+fleetDir+" --push",
		&exec.Result{Stderr: "repo already exists"},
		fmt.Errorf("exit status 1"),
	)

	cfgPath := filepath.Join(tmp, "config.yaml")
	var buf bytes.Buffer
	err := FleetInit(FleetInitParams{
		Exec:    mock,
		Paths:   paths.NewWithHome(tmp, nil),
		Stdout:  &buf,
		Dir:     fleetDir,
		CfgFile: cfgPath,
	})
	if err != nil {
		t.Fatalf("FleetInit should not error on repo create failure: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "gh repo create failed") {
		t.Errorf("expected repo create failure message, got:\n%s", out)
	}
	if !strings.Contains(out, "git remote add origin <url>") {
		t.Errorf("expected manual remote instructions, got:\n%s", out)
	}
}

func TestFleetStatusNoGitDir(t *testing.T) {
	tmp := t.TempDir()
	fleetDir := filepath.Join(tmp, "fleet-deploy")
	// Do NOT create .git -- fleet not initialized

	mock := exec.NewMockExecutor()
	var buf bytes.Buffer
	err := FleetStatus(FleetStatusParams{
		Exec:   mock,
		Paths:  paths.NewWithHome(tmp, nil),
		Stdout: &buf,
		Dir:    fleetDir,
	})
	if err != nil {
		t.Fatalf("FleetStatus returned error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "No fleet-deploy repo found.") {
		t.Errorf("expected 'No fleet-deploy repo found.' message, got:\n%s", out)
	}
	// No commands should be called
	mock.AssertCallCount(t, 0)
}

func TestFleetStatusNoClustersDir(t *testing.T) {
	tmp := t.TempDir()
	fleetDir := filepath.Join(tmp, "fleet-deploy")
	// Create .git but NO clusters/ dir
	if err := os.MkdirAll(filepath.Join(fleetDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	mock := exec.NewMockExecutor()
	var buf bytes.Buffer
	err := FleetStatus(FleetStatusParams{
		Exec:   mock,
		Paths:  paths.NewWithHome(tmp, nil),
		Stdout: &buf,
		Dir:    fleetDir,
	})
	if err != nil {
		t.Fatalf("FleetStatus returned error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Fleet repo:") {
		t.Error("output missing 'Fleet repo:'")
	}
	if !strings.Contains(out, "Clusters:") {
		t.Error("output missing 'Clusters:'")
	}
	// Should not list any cluster names
	// Status section should still appear
	if !strings.Contains(out, "Status:") {
		t.Error("output missing 'Status:'")
	}
}

func TestFleetStatusClustersWithFiles(t *testing.T) {
	// Clusters dir contains both directories and files -- only dirs should be listed.
	tmp := t.TempDir()
	fleetDir := filepath.Join(tmp, "fleet-deploy")
	if err := os.MkdirAll(filepath.Join(fleetDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	clustersDir := filepath.Join(fleetDir, "clusters")
	if err := os.MkdirAll(filepath.Join(clustersDir, "dev"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(clustersDir, "README.md"), []byte("readme"), 0o644); err != nil {
		t.Fatal(err)
	}

	mock := exec.NewMockExecutor()
	var buf bytes.Buffer
	err := FleetStatus(FleetStatusParams{
		Exec:   mock,
		Paths:  paths.NewWithHome(tmp, nil),
		Stdout: &buf,
		Dir:    fleetDir,
	})
	if err != nil {
		t.Fatalf("FleetStatus returned error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "dev") {
		t.Error("output missing cluster 'dev'")
	}
	// README.md should NOT appear as a cluster
	if strings.Contains(out, "README") {
		t.Error("files should not be listed as clusters")
	}
}

// ---------------------------------------------------------------------------
// BUG 5: --dry-run must prevent all side effects
// ---------------------------------------------------------------------------

func TestFleetInitDryRun(t *testing.T) {
	tmp := t.TempDir()
	fleetDir := filepath.Join(tmp, "fleet-deploy")

	mock := exec.NewMockExecutor()
	mock.ExpectRun("gh api user --jq .login", "testuser\n", nil)
	var buf bytes.Buffer

	err := FleetInit(FleetInitParams{
		Exec:   mock,
		Paths:  paths.NewWithHome(tmp, nil),
		Stdout: &buf,
		Dir:    fleetDir,
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("FleetInit dry-run returned error: %v", err)
	}

	// Only the read-only gh api user call should have been made (owner detection)
	// No git clone, git remote, gh repo create, or config.Save calls
	for _, c := range mock.Calls {
		cmd := c.CommandString()
		if strings.HasPrefix(cmd, "git ") || strings.HasPrefix(cmd, "gh repo") {
			t.Errorf("dry-run should not execute side-effect commands, got: %s", cmd)
		}
	}

	// Fleet dir should NOT exist on disk
	if _, err := os.Stat(fleetDir); !os.IsNotExist(err) {
		t.Errorf("dry-run should not create fleet dir, but it exists")
	}

	// Output should contain [dry-run] prefixed lines
	out := buf.String()
	if !strings.Contains(out, "[dry-run] git clone") {
		t.Errorf("expected [dry-run] git clone message, got:\n%s", out)
	}
	if !strings.Contains(out, "[dry-run] git remote remove origin") {
		t.Errorf("expected [dry-run] remote remove message, got:\n%s", out)
	}
	if !strings.Contains(out, "[dry-run] Would save fleet_dir") {
		t.Errorf("expected [dry-run] config save message, got:\n%s", out)
	}
	if !strings.Contains(out, "[dry-run] gh repo create testuser/fleet-deploy") {
		t.Errorf("expected [dry-run] repo create message, got:\n%s", out)
	}
}

func TestFleetInitDryRunWithRemote(t *testing.T) {
	tmp := t.TempDir()
	fleetDir := filepath.Join(tmp, "fleet-deploy")

	mock := exec.NewMockExecutor()
	var buf bytes.Buffer

	err := FleetInit(FleetInitParams{
		Exec:   mock,
		Paths:  paths.NewWithHome(tmp, nil),
		Stdout: &buf,
		Dir:    fleetDir,
		Remote: "git@github.com:myorg/my-fleet.git",
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("FleetInit dry-run returned error: %v", err)
	}

	// With explicit remote, no gh api user call needed
	for _, c := range mock.Calls {
		cmd := c.CommandString()
		if strings.HasPrefix(cmd, "git ") || strings.HasPrefix(cmd, "gh ") {
			t.Errorf("dry-run with remote should not execute any commands, got: %s", cmd)
		}
	}

	out := buf.String()
	if !strings.Contains(out, "[dry-run] git remote add origin git@github.com:myorg/my-fleet.git") {
		t.Errorf("expected [dry-run] remote add message, got:\n%s", out)
	}
}

func TestFleetInitDryRunDoesNotWriteConfig(t *testing.T) {
	tmp := t.TempDir()
	fleetDir := filepath.Join(tmp, "fleet-deploy")

	// Create a config file and track its content
	cfgDir := filepath.Join(tmp, ".config", "launch")
	if err := os.MkdirAll(cfgDir, 0750); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(cfgDir, "config.yaml")
	original := "cluster_name: test\n"
	if err := os.WriteFile(cfgPath, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}

	mock := exec.NewMockExecutor()
	var buf bytes.Buffer

	err := FleetInit(FleetInitParams{
		Exec:    mock,
		Paths:   paths.NewWithHome(tmp, nil),
		Stdout:  &buf,
		Dir:     fleetDir,
		CfgFile: cfgPath,
		DryRun:  true,
	})
	if err != nil {
		t.Fatalf("FleetInit dry-run returned error: %v", err)
	}

	// Config file should be unchanged
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("could not read config: %v", err)
	}
	if string(data) != original {
		t.Errorf("dry-run should not modify config, got:\n%s", string(data))
	}
}

// ---------------------------------------------------------------------------
// BUG 6: fleet init should prefer config.github_org over gh api user
// ---------------------------------------------------------------------------

func TestFleetInitUsesGitHubOrg(t *testing.T) {
	tmp := t.TempDir()
	fleetDir := filepath.Join(tmp, "fleet-deploy")

	mock := exec.NewMockExecutor()
	mock.ExpectRun("git clone https://github.com/disentangle-network/fleet.git "+fleetDir, "", nil)
	mock.ExpectRun("git remote remove origin", "", nil)
	mock.ExpectRun("git remote add template https://github.com/disentangle-network/fleet.git", "", nil)
	mock.ExpectRun("gh repo create disentangle-network/fleet-deploy --private --source "+fleetDir+" --push", "", nil)

	cfg := &config.Config{GitHubOrg: "disentangle-network"}
	cfgPath := filepath.Join(tmp, "config.yaml")

	var buf bytes.Buffer
	err := FleetInit(FleetInitParams{
		Exec:    mock,
		Paths:   paths.NewWithHome(tmp, nil),
		Stdout:  &buf,
		Dir:     fleetDir,
		Config:  cfg,
		CfgFile: cfgPath,
	})
	if err != nil {
		t.Fatalf("FleetInit returned error: %v", err)
	}

	// Should use org, NOT call gh api user
	for _, c := range mock.Calls {
		if strings.Contains(c.CommandString(), "gh api user") {
			t.Error("should not call gh api user when github_org is set in config")
		}
	}

	// Should create repo under org
	mock.AssertCalled(t, "gh repo create disentangle-network/fleet-deploy --private --source "+fleetDir+" --push")

	out := buf.String()
	if !strings.Contains(out, "Creating private repo: disentangle-network/fleet-deploy") {
		t.Errorf("expected org-based repo creation message, got:\n%s", out)
	}
}

func TestFleetInitFallsBackToGhUser(t *testing.T) {
	tmp := t.TempDir()
	fleetDir := filepath.Join(tmp, "fleet-deploy")

	mock := exec.NewMockExecutor()
	mock.ExpectRun("git clone https://github.com/disentangle-network/fleet.git "+fleetDir, "", nil)
	mock.ExpectRun("git remote remove origin", "", nil)
	mock.ExpectRun("git remote add template https://github.com/disentangle-network/fleet.git", "", nil)
	mock.ExpectRun("gh api user --jq .login", "personaluser\n", nil)
	mock.ExpectRun("gh repo create personaluser/fleet-deploy --private --source "+fleetDir+" --push", "", nil)

	// Empty GitHubOrg -- should fall back to gh api user
	cfg := &config.Config{GitHubOrg: ""}
	cfgPath := filepath.Join(tmp, "config.yaml")

	var buf bytes.Buffer
	err := FleetInit(FleetInitParams{
		Exec:    mock,
		Paths:   paths.NewWithHome(tmp, nil),
		Stdout:  &buf,
		Dir:     fleetDir,
		Config:  cfg,
		CfgFile: cfgPath,
	})
	if err != nil {
		t.Fatalf("FleetInit returned error: %v", err)
	}

	mock.AssertCalled(t, "gh api user --jq .login")
	mock.AssertCalled(t, "gh repo create personaluser/fleet-deploy --private --source "+fleetDir+" --push")
}

// ---------------------------------------------------------------------------
// BUG 7: config should not be polluted by test/temp paths
// ---------------------------------------------------------------------------

func TestFleetInitConfigSavesCorrectPath(t *testing.T) {
	tmp := t.TempDir()
	fleetDir := filepath.Join(tmp, "fleet-deploy")

	// Create a config file for Save to target
	cfgDir := filepath.Join(tmp, ".config", "launch")
	if err := os.MkdirAll(cfgDir, 0750); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(cfgDir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("cluster_name: test\n"), 0600); err != nil {
		t.Fatal(err)
	}

	mock := exec.NewMockExecutor()
	mock.ExpectRun("git clone https://github.com/disentangle-network/fleet.git "+fleetDir, "", nil)
	mock.ExpectRun("git remote remove origin", "", nil)
	mock.ExpectRun("git remote add template https://github.com/disentangle-network/fleet.git", "", nil)
	mock.ExpectRun("gh api user --jq .login", "testuser\n", nil)
	mock.ExpectRun("gh repo create testuser/fleet-deploy --private --source "+fleetDir+" --push", "", nil)

	var buf bytes.Buffer
	err := FleetInit(FleetInitParams{
		Exec:    mock,
		Paths:   paths.NewWithHome(tmp, nil),
		Stdout:  &buf,
		Dir:     fleetDir,
		CfgFile: cfgPath,
	})
	if err != nil {
		t.Fatalf("FleetInit returned error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Saved fleet_dir to config") {
		t.Errorf("expected config save message, got:\n%s", out)
	}

	// The fleet_dir in the saved config should be the actual fleet dir, not a temp path
	// from a different test run
	if !strings.Contains(out, fleetDir) {
		t.Errorf("saved fleet_dir should be %s, got:\n%s", fleetDir, out)
	}
}
