//go:build integration

package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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
