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

func TestMeshInitNewCA(t *testing.T) {
	home := t.TempDir()
	mock := exec.NewMockExecutor()
	p := paths.NewWithHome(home, nil)

	// Mock the nebula-cert ca command
	caKey := p.NebulaCAKey()
	caCrt := p.NebulaCACert()
	mock.ExpectRun(
		fmt.Sprintf("nebula-cert ca -curve PQ -name disentangle -out-key %s -out-crt %s", caKey, caCrt),
		"", nil,
	)

	// Use a non-existent config path so no real config is loaded
	cfgPath := filepath.Join(home, "nonexistent-config.yaml")

	var buf bytes.Buffer
	err := MeshInit(MeshInitParams{
		Exec:    mock,
		Paths:   p,
		Stdout:  &buf,
		CfgFile: cfgPath,
	})
	if err != nil {
		t.Fatalf("MeshInit returned error: %v", err)
	}

	// Verify CA dir was created
	caDir := p.NebulaCADir()
	if _, err := os.Stat(caDir); os.IsNotExist(err) {
		t.Errorf("CA directory not created at %s", caDir)
	}

	// Verify the nebula-cert command was called
	mock.AssertCallCount(t, 1)
	mock.AssertCalled(t,
		fmt.Sprintf("nebula-cert ca -curve PQ -name disentangle -out-key %s -out-crt %s", caKey, caCrt),
	)

	// Verify output mentions the key and cert paths
	output := buf.String()
	if !strings.Contains(output, caKey) {
		t.Errorf("output should mention CA key path, got: %s", output)
	}
	if !strings.Contains(output, caCrt) {
		t.Errorf("output should mention CA cert path, got: %s", output)
	}
}

func TestMeshInitAlreadyExists(t *testing.T) {
	home := t.TempDir()
	mock := exec.NewMockExecutor()
	p := paths.NewWithHome(home, nil)

	// Create the CA key file so init detects it already exists
	caDir := p.NebulaCADir()
	if err := os.MkdirAll(caDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.NebulaCAKey(), []byte("existing-key"), 0600); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	err := MeshInit(MeshInitParams{
		Exec:   mock,
		Paths:  p,
		Stdout: &buf,
	})
	if err == nil {
		t.Fatal("expected error when CA key already exists")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error should mention 'already exists', got: %v", err)
	}

	// No commands should have been executed
	mock.AssertCallCount(t, 0)
}

func TestMeshInitCANameFromConfig(t *testing.T) {
	home := t.TempDir()
	mock := exec.NewMockExecutor()
	p := paths.NewWithHome(home, nil)

	// Write a config file with a domain set
	cfgDir := filepath.Join(home, ".config", "launch")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(cfgDir, "config.yaml")
	cfg := &config.Config{Domain: "example.mesh"}
	if err := config.Save(cfg, cfgPath); err != nil {
		t.Fatal(err)
	}

	caKey := p.NebulaCAKey()
	caCrt := p.NebulaCACert()

	// Expect the CA name to be "example.mesh" from the config
	mock.ExpectRun(
		fmt.Sprintf("nebula-cert ca -curve PQ -name example.mesh -out-key %s -out-crt %s", caKey, caCrt),
		"", nil,
	)

	var buf bytes.Buffer
	err := MeshInit(MeshInitParams{
		Exec:    mock,
		Paths:   p,
		Stdout:  &buf,
		CfgFile: cfgPath,
	})
	if err != nil {
		t.Fatalf("MeshInit returned error: %v", err)
	}

	mock.AssertCalled(t,
		fmt.Sprintf("nebula-cert ca -curve PQ -name example.mesh -out-key %s -out-crt %s", caKey, caCrt),
	)
}

func TestMeshAddPerNodeCerts(t *testing.T) {
	home := t.TempDir()
	mock := exec.NewMockExecutor()
	p := paths.NewWithHome(home, nil)

	fleetDir := t.TempDir()
	cluster := "prod"

	// Create CA key and cert so the check passes
	caDir := p.NebulaCADir()
	if err := os.MkdirAll(caDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.NebulaCAKey(), []byte("key"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.NebulaCACert(), []byte("cert"), 0600); err != nil {
		t.Fatal(err)
	}

	// Create cluster-settings.yaml with nodes: "3"
	clusterDir := p.FleetClusterDir(fleetDir, cluster)
	if err := os.MkdirAll(clusterDir, 0755); err != nil {
		t.Fatal(err)
	}
	settingsContent := `apiVersion: v1
kind: ConfigMap
metadata:
  name: cluster-settings
data:
  nodes: "3"
  nebula_prefix: "10.42.0"
`
	if err := os.WriteFile(filepath.Join(clusterDir, "cluster-settings.yaml"), []byte(settingsContent), 0644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	err := MeshAdd(MeshAddParams{
		Exec:     mock,
		Paths:    p,
		Stdout:   &buf,
		Cluster:  cluster,
		FleetDir: fleetDir,
	})
	if err != nil {
		t.Fatalf("MeshAdd returned error: %v", err)
	}

	// Verify 3 sign commands were issued
	mock.AssertCallCount(t, 3)

	secretsDir := p.FleetSecretsDir(fleetDir, cluster)
	caKey := p.NebulaCAKey()
	caCrt := p.NebulaCACert()

	for i := 1; i <= 3; i++ {
		certName := fmt.Sprintf("%s-node%d", cluster, i)
		ip := fmt.Sprintf("10.42.0.%d/16", i)
		expected := fmt.Sprintf("nebula-cert sign -ca-key %s -ca-crt %s -name %s -networks %s -groups disentangle -out-key %s/%s.key -out-crt %s/%s.crt",
			caKey, caCrt, certName, ip, secretsDir, certName, secretsDir, certName)
		mock.AssertCalled(t, expected)
	}

	// Verify output mentions 3 nodes
	output := buf.String()
	if !strings.Contains(output, "3 nodes") {
		t.Errorf("output should mention '3 nodes', got: %s", output)
	}
}

func TestMeshAddLighthouse(t *testing.T) {
	home := t.TempDir()
	mock := exec.NewMockExecutor()
	p := paths.NewWithHome(home, nil)

	fleetDir := t.TempDir()
	cluster := "lh"

	// Create CA key
	caDir := p.NebulaCADir()
	if err := os.MkdirAll(caDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.NebulaCAKey(), []byte("key"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.NebulaCACert(), []byte("cert"), 0600); err != nil {
		t.Fatal(err)
	}

	// Create cluster-settings.yaml with 1 node
	clusterDir := p.FleetClusterDir(fleetDir, cluster)
	if err := os.MkdirAll(clusterDir, 0755); err != nil {
		t.Fatal(err)
	}
	settingsContent := `data:
  nodes: "1"
`
	if err := os.WriteFile(filepath.Join(clusterDir, "cluster-settings.yaml"), []byte(settingsContent), 0644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	err := MeshAdd(MeshAddParams{
		Exec:         mock,
		Paths:        p,
		Stdout:       &buf,
		Cluster:      cluster,
		FleetDir:     fleetDir,
		IsLighthouse: true,
	})
	if err != nil {
		t.Fatalf("MeshAdd returned error: %v", err)
	}

	mock.AssertCallCount(t, 1)

	// Verify the groups contain "lighthouse"
	if len(mock.Calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(mock.Calls))
	}
	call := mock.Calls[0]
	cmdStr := call.CommandString()
	if !strings.Contains(cmdStr, "disentangle,lighthouse") {
		t.Errorf("lighthouse group not found in command: %s", cmdStr)
	}
}

func TestMeshAddNoCluster(t *testing.T) {
	home := t.TempDir()
	mock := exec.NewMockExecutor()
	p := paths.NewWithHome(home, nil)

	fleetDir := t.TempDir()

	// Create CA key (but no cluster directory)
	caDir := p.NebulaCADir()
	if err := os.MkdirAll(caDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.NebulaCAKey(), []byte("key"), 0600); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	err := MeshAdd(MeshAddParams{
		Exec:     mock,
		Paths:    p,
		Stdout:   &buf,
		Cluster:  "nonexistent",
		FleetDir: fleetDir,
	})
	if err == nil {
		t.Fatal("expected error when cluster directory doesn't exist")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got: %v", err)
	}

	mock.AssertCallCount(t, 0)
}

func TestMeshAddNoCA(t *testing.T) {
	home := t.TempDir()
	mock := exec.NewMockExecutor()
	p := paths.NewWithHome(home, nil)

	// Do NOT create CA key

	var buf bytes.Buffer
	err := MeshAdd(MeshAddParams{
		Exec:     mock,
		Paths:    p,
		Stdout:   &buf,
		Cluster:  "test",
		FleetDir: t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected error when CA key doesn't exist")
	}
	if !strings.Contains(err.Error(), "no CA found") {
		t.Errorf("error should mention 'no CA found', got: %v", err)
	}

	mock.AssertCallCount(t, 0)
}
