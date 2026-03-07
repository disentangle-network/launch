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

func TestStatusNoClusters(t *testing.T) {
	tmp := t.TempDir()
	fleetDir := filepath.Join(tmp, "fleet")
	if err := os.MkdirAll(filepath.Join(fleetDir, "clusters"), 0755); err != nil {
		t.Fatal(err)
	}

	mock := exec.NewMockExecutor()
	p := paths.NewWithHome(tmp, nil)
	var buf bytes.Buffer

	err := Status(StatusParams{
		Exec:     mock,
		Paths:    p,
		Stdout:   &buf,
		FleetDir: fleetDir,
	})
	if err != nil {
		t.Fatalf("Status() returned error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "No clusters configured") {
		t.Errorf("expected 'No clusters configured' in output, got: %s", out)
	}
}

func TestStatusWithClusters(t *testing.T) {
	tmp := t.TempDir()
	fleetDir := filepath.Join(tmp, "fleet")
	if err := os.MkdirAll(filepath.Join(fleetDir, "clusters", "dev"), 0755); err != nil {
		t.Fatal(err)
	}

	mock := exec.NewMockExecutor()
	// resolveContext: exact match succeeds
	mock.ExpectRun("kubectl config get-contexts dev --no-headers", "dev", nil)
	// kubectl get ns succeeds (FluxCD installed)
	mock.ExpectRun("kubectl --context dev get ns flux-system", "", nil)
	// flux get all returns output
	mock.ExpectRun("flux --context dev get all -A --no-header", "kustomization/flux-system  True  Applied", nil)
	// kubectl get pods returns pod list
	mock.ExpectRun("kubectl --context dev get pods -n disentangle -o wide --no-headers",
		"node-0  1/1  Running  0  2h  10.0.0.1  worker-1", nil)

	p := paths.NewWithHome(tmp, nil)
	var buf bytes.Buffer

	err := Status(StatusParams{
		Exec:     mock,
		Paths:    p,
		Stdout:   &buf,
		FleetDir: fleetDir,
	})
	if err != nil {
		t.Fatalf("Status() returned error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "=== Cluster: dev ===") {
		t.Errorf("expected cluster name in output, got: %s", out)
	}
	if !strings.Contains(out, "FluxCD:") {
		t.Errorf("expected 'FluxCD:' in output, got: %s", out)
	}
	if !strings.Contains(out, "kustomization/flux-system") {
		t.Errorf("expected flux output in output, got: %s", out)
	}
	if !strings.Contains(out, "node-0") {
		t.Errorf("expected pod info in output, got: %s", out)
	}
}

func TestStatusUnreachableCluster(t *testing.T) {
	tmp := t.TempDir()
	fleetDir := filepath.Join(tmp, "fleet")
	if err := os.MkdirAll(filepath.Join(fleetDir, "clusters", "dev"), 0755); err != nil {
		t.Fatal(err)
	}

	mock := exec.NewMockExecutor()
	// resolveContext: exact match succeeds
	mock.ExpectRun("kubectl config get-contexts dev --no-headers", "dev", nil)
	// kubectl get ns fails (cluster unreachable)
	mock.ExpectRun("kubectl --context dev get ns flux-system", "", fmt.Errorf("connection refused"))

	p := paths.NewWithHome(tmp, nil)
	var buf bytes.Buffer

	err := Status(StatusParams{
		Exec:     mock,
		Paths:    p,
		Stdout:   &buf,
		FleetDir: fleetDir,
	})
	if err != nil {
		t.Fatalf("Status() returned error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "FluxCD not installed (or cluster unreachable)") {
		t.Errorf("expected unreachable message in output, got: %s", out)
	}
}

func TestStatusKindContextFallback(t *testing.T) {
	tmp := t.TempDir()
	fleetDir := filepath.Join(tmp, "fleet")
	if err := os.MkdirAll(filepath.Join(fleetDir, "clusters", "disentangle-local"), 0755); err != nil {
		t.Fatal(err)
	}

	mock := exec.NewMockExecutor()
	// resolveContext: exact name fails, kind- prefix succeeds
	mock.ExpectRun("kubectl config get-contexts disentangle-local --no-headers", "",
		fmt.Errorf("no context named disentangle-local"))
	mock.ExpectRun("kubectl config get-contexts kind-disentangle-local --no-headers",
		"kind-disentangle-local", nil)
	// kubectl get ns with resolved kind- context
	mock.ExpectRun("kubectl --context kind-disentangle-local get ns flux-system", "", nil)
	mock.ExpectRun("flux --context kind-disentangle-local get all -A --no-header",
		"kustomization/flux-system  True  Applied", nil)
	mock.ExpectRun("kubectl --context kind-disentangle-local get pods -n disentangle -o wide --no-headers",
		"node-0  1/1  Running  0  1h  10.0.0.1  worker-1", nil)

	p := paths.NewWithHome(tmp, nil)
	var buf bytes.Buffer

	err := Status(StatusParams{
		Exec:     mock,
		Paths:    p,
		Stdout:   &buf,
		FleetDir: fleetDir,
	})
	if err != nil {
		t.Fatalf("Status() returned error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "=== Cluster: disentangle-local ===") {
		t.Errorf("expected cluster name in output, got: %s", out)
	}
	if !strings.Contains(out, "kustomization/flux-system") {
		t.Errorf("expected flux output using kind- context, got: %s", out)
	}

	// Verify the kind- prefixed context was used for kubectl commands
	mock.AssertCalled(t, "kubectl --context kind-disentangle-local get ns flux-system")
}

func TestStatusNoSubstringFallback(t *testing.T) {
	// When exact and prefix matches fail, resolveContext returns the original
	// name -- it does NOT do substring matching (which would cause false positives).
	tmp := t.TempDir()
	fleetDir := filepath.Join(tmp, "fleet")
	if err := os.MkdirAll(filepath.Join(fleetDir, "clusters", "prod"), 0755); err != nil {
		t.Fatal(err)
	}

	mock := exec.NewMockExecutor()
	// resolveContext: exact name fails, kind- prefix fails
	mock.ExpectRun("kubectl config get-contexts prod --no-headers", "",
		fmt.Errorf("no context named prod"))
	mock.ExpectRun("kubectl config get-contexts kind-prod --no-headers", "",
		fmt.Errorf("no context named kind-prod"))
	// Falls through to original name "prod" (no substring search)
	mock.ExpectRun("kubectl --context prod get ns flux-system", "",
		fmt.Errorf("connection refused"))

	p := paths.NewWithHome(tmp, nil)
	var buf bytes.Buffer

	err := Status(StatusParams{
		Exec:     mock,
		Paths:    p,
		Stdout:   &buf,
		FleetDir: fleetDir,
	})
	if err != nil {
		t.Fatalf("Status() returned error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "=== Cluster: prod ===") {
		t.Errorf("expected cluster name in output, got: %s", out)
	}
	// Should use original name, NOT a substring match
	mock.AssertCalled(t, "kubectl --context prod get ns flux-system")
}

func TestStatusFleetDirResolution(t *testing.T) {
	tmp := t.TempDir()
	// Create a fleet dir under the resolver's default path structure
	resolvedFleet := filepath.Join(tmp, "DISENTANGLE-NETWORK", "fleet-deploy")
	if err := os.MkdirAll(filepath.Join(resolvedFleet, "clusters"), 0755); err != nil {
		t.Fatal(err)
	}

	mock := exec.NewMockExecutor()
	p := paths.NewWithHome(tmp, nil)
	var buf bytes.Buffer

	// Pass empty FleetDir so the resolver's default is used
	err := Status(StatusParams{
		Exec:     mock,
		Paths:    p,
		Stdout:   &buf,
		FleetDir: "",
	})
	if err != nil {
		t.Fatalf("Status() returned error: %v", err)
	}

	out := buf.String()
	// With no clusters, it should still succeed (resolved to default fleet dir)
	if !strings.Contains(out, "No clusters configured") {
		t.Errorf("expected 'No clusters configured' from resolved fleet dir, got: %s", out)
	}
}
