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
