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

func TestSecretsInitAge(t *testing.T) {
	home := t.TempDir()
	mock := exec.NewMockExecutor()
	p := paths.NewWithHome(home, nil)

	fleetDir := t.TempDir()
	cluster := "dev"

	// Create cluster directory so the check passes
	clusterDir := p.FleetClusterDir(fleetDir, cluster)
	if err := os.MkdirAll(clusterDir, 0755); err != nil {
		t.Fatal(err)
	}

	secretsDir := p.FleetSecretsDir(fleetDir, cluster)
	keyPath := filepath.Join(secretsDir, "age.key")

	// Mock age-keygen command
	mock.ExpectRun(
		fmt.Sprintf("age-keygen -o %s", keyPath),
		"age1abc123publickey", nil,
	)

	var buf bytes.Buffer
	err := SecretsInit(SecretsInitParams{
		Exec:     mock,
		Paths:    p,
		Stdout:   &buf,
		Cluster:  cluster,
		Provider: "age",
		FleetDir: fleetDir,
	})
	if err != nil {
		t.Fatalf("SecretsInit returned error: %v", err)
	}

	// Verify age-keygen was called
	mock.AssertCallCount(t, 1)
	mock.AssertCalled(t, fmt.Sprintf("age-keygen -o %s", keyPath))

	// Verify genesis-config.yaml was written
	cfgPath := filepath.Join(secretsDir, "genesis-config.yaml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("genesis-config.yaml not written: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "provider: age") {
		t.Errorf("genesis-config.yaml should contain 'provider: age', got: %s", content)
	}

	// Verify output mentions the key path
	output := buf.String()
	if !strings.Contains(output, "Age key generated") {
		t.Errorf("output should mention 'Age key generated', got: %s", output)
	}
}

func TestSecretsInitAgeAlreadyExists(t *testing.T) {
	home := t.TempDir()
	mock := exec.NewMockExecutor()
	p := paths.NewWithHome(home, nil)

	fleetDir := t.TempDir()
	cluster := "dev"

	// Create cluster directory
	clusterDir := p.FleetClusterDir(fleetDir, cluster)
	if err := os.MkdirAll(clusterDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create age.key so it's detected as existing
	secretsDir := p.FleetSecretsDir(fleetDir, cluster)
	if err := os.MkdirAll(secretsDir, 0755); err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(secretsDir, "age.key")
	if err := os.WriteFile(keyPath, []byte("existing-age-key"), 0600); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	err := SecretsInit(SecretsInitParams{
		Exec:     mock,
		Paths:    p,
		Stdout:   &buf,
		Cluster:  cluster,
		Provider: "age",
		FleetDir: fleetDir,
	})
	if err != nil {
		t.Fatalf("expected no error for existing key, got: %v", err)
	}

	// No commands should have been executed (early return)
	mock.AssertCallCount(t, 0)

	// Verify output mentions existing key
	output := buf.String()
	if !strings.Contains(output, "already exists") {
		t.Errorf("output should mention 'already exists', got: %s", output)
	}
}

func TestSecretsInitYubikey(t *testing.T) {
	home := t.TempDir()
	mock := exec.NewMockExecutor()
	p := paths.NewWithHome(home, nil)

	fleetDir := t.TempDir()
	cluster := "sec"

	// Create cluster directory
	clusterDir := p.FleetClusterDir(fleetDir, cluster)
	if err := os.MkdirAll(clusterDir, 0755); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	err := SecretsInit(SecretsInitParams{
		Exec:     mock,
		Paths:    p,
		Stdout:   &buf,
		Cluster:  cluster,
		Provider: "yubikey",
		FleetDir: fleetDir,
	})
	if err != nil {
		t.Fatalf("SecretsInit returned error: %v", err)
	}

	// Verify output mentions YubiKey
	output := buf.String()
	if !strings.Contains(output, "YubiKey") {
		t.Errorf("output should mention 'YubiKey', got: %s", output)
	}

	// Verify genesis-config.yaml was written with yubikey provider
	secretsDir := p.FleetSecretsDir(fleetDir, cluster)
	cfgPath := filepath.Join(secretsDir, "genesis-config.yaml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("genesis-config.yaml not written: %v", err)
	}
	if !strings.Contains(string(data), "provider: yubikey") {
		t.Errorf("genesis-config.yaml should contain 'provider: yubikey', got: %s", string(data))
	}
}

func TestSecretsInitOCIVault(t *testing.T) {
	home := t.TempDir()
	mock := exec.NewMockExecutor()
	p := paths.NewWithHome(home, nil)

	fleetDir := t.TempDir()
	cluster := "oci-prod"

	// Create cluster directory
	clusterDir := p.FleetClusterDir(fleetDir, cluster)
	if err := os.MkdirAll(clusterDir, 0755); err != nil {
		t.Fatal(err)
	}

	keyARN := "ocid1.key.oc1.iad.abc123"

	var buf bytes.Buffer
	err := SecretsInit(SecretsInitParams{
		Exec:     mock,
		Paths:    p,
		Stdout:   &buf,
		Cluster:  cluster,
		Provider: "oci-vault",
		KeyARN:   keyARN,
		FleetDir: fleetDir,
	})
	if err != nil {
		t.Fatalf("SecretsInit returned error: %v", err)
	}

	// Verify genesis-config.yaml was written with correct content
	secretsDir := p.FleetSecretsDir(fleetDir, cluster)
	cfgPath := filepath.Join(secretsDir, "genesis-config.yaml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("genesis-config.yaml not written: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "provider: oci-vault") {
		t.Errorf("genesis-config.yaml should contain 'provider: oci-vault', got: %s", content)
	}
	if !strings.Contains(content, keyARN) {
		t.Errorf("genesis-config.yaml should contain key ARN %q, got: %s", keyARN, content)
	}

	// Verify output mentions the KMS key
	output := buf.String()
	if !strings.Contains(output, keyARN) {
		t.Errorf("output should mention KMS key ARN, got: %s", output)
	}
}

func TestSecretsInitMissingKeyARN(t *testing.T) {
	home := t.TempDir()
	mock := exec.NewMockExecutor()
	p := paths.NewWithHome(home, nil)

	fleetDir := t.TempDir()
	cluster := "oci-prod"

	// Create cluster directory
	clusterDir := p.FleetClusterDir(fleetDir, cluster)
	if err := os.MkdirAll(clusterDir, 0755); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	err := SecretsInit(SecretsInitParams{
		Exec:     mock,
		Paths:    p,
		Stdout:   &buf,
		Cluster:  cluster,
		Provider: "oci-vault",
		KeyARN:   "", // missing
		FleetDir: fleetDir,
	})
	if err == nil {
		t.Fatal("expected error when key-arn is missing for oci-vault")
	}
	if !strings.Contains(err.Error(), "--key-arn is required") {
		t.Errorf("error should mention '--key-arn is required', got: %v", err)
	}
}

func TestSecretsInitUnknownProvider(t *testing.T) {
	home := t.TempDir()
	mock := exec.NewMockExecutor()
	p := paths.NewWithHome(home, nil)

	fleetDir := t.TempDir()
	cluster := "test"

	// Create cluster directory
	clusterDir := p.FleetClusterDir(fleetDir, cluster)
	if err := os.MkdirAll(clusterDir, 0755); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	err := SecretsInit(SecretsInitParams{
		Exec:     mock,
		Paths:    p,
		Stdout:   &buf,
		Cluster:  cluster,
		Provider: "invalid",
		FleetDir: fleetDir,
	})
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
	if !strings.Contains(err.Error(), "unknown provider") {
		t.Errorf("error should mention 'unknown provider', got: %v", err)
	}
	// Verify it lists valid providers
	if !strings.Contains(err.Error(), "age") || !strings.Contains(err.Error(), "yubikey") {
		t.Errorf("error should list valid providers, got: %v", err)
	}
}

func TestSecretsInitNoCluster(t *testing.T) {
	home := t.TempDir()
	mock := exec.NewMockExecutor()
	p := paths.NewWithHome(home, nil)

	fleetDir := t.TempDir()
	// Do NOT create cluster directory

	var buf bytes.Buffer
	err := SecretsInit(SecretsInitParams{
		Exec:     mock,
		Paths:    p,
		Stdout:   &buf,
		Cluster:  "nonexistent",
		Provider: "age",
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
