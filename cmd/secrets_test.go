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

	// Mock age-keygen command with standard output format
	mock.ExpectRun(
		fmt.Sprintf("age-keygen -o %s", keyPath),
		"# public key: age1abc123publickey\n", nil,
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

	// Verify .sops.yaml was created with correct rule
	sopsPath := filepath.Join(fleetDir, ".sops.yaml")
	sopsData, err := os.ReadFile(sopsPath)
	if err != nil {
		t.Fatalf(".sops.yaml not created: %v", err)
	}
	sopsContent := string(sopsData)
	if !strings.Contains(sopsContent, "secrets/dev/.*") {
		t.Errorf(".sops.yaml should contain path_regex for dev, got: %s", sopsContent)
	}
	if !strings.Contains(sopsContent, "age1abc123publickey") {
		t.Errorf(".sops.yaml should contain the public key, got: %s", sopsContent)
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
	if !strings.Contains(err.Error(), "local") {
		t.Errorf("error should list 'local' as valid provider, got: %v", err)
	}
}

func TestSecretsInitLocal(t *testing.T) {
	genesisExistsFunc = func(name string) bool { return true }
	defer func() { genesisExistsFunc = exec.CommandExists }()

	home := t.TempDir()
	mock := exec.NewMockExecutor()
	p := paths.NewWithHome(home, nil)

	fleetDir := t.TempDir()
	cluster := "pq-cluster"

	// Create cluster directory
	clusterDir := p.FleetClusterDir(fleetDir, cluster)
	if err := os.MkdirAll(clusterDir, 0755); err != nil {
		t.Fatal(err)
	}

	secretsDir := p.FleetSecretsDir(fleetDir, cluster)
	envelopePath := filepath.Join(secretsDir, "master-key.enc")

	// Mock genesis init command
	genesisCmd := fmt.Sprintf("genesis init --provider local --envelope-path %s --output %s",
		envelopePath, secretsDir)
	mock.ExpectRun(genesisCmd, "Genesis initialized successfully!\n  Public Key: age1test123\n", nil)

	var buf bytes.Buffer
	err := SecretsInit(SecretsInitParams{
		Exec:     mock,
		Paths:    p,
		Stdout:   &buf,
		Cluster:  cluster,
		Provider: "local",
		FleetDir: fleetDir,
	})
	if err != nil {
		t.Fatalf("SecretsInit returned error: %v", err)
	}

	// Verify genesis was called
	mock.AssertCalled(t, genesisCmd)

	// Verify genesis-config.yaml was written
	cfgPath := filepath.Join(secretsDir, "genesis-config.yaml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("genesis-config.yaml not written: %v", err)
	}
	if !strings.Contains(string(data), "provider: local") {
		t.Errorf("expected 'provider: local', got: %s", string(data))
	}

	// Verify output
	output := buf.String()
	if !strings.Contains(output, "Genesis PQ initialized") {
		t.Errorf("output should mention 'Genesis PQ initialized', got: %s", output)
	}
}

func TestSecretsInitLocalAlreadyExists(t *testing.T) {
	genesisExistsFunc = func(name string) bool { return true }
	defer func() { genesisExistsFunc = exec.CommandExists }()

	home := t.TempDir()
	mock := exec.NewMockExecutor()
	p := paths.NewWithHome(home, nil)

	fleetDir := t.TempDir()
	cluster := "existing"

	// Create cluster directory
	clusterDir := p.FleetClusterDir(fleetDir, cluster)
	if err := os.MkdirAll(clusterDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create existing genesis-bootstrap.yaml
	secretsDir := p.FleetSecretsDir(fleetDir, cluster)
	if err := os.MkdirAll(secretsDir, 0755); err != nil {
		t.Fatal(err)
	}
	bootstrapPath := filepath.Join(secretsDir, "genesis-bootstrap.yaml")
	if err := os.WriteFile(bootstrapPath, []byte("existing"), 0600); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	err := SecretsInit(SecretsInitParams{
		Exec:     mock,
		Paths:    p,
		Stdout:   &buf,
		Cluster:  cluster,
		Provider: "local",
		FleetDir: fleetDir,
	})
	if err != nil {
		t.Fatalf("expected no error for existing init, got: %v", err)
	}

	// genesis should NOT have been called
	mock.AssertCallCount(t, 0)

	output := buf.String()
	if !strings.Contains(output, "already initialized") {
		t.Errorf("output should mention 'already initialized', got: %s", output)
	}
}

func TestSecretsInitLocalGenesisFailure(t *testing.T) {
	genesisExistsFunc = func(name string) bool { return true }
	defer func() { genesisExistsFunc = exec.CommandExists }()

	home := t.TempDir()
	mock := exec.NewMockExecutor()
	p := paths.NewWithHome(home, nil)

	fleetDir := t.TempDir()
	cluster := "fail"

	clusterDir := p.FleetClusterDir(fleetDir, cluster)
	if err := os.MkdirAll(clusterDir, 0755); err != nil {
		t.Fatal(err)
	}

	secretsDir := p.FleetSecretsDir(fleetDir, cluster)
	envelopePath := filepath.Join(secretsDir, "master-key.enc")

	// Mock genesis init to fail
	genesisCmd := fmt.Sprintf("genesis init --provider local --envelope-path %s --output %s",
		envelopePath, secretsDir)
	mock.ExpectRun(genesisCmd, "", fmt.Errorf("exit status 1"))

	var buf bytes.Buffer
	err := SecretsInit(SecretsInitParams{
		Exec:     mock,
		Paths:    p,
		Stdout:   &buf,
		Cluster:  cluster,
		Provider: "local",
		FleetDir: fleetDir,
	})
	if err == nil {
		t.Fatal("expected error when genesis fails")
	}
	if !strings.Contains(err.Error(), "genesis init failed") {
		t.Errorf("error should mention 'genesis init failed', got: %v", err)
	}
}

func TestSecretsInitLocalMissingGenesis(t *testing.T) {
	genesisExistsFunc = func(name string) bool { return false }
	defer func() { genesisExistsFunc = exec.CommandExists }()

	home := t.TempDir()
	mock := exec.NewMockExecutor()
	p := paths.NewWithHome(home, nil)

	fleetDir := t.TempDir()
	cluster := "no-genesis"

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
		Provider: "local",
		FleetDir: fleetDir,
	})
	if err == nil {
		t.Fatal("expected error when genesis not installed")
	}
	if !strings.Contains(err.Error(), "genesis CLI not found") {
		t.Errorf("error should mention 'genesis CLI not found', got: %v", err)
	}
	mock.AssertCallCount(t, 0)
}

func TestSecretsInitNoCluster(t *testing.T) {
	home := t.TempDir()
	mock := exec.NewMockExecutor()
	p := paths.NewWithHome(home, nil)

	fleetDir := t.TempDir()
	cluster := "nonexistent"
	// Do NOT create cluster directory

	secretsDir := p.FleetSecretsDir(fleetDir, cluster)
	keyPath := filepath.Join(secretsDir, "age.key")

	// Mock age-keygen since it should proceed despite missing cluster dir
	mock.ExpectRun(
		fmt.Sprintf("age-keygen -o %s", keyPath),
		"# public key: age1noclusterkey\n", nil,
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
		t.Fatalf("expected no error (warning only), got: %v", err)
	}

	// Verify the warning is printed
	output := buf.String()
	if !strings.Contains(output, "Note: cluster 'nonexistent' not yet added") {
		t.Errorf("output should contain cluster warning, got: %s", output)
	}

	// Verify genesis-config.yaml was still written
	cfgPath := filepath.Join(secretsDir, "genesis-config.yaml")
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		t.Error("genesis-config.yaml should have been written despite missing cluster dir")
	}

	mock.AssertCallCount(t, 1)
}

func TestSecretsInitAgeUpdatesSOPS(t *testing.T) {
	home := t.TempDir()
	mock := exec.NewMockExecutor()
	p := paths.NewWithHome(home, nil)

	fleetDir := t.TempDir()
	cluster := "new-cluster"

	// Create cluster directory
	clusterDir := p.FleetClusterDir(fleetDir, cluster)
	if err := os.MkdirAll(clusterDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create existing .sops.yaml with a different cluster's rule
	existingSops := `creation_rules:
    - path_regex: secrets/old-cluster/.*
      age: age1oldkey
`
	sopsPath := filepath.Join(fleetDir, ".sops.yaml")
	if err := os.WriteFile(sopsPath, []byte(existingSops), 0600); err != nil {
		t.Fatal(err)
	}

	secretsDir := p.FleetSecretsDir(fleetDir, cluster)
	keyPath := filepath.Join(secretsDir, "age.key")

	mock.ExpectRun(
		fmt.Sprintf("age-keygen -o %s", keyPath),
		"# public key: age1newclusterkey\n", nil,
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

	// Verify .sops.yaml was updated with BOTH rules
	sopsData, err := os.ReadFile(sopsPath)
	if err != nil {
		t.Fatalf("could not read .sops.yaml: %v", err)
	}
	sopsContent := string(sopsData)
	if !strings.Contains(sopsContent, "secrets/old-cluster/.*") {
		t.Errorf(".sops.yaml should still contain old rule, got: %s", sopsContent)
	}
	if !strings.Contains(sopsContent, "secrets/new-cluster/.*") {
		t.Errorf(".sops.yaml should contain new rule, got: %s", sopsContent)
	}
	if !strings.Contains(sopsContent, "age1newclusterkey") {
		t.Errorf(".sops.yaml should contain new key, got: %s", sopsContent)
	}

	output := buf.String()
	if !strings.Contains(output, "Updated") {
		t.Errorf("output should mention 'Updated', got: %s", output)
	}
}

func TestSecretsInitAgeCreatesSOPS(t *testing.T) {
	home := t.TempDir()
	mock := exec.NewMockExecutor()
	p := paths.NewWithHome(home, nil)

	fleetDir := t.TempDir()
	cluster := "fresh"

	// Create cluster directory
	clusterDir := p.FleetClusterDir(fleetDir, cluster)
	if err := os.MkdirAll(clusterDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Do NOT create .sops.yaml -- it should be created automatically

	secretsDir := p.FleetSecretsDir(fleetDir, cluster)
	keyPath := filepath.Join(secretsDir, "age.key")

	mock.ExpectRun(
		fmt.Sprintf("age-keygen -o %s", keyPath),
		"# public key: age1freshkey\n", nil,
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

	// Verify .sops.yaml was created
	sopsPath := filepath.Join(fleetDir, ".sops.yaml")
	sopsData, err := os.ReadFile(sopsPath)
	if err != nil {
		t.Fatalf(".sops.yaml should have been created: %v", err)
	}
	sopsContent := string(sopsData)
	if !strings.Contains(sopsContent, "secrets/fresh/.*") {
		t.Errorf(".sops.yaml should contain path_regex, got: %s", sopsContent)
	}
	if !strings.Contains(sopsContent, "age1freshkey") {
		t.Errorf(".sops.yaml should contain the public key, got: %s", sopsContent)
	}
}

func TestSecretsInitLocalUpdatesSOPS(t *testing.T) {
	genesisExistsFunc = func(name string) bool { return true }
	defer func() { genesisExistsFunc = exec.CommandExists }()

	home := t.TempDir()
	mock := exec.NewMockExecutor()
	p := paths.NewWithHome(home, nil)

	fleetDir := t.TempDir()
	cluster := "local-sops"

	// Create cluster directory
	clusterDir := p.FleetClusterDir(fleetDir, cluster)
	if err := os.MkdirAll(clusterDir, 0755); err != nil {
		t.Fatal(err)
	}

	secretsDir := p.FleetSecretsDir(fleetDir, cluster)
	envelopePath := filepath.Join(secretsDir, "master-key.enc")

	// Pre-create the genesis-generated .sops.yaml in secrets dir
	// (the mock executor won't actually run genesis, so we create it ourselves)
	if err := os.MkdirAll(secretsDir, 0755); err != nil {
		t.Fatal(err)
	}
	genSops := `creation_rules:
    - path_regex: "secrets/local-sops/.*"
      age: age1genesisgeneratedkey
`
	clusterSopsPath := filepath.Join(secretsDir, ".sops.yaml")
	if err := os.WriteFile(clusterSopsPath, []byte(genSops), 0600); err != nil {
		t.Fatal(err)
	}

	// Mock genesis init command
	genesisCmd := fmt.Sprintf("genesis init --provider local --envelope-path %s --output %s",
		envelopePath, secretsDir)
	mock.ExpectRun(genesisCmd, "Genesis initialized successfully!\n", nil)

	var buf bytes.Buffer
	err := SecretsInit(SecretsInitParams{
		Exec:     mock,
		Paths:    p,
		Stdout:   &buf,
		Cluster:  cluster,
		Provider: "local",
		FleetDir: fleetDir,
	})
	if err != nil {
		t.Fatalf("SecretsInit returned error: %v", err)
	}

	// Verify fleet root .sops.yaml was created with the genesis key
	sopsPath := filepath.Join(fleetDir, ".sops.yaml")
	sopsData, err := os.ReadFile(sopsPath)
	if err != nil {
		t.Fatalf(".sops.yaml should have been created: %v", err)
	}
	sopsContent := string(sopsData)
	if !strings.Contains(sopsContent, "secrets/local-sops/.*") {
		t.Errorf(".sops.yaml should contain path_regex, got: %s", sopsContent)
	}
	if !strings.Contains(sopsContent, "age1genesisgeneratedkey") {
		t.Errorf(".sops.yaml should contain genesis key, got: %s", sopsContent)
	}

	output := buf.String()
	if !strings.Contains(output, "Updated") {
		t.Errorf("output should mention 'Updated', got: %s", output)
	}
}

func TestExtractAgePublicKey(t *testing.T) {
	tests := []struct {
		name   string
		result *exec.Result
		want   string
	}{
		{
			name: "age-keygen v1.3+ format on stderr",
			result: &exec.Result{
				Stderr: "Public key: age1t65wxyqk7ypcxxk42ak5fqnvt5rr387pm2g6x46n2k9u0yf3fanqw43vkw\n",
			},
			want: "age1t65wxyqk7ypcxxk42ak5fqnvt5rr387pm2g6x46n2k9u0yf3fanqw43vkw",
		},
		{
			name: "old age-keygen comment format",
			result: &exec.Result{
				Stdout: "# created: 2024-01-01T00:00:00Z\n# public key: age1abc123def456\nAGE-SECRET-KEY-1...\n",
			},
			want: "age1abc123def456",
		},
		{
			name: "public key on stderr with comment prefix",
			result: &exec.Result{
				Stderr: "# public key: age1stderrkey789\n",
			},
			want: "age1stderrkey789",
		},
		{
			name: "bare age1 key output",
			result: &exec.Result{
				Stdout: "age1barekey000\n",
			},
			want: "age1barekey000",
		},
		{
			name:   "nil result",
			result: nil,
			want:   "",
		},
		{
			name: "no key in output",
			result: &exec.Result{
				Stdout: "some random output\nno key here\n",
			},
			want: "",
		},
		{
			name: "empty output",
			result: &exec.Result{
				Stdout: "",
				Stderr: "",
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractAgePublicKey(tt.result)
			if got != tt.want {
				t.Errorf("extractAgePublicKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAppendSOPSRule_NewFile(t *testing.T) {
	fleetDir := t.TempDir()

	err := appendSOPSRule(fleetDir, "test-cluster", "age1newfilekey")
	if err != nil {
		t.Fatalf("appendSOPSRule returned error: %v", err)
	}

	sopsPath := filepath.Join(fleetDir, ".sops.yaml")
	data, err := os.ReadFile(sopsPath)
	if err != nil {
		t.Fatalf(".sops.yaml not created: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "secrets/test-cluster/.*") {
		t.Errorf("should contain path_regex, got: %s", content)
	}
	if !strings.Contains(content, "age1newfilekey") {
		t.Errorf("should contain age key, got: %s", content)
	}
}

func TestAppendSOPSRule_ExistingFile(t *testing.T) {
	fleetDir := t.TempDir()

	// Create initial rule
	if err := appendSOPSRule(fleetDir, "cluster-a", "age1keya"); err != nil {
		t.Fatalf("first appendSOPSRule failed: %v", err)
	}

	// Append second rule
	if err := appendSOPSRule(fleetDir, "cluster-b", "age1keyb"); err != nil {
		t.Fatalf("second appendSOPSRule failed: %v", err)
	}

	sopsPath := filepath.Join(fleetDir, ".sops.yaml")
	data, err := os.ReadFile(sopsPath)
	if err != nil {
		t.Fatalf(".sops.yaml not found: %v", err)
	}
	content := string(data)

	// Both rules should be present
	if !strings.Contains(content, "secrets/cluster-a/.*") {
		t.Errorf("should contain cluster-a rule, got: %s", content)
	}
	if !strings.Contains(content, "age1keya") {
		t.Errorf("should contain cluster-a key, got: %s", content)
	}
	if !strings.Contains(content, "secrets/cluster-b/.*") {
		t.Errorf("should contain cluster-b rule, got: %s", content)
	}
	if !strings.Contains(content, "age1keyb") {
		t.Errorf("should contain cluster-b key, got: %s", content)
	}
}

func TestAppendSOPSRule_UpdateExisting(t *testing.T) {
	fleetDir := t.TempDir()

	// Create initial rule
	if err := appendSOPSRule(fleetDir, "my-cluster", "age1oldkey"); err != nil {
		t.Fatalf("first appendSOPSRule failed: %v", err)
	}

	// Update same cluster with new key
	if err := appendSOPSRule(fleetDir, "my-cluster", "age1newkey"); err != nil {
		t.Fatalf("second appendSOPSRule failed: %v", err)
	}

	sopsPath := filepath.Join(fleetDir, ".sops.yaml")
	data, err := os.ReadFile(sopsPath)
	if err != nil {
		t.Fatalf(".sops.yaml not found: %v", err)
	}
	content := string(data)

	// Should have new key, not old key
	if strings.Contains(content, "age1oldkey") {
		t.Errorf("should NOT contain old key, got: %s", content)
	}
	if !strings.Contains(content, "age1newkey") {
		t.Errorf("should contain new key, got: %s", content)
	}

	// Should only have one rule (not duplicated)
	count := strings.Count(content, "secrets/my-cluster/.*")
	if count != 1 {
		t.Errorf("should have exactly 1 rule for my-cluster, found %d in: %s", count, content)
	}
}

func TestSecretsInitAgeV13Format(t *testing.T) {
	// age-keygen v1.3+ outputs "Public key: age1..." on stderr (no # prefix)
	home := t.TempDir()
	mock := exec.NewMockExecutor()
	p := paths.NewWithHome(home, nil)

	fleetDir := t.TempDir()
	cluster := "v13-cluster"

	clusterDir := p.FleetClusterDir(fleetDir, cluster)
	if err := os.MkdirAll(clusterDir, 0755); err != nil {
		t.Fatal(err)
	}

	secretsDir := p.FleetSecretsDir(fleetDir, cluster)
	keyPath := filepath.Join(secretsDir, "age.key")

	// Simulate v1.3+ output: "Public key:" on stderr, no stdout
	mock.ExpectRunWithResult(
		fmt.Sprintf("age-keygen -o %s", keyPath),
		&exec.Result{
			Command: "age-keygen",
			Stderr:  "Public key: age1realv13publickey\n",
		},
		nil,
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

	// Verify .sops.yaml was created with the extracted key
	sopsPath := filepath.Join(fleetDir, ".sops.yaml")
	sopsData, err := os.ReadFile(sopsPath)
	if err != nil {
		t.Fatalf(".sops.yaml not created: %v", err)
	}
	sopsContent := string(sopsData)
	if !strings.Contains(sopsContent, "age1realv13publickey") {
		t.Errorf(".sops.yaml should contain the v1.3 public key, got: %s", sopsContent)
	}
	if !strings.Contains(sopsContent, "secrets/v13-cluster/.*") {
		t.Errorf(".sops.yaml should contain path_regex, got: %s", sopsContent)
	}

	output := buf.String()
	if !strings.Contains(output, "Updated") {
		t.Errorf("output should mention 'Updated', got: %s", output)
	}
}
