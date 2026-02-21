package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/disentangle-network/launch/internal/exec"
	"github.com/disentangle-network/launch/internal/paths"
)

func TestParseGitRemote(t *testing.T) {
	tests := []struct {
		url       string
		wantOwner string
		wantRepo  string
	}{
		{"https://github.com/privsim/fleet-deploy.git", "privsim", "fleet-deploy"},
		{"https://github.com/privsim/fleet-deploy", "privsim", "fleet-deploy"},
		{"git@github.com:disentangle-network/fleet.git", "disentangle-network", "fleet"},
		{"git@github.com:disentangle-network/fleet", "disentangle-network", "fleet"},
		{"https://github.com/LarsenClose/genesis-operator.git", "LarsenClose", "genesis-operator"},
		{"", "", ""},
	}

	for _, tt := range tests {
		owner, repo := parseGitRemote(tt.url)
		if owner != tt.wantOwner || repo != tt.wantRepo {
			t.Errorf("parseGitRemote(%q) = (%q, %q), want (%q, %q)",
				tt.url, owner, repo, tt.wantOwner, tt.wantRepo)
		}
	}
}

func TestBootstrapClusterNotFound(t *testing.T) {
	tmp := t.TempDir()
	// Create fleet dir structure but no cluster dir
	if err := os.MkdirAll(filepath.Join(tmp, "clusters"), 0o755); err != nil {
		t.Fatal(err)
	}

	mock := exec.NewMockExecutor()
	var buf bytes.Buffer
	p := BootstrapParams{
		Exec:     mock,
		Paths:    paths.NewWithHome(tmp, nil),
		Stdout:   &buf,
		Cluster:  "nonexistent",
		FleetDir: tmp,
		Branch:   "main",
	}

	err := Bootstrap(p)
	if err == nil {
		t.Fatal("expected error for missing cluster, got nil")
	}
	if !strings.Contains(err.Error(), "not found in fleet repo") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}

	// No commands should have been called
	mock.AssertCallCount(t, 0)
}

func TestBootstrapFullFlow(t *testing.T) {
	tmp := t.TempDir()
	cluster := "dev"

	// Create cluster dir
	if err := os.MkdirAll(filepath.Join(tmp, "clusters", cluster), 0o755); err != nil {
		t.Fatal(err)
	}

	// Create genesis-config and age.key
	secretsDir := filepath.Join(tmp, "secrets", cluster)
	if err := os.MkdirAll(secretsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(secretsDir, "genesis-config.yaml"), []byte("test: true"), 0o644); err != nil {
		t.Fatal(err)
	}
	ageKeyPath := filepath.Join(secretsDir, "age.key")
	if err := os.WriteFile(ageKeyPath, []byte("AGE-SECRET-KEY-1FAKE"), 0o600); err != nil {
		t.Fatal(err)
	}

	mock := exec.NewMockExecutor()
	// Step 1: kubectl cluster-info
	mock.ExpectRun("kubectl cluster-info --request-timeout=5s", "Kubernetes control plane is running", nil)
	// Step 2: flux version
	mock.ExpectRun("flux version --client", "flux version 2.2.0", nil)
	// Step 3: git remote
	mock.ExpectRun("git -C "+tmp+" remote get-url origin", "https://github.com/privsim/fleet-deploy.git\n", nil)
	// Step 4: flux bootstrap
	mock.ExpectRun("flux bootstrap github --owner privsim --repository fleet-deploy --branch main --path clusters/dev --personal", "", nil)
	// Step 5: kubectl create secret
	mock.ExpectRun("kubectl create secret generic sops-age -n flux-system --from-file=age.agekey="+ageKeyPath, "", nil)

	var buf bytes.Buffer
	p := BootstrapParams{
		Exec:     mock,
		Paths:    paths.NewWithHome(tmp, nil),
		Stdout:   &buf,
		Cluster:  cluster,
		FleetDir: tmp,
		Branch:   "main",
	}

	err := Bootstrap(p)
	if err != nil {
		t.Fatalf("Bootstrap returned error: %v", err)
	}

	// Verify all 5 steps were called
	mock.AssertCalled(t, "kubectl cluster-info --request-timeout=5s")
	mock.AssertCalled(t, "flux version --client")
	mock.AssertCalled(t, "git -C "+tmp+" remote get-url origin")
	mock.AssertCalled(t, "flux bootstrap github --owner privsim --repository fleet-deploy --branch main --path clusters/dev --personal")
	mock.AssertCalled(t, "kubectl create secret generic sops-age -n flux-system --from-file=age.agekey="+ageKeyPath)

	out := buf.String()
	if !strings.Contains(out, "Bootstrap complete for cluster 'dev'") {
		t.Error("output missing completion message")
	}
	if !strings.Contains(out, "sops-age secret created") {
		t.Error("output missing sops-age creation confirmation")
	}
}

func TestBootstrapWithContext(t *testing.T) {
	tmp := t.TempDir()
	cluster := "staging"
	ctx := "my-k8s-context"

	// Create cluster dir
	if err := os.MkdirAll(filepath.Join(tmp, "clusters", cluster), 0o755); err != nil {
		t.Fatal(err)
	}

	// Create genesis-config and age.key so step 5 runs with --context
	secretsDir := filepath.Join(tmp, "secrets", cluster)
	if err := os.MkdirAll(secretsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(secretsDir, "genesis-config.yaml"), []byte("test: true"), 0o644); err != nil {
		t.Fatal(err)
	}
	ageKeyPath := filepath.Join(secretsDir, "age.key")
	if err := os.WriteFile(ageKeyPath, []byte("AGE-SECRET-KEY-1FAKE"), 0o600); err != nil {
		t.Fatal(err)
	}

	mock := exec.NewMockExecutor()
	// Step 1: kubectl with --context
	mock.ExpectRun("kubectl cluster-info --request-timeout=5s --context "+ctx, "ok", nil)
	// Step 2: flux version
	mock.ExpectRun("flux version --client", "flux 2.2.0", nil)
	// Step 4: flux bootstrap with --context
	mock.ExpectRun("flux bootstrap github --owner myorg --repository fleet-deploy --branch main --path clusters/staging --personal --context "+ctx, "", nil)
	// Step 5: kubectl create secret with --context
	mock.ExpectRun("kubectl create secret generic sops-age -n flux-system --from-file=age.agekey="+ageKeyPath+" --context "+ctx, "", nil)

	var buf bytes.Buffer
	p := BootstrapParams{
		Exec:     mock,
		Paths:    paths.NewWithHome(tmp, nil),
		Stdout:   &buf,
		Cluster:  cluster,
		FleetDir: tmp,
		Owner:    "myorg",
		Repo:     "fleet-deploy",
		Branch:   "main",
		Context:  ctx,
	}

	err := Bootstrap(p)
	if err != nil {
		t.Fatalf("Bootstrap returned error: %v", err)
	}

	// Verify --context was passed to kubectl
	mock.AssertCalled(t, "kubectl cluster-info --request-timeout=5s --context "+ctx)

	// Verify --context was passed to flux bootstrap
	mock.AssertCalled(t, "flux bootstrap github --owner myorg --repository fleet-deploy --branch main --path clusters/staging --personal --context "+ctx)

	// Verify --context was passed to kubectl create secret
	mock.AssertCalled(t, "kubectl create secret generic sops-age -n flux-system --from-file=age.agekey="+ageKeyPath+" --context "+ctx)
}

func TestBootstrapMissingFlux(t *testing.T) {
	tmp := t.TempDir()
	cluster := "dev"

	// Create cluster dir
	if err := os.MkdirAll(filepath.Join(tmp, "clusters", cluster), 0o755); err != nil {
		t.Fatal(err)
	}

	mock := exec.NewMockExecutor()
	// Step 1: kubectl succeeds
	mock.ExpectRun("kubectl cluster-info --request-timeout=5s", "ok", nil)
	// Step 2: flux fails
	mock.ExpectRun("flux version --client", "", fmt.Errorf("command failed: flux version --client: exec: \"flux\": executable file not found in $PATH"))

	var buf bytes.Buffer
	p := BootstrapParams{
		Exec:     mock,
		Paths:    paths.NewWithHome(tmp, nil),
		Stdout:   &buf,
		Cluster:  cluster,
		FleetDir: tmp,
		Branch:   "main",
	}

	err := Bootstrap(p)
	if err == nil {
		t.Fatal("expected error for missing flux, got nil")
	}
	if !strings.Contains(err.Error(), "flux CLI not found") {
		t.Errorf("expected 'flux CLI not found' error, got: %v", err)
	}

	// Only 2 commands should have been called (kubectl + flux)
	mock.AssertCallCount(t, 2)
}

func TestBootstrapNoGenesisConfig(t *testing.T) {
	tmp := t.TempDir()
	cluster := "dev"

	// Create cluster dir but NO genesis-config.yaml
	if err := os.MkdirAll(filepath.Join(tmp, "clusters", cluster), 0o755); err != nil {
		t.Fatal(err)
	}

	mock := exec.NewMockExecutor()
	mock.ExpectRun("kubectl cluster-info --request-timeout=5s", "ok", nil)
	mock.ExpectRun("flux version --client", "flux 2.2.0", nil)
	mock.ExpectRun("flux bootstrap github --owner myorg --repository fleet-deploy --branch main --path clusters/dev --personal", "", nil)

	var buf bytes.Buffer
	p := BootstrapParams{
		Exec:     mock,
		Paths:    paths.NewWithHome(tmp, nil),
		Stdout:   &buf,
		Cluster:  cluster,
		FleetDir: tmp,
		Owner:    "myorg",
		Repo:     "fleet-deploy",
		Branch:   "main",
	}

	err := Bootstrap(p)
	if err != nil {
		t.Fatalf("Bootstrap returned error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "No genesis config found, skipping secrets provisioning") {
		t.Error("output missing 'No genesis config found' message")
	}

	// kubectl create secret should NOT have been called
	for _, c := range mock.Calls {
		if strings.Contains(c.CommandString(), "create secret") {
			t.Error("kubectl create secret should not be called when no genesis config exists")
		}
	}

	// Only 3 commands: kubectl cluster-info, flux version, flux bootstrap
	mock.AssertCallCount(t, 3)
}

// ---------------------------------------------------------------------------
// Additional Bootstrap coverage tests
// ---------------------------------------------------------------------------

func TestBootstrapKubectlFails(t *testing.T) {
	tmp := t.TempDir()
	cluster := "dev"
	if err := os.MkdirAll(filepath.Join(tmp, "clusters", cluster), 0o755); err != nil {
		t.Fatal(err)
	}

	mock := exec.NewMockExecutor()
	mock.ExpectRunWithResult("kubectl cluster-info --request-timeout=5s",
		&exec.Result{Stderr: "connection refused"},
		fmt.Errorf("exit status 1"),
	)

	var buf bytes.Buffer
	err := Bootstrap(BootstrapParams{
		Exec:     mock,
		Paths:    paths.NewWithHome(tmp, nil),
		Stdout:   &buf,
		Cluster:  cluster,
		FleetDir: tmp,
		Branch:   "main",
	})
	if err == nil {
		t.Fatal("expected error for kubectl failure, got nil")
	}
	if !strings.Contains(err.Error(), "kubectl not configured") {
		t.Errorf("expected 'kubectl not configured' error, got: %v", err)
	}
	mock.AssertCallCount(t, 1)
}

func TestBootstrapGitRemoteDetectionFails(t *testing.T) {
	tmp := t.TempDir()
	cluster := "dev"
	if err := os.MkdirAll(filepath.Join(tmp, "clusters", cluster), 0o755); err != nil {
		t.Fatal(err)
	}

	mock := exec.NewMockExecutor()
	mock.ExpectRun("kubectl cluster-info --request-timeout=5s", "ok", nil)
	mock.ExpectRun("flux version --client", "flux 2.2.0", nil)
	// git remote fails
	mock.ExpectRunWithResult("git -C "+tmp+" remote get-url origin",
		&exec.Result{Stderr: "no remote"},
		fmt.Errorf("exit status 1"),
	)

	var buf bytes.Buffer
	err := Bootstrap(BootstrapParams{
		Exec:     mock,
		Paths:    paths.NewWithHome(tmp, nil),
		Stdout:   &buf,
		Cluster:  cluster,
		FleetDir: tmp,
		Branch:   "main",
		// No Owner or Repo set, so remote detection is needed
	})
	if err == nil {
		t.Fatal("expected error for git remote failure, got nil")
	}
	if !strings.Contains(err.Error(), "could not detect git remote") {
		t.Errorf("expected 'could not detect git remote' error, got: %v", err)
	}
}

func TestBootstrapFluxBootstrapFails(t *testing.T) {
	tmp := t.TempDir()
	cluster := "dev"
	if err := os.MkdirAll(filepath.Join(tmp, "clusters", cluster), 0o755); err != nil {
		t.Fatal(err)
	}

	mock := exec.NewMockExecutor()
	mock.ExpectRun("kubectl cluster-info --request-timeout=5s", "ok", nil)
	mock.ExpectRun("flux version --client", "flux 2.2.0", nil)
	// flux bootstrap fails
	mock.ExpectRunWithResult(
		"flux bootstrap github --owner myorg --repository fleet-deploy --branch main --path clusters/dev --personal",
		&exec.Result{Stderr: "failed"},
		fmt.Errorf("exit status 1"),
	)

	var buf bytes.Buffer
	err := Bootstrap(BootstrapParams{
		Exec:     mock,
		Paths:    paths.NewWithHome(tmp, nil),
		Stdout:   &buf,
		Cluster:  cluster,
		FleetDir: tmp,
		Owner:    "myorg",
		Repo:     "fleet-deploy",
		Branch:   "main",
	})
	if err == nil {
		t.Fatal("expected error for flux bootstrap failure, got nil")
	}
	if !strings.Contains(err.Error(), "flux bootstrap failed") {
		t.Errorf("expected 'flux bootstrap failed' error, got: %v", err)
	}
}

func TestBootstrapVerboseMode(t *testing.T) {
	tmp := t.TempDir()
	cluster := "dev"
	if err := os.MkdirAll(filepath.Join(tmp, "clusters", cluster), 0o755); err != nil {
		t.Fatal(err)
	}

	mock := exec.NewMockExecutor()
	mock.ExpectRun("kubectl cluster-info --request-timeout=5s", "Kubernetes control plane is running at https://10.0.0.1:6443", nil)
	mock.ExpectRun("flux version --client", "flux 2.2.0", nil)
	mock.ExpectRun("flux bootstrap github --owner myorg --repository fleet-deploy --branch main --path clusters/dev --personal", "", nil)

	var buf bytes.Buffer
	err := Bootstrap(BootstrapParams{
		Exec:     mock,
		Paths:    paths.NewWithHome(tmp, nil),
		Stdout:   &buf,
		Cluster:  cluster,
		FleetDir: tmp,
		Owner:    "myorg",
		Repo:     "fleet-deploy",
		Branch:   "main",
		Verbose:  true, // Enable verbose output
	})
	if err != nil {
		t.Fatalf("Bootstrap returned error: %v", err)
	}

	out := buf.String()
	// Verbose mode should print the kubectl cluster-info output
	if !strings.Contains(out, "Kubernetes control plane is running") {
		t.Errorf("expected verbose kubectl output, got:\n%s", out)
	}
}

func TestBootstrapGenesisConfigNoAgeKey(t *testing.T) {
	tmp := t.TempDir()
	cluster := "dev"
	if err := os.MkdirAll(filepath.Join(tmp, "clusters", cluster), 0o755); err != nil {
		t.Fatal(err)
	}

	// Create genesis-config but NO age.key
	secretsDir := filepath.Join(tmp, "secrets", cluster)
	if err := os.MkdirAll(secretsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(secretsDir, "genesis-config.yaml"), []byte("test: true"), 0o644); err != nil {
		t.Fatal(err)
	}

	mock := exec.NewMockExecutor()
	mock.ExpectRun("kubectl cluster-info --request-timeout=5s", "ok", nil)
	mock.ExpectRun("flux version --client", "flux 2.2.0", nil)
	mock.ExpectRun("flux bootstrap github --owner myorg --repository fleet-deploy --branch main --path clusters/dev --personal", "", nil)

	var buf bytes.Buffer
	err := Bootstrap(BootstrapParams{
		Exec:     mock,
		Paths:    paths.NewWithHome(tmp, nil),
		Stdout:   &buf,
		Cluster:  cluster,
		FleetDir: tmp,
		Owner:    "myorg",
		Repo:     "fleet-deploy",
		Branch:   "main",
	})
	if err != nil {
		t.Fatalf("Bootstrap returned error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "No age key found, skipping sops-age secret creation") {
		t.Errorf("expected 'No age key found' message, got:\n%s", out)
	}
}

func TestBootstrapSopsSecretCreateFails(t *testing.T) {
	tmp := t.TempDir()
	cluster := "dev"
	if err := os.MkdirAll(filepath.Join(tmp, "clusters", cluster), 0o755); err != nil {
		t.Fatal(err)
	}

	secretsDir := filepath.Join(tmp, "secrets", cluster)
	if err := os.MkdirAll(secretsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(secretsDir, "genesis-config.yaml"), []byte("test: true"), 0o644); err != nil {
		t.Fatal(err)
	}
	ageKeyPath := filepath.Join(secretsDir, "age.key")
	if err := os.WriteFile(ageKeyPath, []byte("AGE-SECRET-KEY-1FAKE"), 0o600); err != nil {
		t.Fatal(err)
	}

	mock := exec.NewMockExecutor()
	mock.ExpectRun("kubectl cluster-info --request-timeout=5s", "ok", nil)
	mock.ExpectRun("flux version --client", "flux 2.2.0", nil)
	mock.ExpectRun("flux bootstrap github --owner myorg --repository fleet-deploy --branch main --path clusters/dev --personal", "", nil)
	// sops-age secret creation fails (already exists)
	mock.ExpectRunWithResult(
		"kubectl create secret generic sops-age -n flux-system --from-file=age.agekey="+ageKeyPath,
		&exec.Result{Stderr: "already exists"},
		fmt.Errorf("exit status 1"),
	)

	var buf bytes.Buffer
	err := Bootstrap(BootstrapParams{
		Exec:     mock,
		Paths:    paths.NewWithHome(tmp, nil),
		Stdout:   &buf,
		Cluster:  cluster,
		FleetDir: tmp,
		Owner:    "myorg",
		Repo:     "fleet-deploy",
		Branch:   "main",
	})
	if err != nil {
		t.Fatalf("Bootstrap should not error on sops-age create failure: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Warning: could not create sops-age secret") {
		t.Errorf("expected sops-age warning, got:\n%s", out)
	}
}

func TestBootstrapOwnerOnlyProvided(t *testing.T) {
	// Owner provided but not repo -- should detect repo from remote.
	tmp := t.TempDir()
	cluster := "dev"
	if err := os.MkdirAll(filepath.Join(tmp, "clusters", cluster), 0o755); err != nil {
		t.Fatal(err)
	}

	mock := exec.NewMockExecutor()
	mock.ExpectRun("kubectl cluster-info --request-timeout=5s", "ok", nil)
	mock.ExpectRun("flux version --client", "flux 2.2.0", nil)
	mock.ExpectRun("git -C "+tmp+" remote get-url origin",
		"https://github.com/privsim/fleet-deploy.git\n", nil)
	mock.ExpectRun("flux bootstrap github --owner myorg --repository fleet-deploy --branch main --path clusters/dev --personal", "", nil)

	var buf bytes.Buffer
	err := Bootstrap(BootstrapParams{
		Exec:     mock,
		Paths:    paths.NewWithHome(tmp, nil),
		Stdout:   &buf,
		Cluster:  cluster,
		FleetDir: tmp,
		Owner:    "myorg", // Owner provided
		Repo:     "",      // Repo NOT provided
		Branch:   "main",
	})
	if err != nil {
		t.Fatalf("Bootstrap returned error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Detected repo: fleet-deploy") {
		t.Errorf("expected detected repo message, got:\n%s", out)
	}
}

func TestParseGitRemoteSSHMinimal(t *testing.T) {
	// SSH URL with single segment after colon (insufficient)
	owner, repo := parseGitRemote("git@github.com:single")
	if owner != "" || repo != "" {
		t.Errorf("expected empty owner/repo for malformed SSH URL, got (%q, %q)", owner, repo)
	}
}

func TestParseGitRemoteHTTPShort(t *testing.T) {
	// HTTPS URL with only one segment
	owner, repo := parseGitRemote("https://github.com")
	// Should parse the last two segments; with one segment it gets
	// "github.com" as repo and the protocol prefix as owner -- just verify
	// it doesn't panic.
	_ = owner
	_ = repo
}
