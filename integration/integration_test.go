//go:build integration

package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/disentangle-network/launch/internal/fleet"
)

// skipIfMissing skips the test if the given command is not in PATH.
func skipIfMissing(t *testing.T, cmd string) {
	t.Helper()
	if _, err := exec.LookPath(cmd); err != nil {
		t.Skipf("%s not found in PATH, skipping", cmd)
	}
}

// projectRoot returns the absolute path to the project root.
func projectRoot(t *testing.T) string {
	t.Helper()
	// integration/ is one level below the project root
	root, err := filepath.Abs(filepath.Join("..", "."))
	if err != nil {
		t.Fatalf("failed to resolve project root: %v", err)
	}
	return root
}

// launchBinary returns the path to the launch binary, building it if needed.
func launchBinary(t *testing.T) string {
	t.Helper()
	if bin := os.Getenv("LAUNCH_TEST_BINARY"); bin != "" {
		return bin
	}
	// Build in temp dir
	dir := t.TempDir()
	binPath := filepath.Join(dir, "launch")
	root := projectRoot(t)
	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}
	return binPath
}

// kindContext returns the Kind context name if available.
func kindContext(t *testing.T) string {
	t.Helper()
	ctx := os.Getenv("LAUNCH_KIND_CONTEXT")
	if ctx == "" {
		t.Skip("LAUNCH_KIND_CONTEXT not set, skipping Kind-based test")
	}
	// Verify the context is reachable
	cmd := exec.Command("kubectl", "cluster-info", "--context", ctx)
	if err := cmd.Run(); err != nil {
		t.Skipf("Kind context %s not reachable: %v", ctx, err)
	}
	return ctx
}

func TestKubectlConfigMerge(t *testing.T) {
	skipIfMissing(t, "kubectl")

	dir := t.TempDir()

	kubeconfig1 := `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://localhost:6443
  name: test-cluster-1
contexts:
- context:
    cluster: test-cluster-1
    user: test-user-1
  name: test-context-1
current-context: test-context-1
users:
- name: test-user-1
  user:
    token: fake-token-1
`

	kubeconfig2 := `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://localhost:7443
  name: test-cluster-2
contexts:
- context:
    cluster: test-cluster-2
    user: test-user-2
  name: test-context-2
current-context: test-context-2
users:
- name: test-user-2
  user:
    token: fake-token-2
`

	path1 := filepath.Join(dir, "kubeconfig1.yaml")
	path2 := filepath.Join(dir, "kubeconfig2.yaml")

	if err := os.WriteFile(path1, []byte(kubeconfig1), 0o600); err != nil {
		t.Fatalf("failed to write kubeconfig1: %v", err)
	}
	if err := os.WriteFile(path2, []byte(kubeconfig2), 0o600); err != nil {
		t.Fatalf("failed to write kubeconfig2: %v", err)
	}

	cmd := exec.Command("kubectl", "config", "view", "--flatten")
	cmd.Env = append(os.Environ(), "KUBECONFIG="+path1+":"+path2)

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("kubectl config view --flatten failed: %v\n%s", err, out)
	}

	merged := string(out)

	if !strings.Contains(merged, "test-cluster-1") {
		t.Error("merged kubeconfig missing test-cluster-1")
	}
	if !strings.Contains(merged, "test-cluster-2") {
		t.Error("merged kubeconfig missing test-cluster-2")
	}
	if !strings.Contains(merged, "test-context-1") {
		t.Error("merged kubeconfig missing test-context-1")
	}
	if !strings.Contains(merged, "test-context-2") {
		t.Error("merged kubeconfig missing test-context-2")
	}
}

func TestGofmtClean(t *testing.T) {
	skipIfMissing(t, "gofmt")

	root := projectRoot(t)

	cmd := exec.Command("gofmt", "-l", ".")
	cmd.Dir = root

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gofmt -l failed: %v\n%s", err, out)
	}

	if trimmed := strings.TrimSpace(string(out)); trimmed != "" {
		t.Errorf("gofmt found unformatted files:\n%s", trimmed)
	}
}

func TestGoVetClean(t *testing.T) {
	skipIfMissing(t, "go")

	root := projectRoot(t)

	cmd := exec.Command("go", "vet", "./...")
	cmd.Dir = root

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go vet ./... failed: %v\n%s", err, out)
	}
}

func TestGoBuildSucceeds(t *testing.T) {
	skipIfMissing(t, "go")

	root := projectRoot(t)

	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = root

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build ./... failed: %v\n%s", err, out)
	}
}

func TestBinaryVersionFlag(t *testing.T) {
	skipIfMissing(t, "go")

	root := projectRoot(t)
	dir := t.TempDir()
	binPath := filepath.Join(dir, "launch")

	build := exec.Command("go", "build", "-o", binPath, ".")
	build.Dir = root

	out, err := build.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}

	run := exec.Command(binPath, "--version")
	versionOut, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("binary --version failed: %v\n%s", err, versionOut)
	}

	version := string(versionOut)
	if !strings.Contains(version, "dev") && !strings.Contains(version, "v") && !strings.Contains(version, ".") {
		t.Errorf("unexpected version output: %s", version)
	}
}

func TestBYOKFlow(t *testing.T) {
	bin := launchBinary(t)
	fleetDir := t.TempDir()

	// Step 1: Scaffold fleet repo (use Go API directly since fleet init clones from GitHub)
	t.Run("scaffold_fleet", func(t *testing.T) {
		if err := fleet.InitFleetRepo(fleetDir, "integration-test"); err != nil {
			t.Fatalf("InitFleetRepo failed: %v", err)
		}
		// Verify apps/base exists (fleet repo indicator)
		if _, err := os.Stat(filepath.Join(fleetDir, "apps", "base")); err != nil {
			t.Fatalf("apps/base not found: %v", err)
		}
	})

	// Step 2: Add cluster
	t.Run("cluster_add", func(t *testing.T) {
		cmd := exec.Command(bin, "cluster", "add", "byok-test",
			"--fleet-dir", fleetDir,
			"--arch", "amd64",
			"--infra", "local",
			"--nodes", "1",
			"--resources", "small",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("cluster add failed: %v\n%s", err, out)
		}
		// Verify files created
		clusterDir := filepath.Join(fleetDir, "clusters", "byok-test")
		for _, f := range []string{"cluster-settings.yaml", "infrastructure.yaml", "apps.yaml"} {
			if _, err := os.Stat(filepath.Join(clusterDir, f)); err != nil {
				t.Errorf("expected %s to exist: %v", f, err)
			}
		}
	})

	// Step 3: Init secrets (age provider -- no external tools needed beyond age-keygen)
	t.Run("secrets_init", func(t *testing.T) {
		skipIfMissing(t, "age-keygen")
		cmd := exec.Command(bin, "secrets", "init",
			"--cluster", "byok-test",
			"--provider", "age",
			"--fleet-dir", fleetDir,
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("secrets init failed: %v\n%s", err, out)
		}

		secretsDir := filepath.Join(fleetDir, "secrets", "byok-test")
		// Verify age.key was created
		if _, err := os.Stat(filepath.Join(secretsDir, "age.key")); err != nil {
			t.Errorf("age.key not created: %v", err)
		}
		// Verify genesis-config.yaml was written
		if _, err := os.Stat(filepath.Join(secretsDir, "genesis-config.yaml")); err != nil {
			t.Errorf("genesis-config.yaml not created: %v", err)
		}
		// Verify .sops.yaml was auto-created
		sopsPath := filepath.Join(fleetDir, ".sops.yaml")
		data, err := os.ReadFile(sopsPath)
		if err != nil {
			t.Fatalf(".sops.yaml not created: %v", err)
		}
		if !strings.Contains(string(data), "secrets/byok-test/.*") {
			t.Errorf(".sops.yaml should contain creation rule for byok-test, got: %s", string(data))
		}

		t.Logf("secrets init output:\n%s", out)
	})

	// Step 4: Import Kind cluster kubeconfig (needs Kind)
	t.Run("cluster_import", func(t *testing.T) {
		ctx := kindContext(t) // skips if no Kind
		_ = ctx

		// Export Kind kubeconfig to temp file
		kindName := strings.TrimPrefix(os.Getenv("LAUNCH_KIND_CONTEXT"), "kind-")
		kubeconfigCmd := exec.Command("kind", "get", "kubeconfig", "--name", kindName)
		kcOut, err := kubeconfigCmd.Output()
		if err != nil {
			t.Fatalf("kind get kubeconfig failed: %v", err)
		}
		kubeconfigPath := filepath.Join(t.TempDir(), "kind-kubeconfig.yaml")
		if err := os.WriteFile(kubeconfigPath, kcOut, 0600); err != nil {
			t.Fatalf("failed to write kubeconfig: %v", err)
		}

		// Set HOME to a temp dir so launch writes to a predictable location
		homeDir := t.TempDir()
		cmd := exec.Command(bin, "cluster", "import", "disentangle", kubeconfigPath)
		cmd.Env = append(os.Environ(), "HOME="+homeDir)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("cluster import failed: %v\n%s", err, out)
		}

		// Verify kubeconfig was written
		targetPath := filepath.Join(homeDir, ".kube", "disentangle", "config")
		if _, err := os.Stat(targetPath); err != nil {
			t.Errorf("expected kubeconfig at %s: %v", targetPath, err)
		}

		t.Logf("cluster import output:\n%s", out)
	})

	// Step 5: Cluster list should show byok-test
	t.Run("cluster_list", func(t *testing.T) {
		cmd := exec.Command(bin, "cluster", "list", "--fleet-dir", fleetDir)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("cluster list failed: %v\n%s", err, out)
		}
		if !strings.Contains(string(out), "byok-test") {
			t.Errorf("cluster list should show byok-test, got: %s", out)
		}
	})
}
