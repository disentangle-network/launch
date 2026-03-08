package cmd

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/disentangle-network/launch/internal/config"
	"github.com/disentangle-network/launch/internal/exec"
	"github.com/disentangle-network/launch/internal/paths"
	"gopkg.in/yaml.v3"
)

// createStubCerts writes stub cert/key files so that writeCertSecret can read them.
func createStubCerts(t *testing.T, secretsDir, cluster string, nodeCount int) {
	t.Helper()
	if err := os.MkdirAll(secretsDir, 0750); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= nodeCount; i++ {
		certName := fmt.Sprintf("%s-node%d", cluster, i)
		if err := os.WriteFile(filepath.Join(secretsDir, certName+".crt"), []byte(certName+"-crt-data"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(secretsDir, certName+".key"), []byte(certName+"-key-data"), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

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
	if err := os.WriteFile(p.NebulaCACert(), []byte("ca-cert-data"), 0600); err != nil {
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

	// Pre-create stub cert/key files (simulating nebula-cert sign output)
	secretsDir := p.FleetSecretsDir(fleetDir, cluster)
	createStubCerts(t, secretsDir, cluster, 3)

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

	// Verify Secret manifest was written
	secretManifest, err := os.ReadFile(filepath.Join(secretsDir, "nebula-certs.yaml"))
	if err != nil {
		t.Fatalf("nebula-certs.yaml not written: %v", err)
	}
	secretStr := string(secretManifest)
	if !strings.Contains(secretStr, "kind: Secret") {
		t.Error("secret manifest missing 'kind: Secret'")
	}
	if !strings.Contains(secretStr, base64.StdEncoding.EncodeToString([]byte("ca-cert-data"))) {
		t.Error("secret manifest missing base64-encoded CA cert")
	}
	for i := 1; i <= 3; i++ {
		certName := fmt.Sprintf("%s-node%d", cluster, i)
		if !strings.Contains(secretStr, certName+".crt:") {
			t.Errorf("secret manifest missing %s.crt entry", certName)
		}
		if !strings.Contains(secretStr, certName+".key:") {
			t.Errorf("secret manifest missing %s.key entry", certName)
		}
	}

	// Verify values overlay was written
	valuesData, err := os.ReadFile(filepath.Join(clusterDir, "nebula-values.yaml"))
	if err != nil {
		t.Fatalf("nebula-values.yaml not written: %v", err)
	}
	valuesStr := string(valuesData)
	if !strings.Contains(valuesStr, "enabled: true") {
		t.Error("values overlay missing 'enabled: true'")
	}
	if !strings.Contains(valuesStr, "mode: node") {
		t.Error("values overlay should have mode: node for non-lighthouse cluster")
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

	// Pre-create stub cert files
	secretsDir := p.FleetSecretsDir(fleetDir, cluster)
	createStubCerts(t, secretsDir, cluster, 1)

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

	// Verify values overlay has mode: lighthouse
	valuesData, err := os.ReadFile(filepath.Join(clusterDir, "nebula-values.yaml"))
	if err != nil {
		t.Fatalf("nebula-values.yaml not written: %v", err)
	}
	if !strings.Contains(string(valuesData), "mode: lighthouse") {
		t.Errorf("values overlay should have mode: lighthouse, got: %s", string(valuesData))
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

func TestMeshAddLighthouseAddressWiring(t *testing.T) {
	home := t.TempDir()
	mock := exec.NewMockExecutor()
	p := paths.NewWithHome(home, nil)

	fleetDir := t.TempDir()
	cluster := "edge"

	// Create CA key and cert
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

	// Create cluster-settings.yaml with 2 nodes
	clusterDir := p.FleetClusterDir(fleetDir, cluster)
	if err := os.MkdirAll(clusterDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(clusterDir, "cluster-settings.yaml"),
		[]byte("data:\n  nodes: \"2\"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Pre-create stub cert files
	secretsDir := p.FleetSecretsDir(fleetDir, cluster)
	createStubCerts(t, secretsDir, cluster, 2)

	var buf bytes.Buffer
	err := MeshAdd(MeshAddParams{
		Exec:       mock,
		Paths:      p,
		Stdout:     &buf,
		Cluster:    cluster,
		FleetDir:   fleetDir,
		Lighthouse: "10.42.0.1=lighthouse.disentangle.network:4242",
	})
	if err != nil {
		t.Fatalf("MeshAdd returned error: %v", err)
	}

	// Verify values overlay has staticHostMap and relay
	valuesData, err := os.ReadFile(filepath.Join(clusterDir, "nebula-values.yaml"))
	if err != nil {
		t.Fatalf("nebula-values.yaml not written: %v", err)
	}

	var values map[string]any
	if err := yaml.Unmarshal(valuesData, &values); err != nil {
		t.Fatalf("failed to parse values overlay: %v", err)
	}

	nebula, ok := values["nebula"].(map[string]any)
	if !ok {
		t.Fatal("values missing 'nebula' key")
	}

	// Check staticHostMap
	hostMap, ok := nebula["staticHostMap"].(map[string]any)
	if !ok {
		t.Fatal("values missing 'staticHostMap'")
	}
	if hostMap["10.42.0.1"] != "lighthouse.disentangle.network:4242" {
		t.Errorf("staticHostMap wrong, got: %v", hostMap)
	}

	// Check relay
	relay, ok := nebula["relay"].(map[string]any)
	if !ok {
		t.Fatal("values missing 'relay'")
	}
	relays, ok := relay["relays"].([]any)
	if !ok || len(relays) != 1 || relays[0] != "10.42.0.1" {
		t.Errorf("relay.relays wrong, got: %v", relay["relays"])
	}

	// Check mode is node (not lighthouse)
	if nebula["mode"] != "node" {
		t.Errorf("expected mode: node, got: %v", nebula["mode"])
	}
}

func TestMeshAddSecretManifestContents(t *testing.T) {
	home := t.TempDir()
	mock := exec.NewMockExecutor()
	p := paths.NewWithHome(home, nil)

	fleetDir := t.TempDir()
	cluster := "sec"

	// Create CA key and cert
	caDir := p.NebulaCADir()
	if err := os.MkdirAll(caDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.NebulaCAKey(), []byte("key"), 0600); err != nil {
		t.Fatal(err)
	}
	caCertContent := "my-ca-cert-content"
	if err := os.WriteFile(p.NebulaCACert(), []byte(caCertContent), 0600); err != nil {
		t.Fatal(err)
	}

	// Create cluster-settings.yaml with 1 node
	clusterDir := p.FleetClusterDir(fleetDir, cluster)
	if err := os.MkdirAll(clusterDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(clusterDir, "cluster-settings.yaml"),
		[]byte("data:\n  nodes: \"1\"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Pre-create stub cert files with known content
	secretsDir := p.FleetSecretsDir(fleetDir, cluster)
	if err := os.MkdirAll(secretsDir, 0750); err != nil {
		t.Fatal(err)
	}
	certContent := "node-cert-content"
	keyContent := "node-key-content"
	if err := os.WriteFile(filepath.Join(secretsDir, "sec-node1.crt"), []byte(certContent), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(secretsDir, "sec-node1.key"), []byte(keyContent), 0600); err != nil {
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

	secretData, err := os.ReadFile(filepath.Join(secretsDir, "nebula-certs.yaml"))
	if err != nil {
		t.Fatalf("nebula-certs.yaml not written: %v", err)
	}
	manifest := string(secretData)

	// Verify base64 encoding
	expectedCA := base64.StdEncoding.EncodeToString([]byte(caCertContent))
	if !strings.Contains(manifest, "ca.crt: "+expectedCA) {
		t.Errorf("manifest missing correct ca.crt base64, got:\n%s", manifest)
	}

	expectedCert := base64.StdEncoding.EncodeToString([]byte(certContent))
	if !strings.Contains(manifest, "sec-node1.crt: "+expectedCert) {
		t.Errorf("manifest missing correct node cert base64, got:\n%s", manifest)
	}

	expectedKey := base64.StdEncoding.EncodeToString([]byte(keyContent))
	if !strings.Contains(manifest, "sec-node1.key: "+expectedKey) {
		t.Errorf("manifest missing correct node key base64, got:\n%s", manifest)
	}

	// Verify namespace
	if !strings.Contains(manifest, "namespace: disentangle") {
		t.Error("manifest missing namespace: disentangle")
	}
}

func TestParseLighthouse(t *testing.T) {
	tests := []struct {
		input    string
		vpnIP    string
		realAddr string
		wantErr  bool
	}{
		{"10.42.0.1=lighthouse.example.com:4242", "10.42.0.1", "lighthouse.example.com:4242", false},
		{"192.168.1.1=10.0.0.5:4242", "192.168.1.1", "10.0.0.5:4242", false},
		{"badformat", "", "", true},
		{"=missing-vpn:4242", "", "", true},
		{"missing-addr=", "", "", true},
		{"", "", "", true},
	}

	for _, tt := range tests {
		vpnIP, realAddr, err := parseLighthouse(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("parseLighthouse(%q): expected error, got nil", tt.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseLighthouse(%q): unexpected error: %v", tt.input, err)
			continue
		}
		if vpnIP != tt.vpnIP {
			t.Errorf("parseLighthouse(%q): vpnIP = %q, want %q", tt.input, vpnIP, tt.vpnIP)
		}
		if realAddr != tt.realAddr {
			t.Errorf("parseLighthouse(%q): realAddr = %q, want %q", tt.input, realAddr, tt.realAddr)
		}
	}
}

func TestMeshAddInvalidLighthouseFormat(t *testing.T) {
	home := t.TempDir()
	mock := exec.NewMockExecutor()
	p := paths.NewWithHome(home, nil)

	// Create CA key
	caDir := p.NebulaCADir()
	if err := os.MkdirAll(caDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.NebulaCAKey(), []byte("key"), 0600); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	err := MeshAdd(MeshAddParams{
		Exec:       mock,
		Paths:      p,
		Stdout:     &buf,
		Cluster:    "test",
		FleetDir:   t.TempDir(),
		Lighthouse: "invalid-format",
	})
	if err == nil {
		t.Fatal("expected error for invalid lighthouse format")
	}
	if !strings.Contains(err.Error(), "invalid lighthouse format") {
		t.Errorf("error should mention 'invalid lighthouse format', got: %v", err)
	}

	mock.AssertCallCount(t, 0)
}

func TestMeshAddOutputMentionsFiles(t *testing.T) {
	home := t.TempDir()
	mock := exec.NewMockExecutor()
	p := paths.NewWithHome(home, nil)

	fleetDir := t.TempDir()
	cluster := "out"

	// Create CA key and cert
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

	clusterDir := p.FleetClusterDir(fleetDir, cluster)
	if err := os.MkdirAll(clusterDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(clusterDir, "cluster-settings.yaml"),
		[]byte("data:\n  nodes: \"1\"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	secretsDir := p.FleetSecretsDir(fleetDir, cluster)
	createStubCerts(t, secretsDir, cluster, 1)

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

	output := buf.String()
	if !strings.Contains(output, "nebula-certs.yaml") {
		t.Errorf("output should mention nebula-certs.yaml, got: %s", output)
	}
	if !strings.Contains(output, "nebula-values.yaml") {
		t.Errorf("output should mention nebula-values.yaml, got: %s", output)
	}
}
