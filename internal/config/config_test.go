package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromExplicitPath(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "test-config.yaml")

	content := `oci_region: us-ashburn-1
oci_compartment_id: ocid1.compartment.test
cloudflare_account_id: cf-account-123
domain: example.com
github_org: disentangle-network
github_user: testuser
repos:
  oci_tf_bootstrap: /path/to/bootstrap
  k8s_oci_foundation: /path/to/foundation
  genesis_operator: /path/to/genesis
  deploy: /path/to/deploy
cluster_name: dev-cluster
environment: dev
protocol_image: ghcr.io/disentangle-network/protocol
protocol_version: v0.3.1
genesis_image: ghcr.io/larsenclose/genesis-operator
genesis_version: v0.1.0
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load(%q) returned error: %v", cfgPath, err)
	}

	if cfg.OCIRegion != "us-ashburn-1" {
		t.Errorf("OCIRegion = %q, want %q", cfg.OCIRegion, "us-ashburn-1")
	}
	if cfg.OCICompartmentID != "ocid1.compartment.test" {
		t.Errorf("OCICompartmentID = %q, want %q", cfg.OCICompartmentID, "ocid1.compartment.test")
	}
	if cfg.CloudflareAccountID != "cf-account-123" {
		t.Errorf("CloudflareAccountID = %q, want %q", cfg.CloudflareAccountID, "cf-account-123")
	}
	if cfg.Domain != "example.com" {
		t.Errorf("Domain = %q, want %q", cfg.Domain, "example.com")
	}
	if cfg.GitHubOrg != "disentangle-network" {
		t.Errorf("GitHubOrg = %q, want %q", cfg.GitHubOrg, "disentangle-network")
	}
	if cfg.GitHubUser != "testuser" {
		t.Errorf("GitHubUser = %q, want %q", cfg.GitHubUser, "testuser")
	}
	if cfg.Repos.OCITFBootstrap != "/path/to/bootstrap" {
		t.Errorf("Repos.OCITFBootstrap = %q, want %q", cfg.Repos.OCITFBootstrap, "/path/to/bootstrap")
	}
	if cfg.Repos.K8sOCIFoundation != "/path/to/foundation" {
		t.Errorf("Repos.K8sOCIFoundation = %q, want %q", cfg.Repos.K8sOCIFoundation, "/path/to/foundation")
	}
	if cfg.Repos.GenesisOperator != "/path/to/genesis" {
		t.Errorf("Repos.GenesisOperator = %q, want %q", cfg.Repos.GenesisOperator, "/path/to/genesis")
	}
	if cfg.Repos.Deploy != "/path/to/deploy" {
		t.Errorf("Repos.Deploy = %q, want %q", cfg.Repos.Deploy, "/path/to/deploy")
	}
	if cfg.ClusterName != "dev-cluster" {
		t.Errorf("ClusterName = %q, want %q", cfg.ClusterName, "dev-cluster")
	}
	if cfg.Environment != "dev" {
		t.Errorf("Environment = %q, want %q", cfg.Environment, "dev")
	}
	if cfg.ProtocolImage != "ghcr.io/disentangle-network/protocol" {
		t.Errorf("ProtocolImage = %q, want %q", cfg.ProtocolImage, "ghcr.io/disentangle-network/protocol")
	}
	if cfg.ProtocolVersion != "v0.3.1" {
		t.Errorf("ProtocolVersion = %q, want %q", cfg.ProtocolVersion, "v0.3.1")
	}
	if cfg.GenesisImage != "ghcr.io/larsenclose/genesis-operator" {
		t.Errorf("GenesisImage = %q, want %q", cfg.GenesisImage, "ghcr.io/larsenclose/genesis-operator")
	}
	if cfg.GenesisVersion != "v0.1.0" {
		t.Errorf("GenesisVersion = %q, want %q", cfg.GenesisVersion, "v0.1.0")
	}
}

func TestLoadNonExistentFileReturnsError(t *testing.T) {
	_, err := Load("/tmp/does-not-exist-launch-test-config.yaml")
	if err == nil {
		t.Fatal("Load with non-existent explicit path should return an error")
	}
}

func TestLoadNoFileReturnsEmptyConfig(t *testing.T) {
	// Use a temp dir as working directory so no .launch.yaml is found.
	// We pass an empty string so Load falls through to default paths.
	// Since we cannot easily control HOME or cwd in a unit test without
	// side effects, we rely on the fact that there is no .launch.yaml
	// in whatever cwd the test runner uses, and the default config path
	// may or may not exist. If the default config file does not exist,
	// we get an empty Config.
	//
	// For a deterministic test, change cwd to a temp dir.
	dir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getting cwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir to temp dir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load(\"\") returned error: %v", err)
	}
	if cfg == nil {
		t.Fatal("Load(\"\") returned nil config")
	}
	// With no config files, all fields should be zero values.
	if cfg.OCIRegion != "" {
		t.Errorf("expected empty OCIRegion, got %q", cfg.OCIRegion)
	}
	if cfg.Domain != "" {
		t.Errorf("expected empty Domain, got %q", cfg.Domain)
	}
}

func TestLoadLocalLaunchYAMLPriority(t *testing.T) {
	dir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getting cwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir to temp dir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })

	// Write a .launch.yaml in the temp dir (now cwd).
	content := "cluster_name: local-cluster\n"
	if err := os.WriteFile(filepath.Join(dir, ".launch.yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("writing .launch.yaml: %v", err)
	}

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load(\"\") returned error: %v", err)
	}
	if cfg.ClusterName != "local-cluster" {
		t.Errorf("ClusterName = %q, want %q", cfg.ClusterName, "local-cluster")
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "subdir", "config.yaml")

	original := &Config{
		OCIRegion:           "us-phoenix-1",
		OCICompartmentID:    "ocid1.compartment.round-trip",
		CloudflareAccountID: "cf-rt-456",
		Domain:              "roundtrip.example.com",
		GitHubOrg:           "test-org",
		GitHubUser:          "test-user",
		Repos: Repos{
			OCITFBootstrap:   "/repos/bootstrap",
			K8sOCIFoundation: "/repos/foundation",
			GenesisOperator:  "/repos/genesis",
			Deploy:           "/repos/deploy",
		},
		ClusterName:     "rt-cluster",
		Environment:     "staging",
		ProtocolImage:   "ghcr.io/test/protocol",
		ProtocolVersion: "v1.0.0",
		GenesisImage:    "ghcr.io/test/genesis",
		GenesisVersion:  "v2.0.0",
	}

	if err := Save(original, cfgPath); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	loaded, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load(%q) returned error: %v", cfgPath, err)
	}

	if loaded.OCIRegion != original.OCIRegion {
		t.Errorf("OCIRegion = %q, want %q", loaded.OCIRegion, original.OCIRegion)
	}
	if loaded.OCICompartmentID != original.OCICompartmentID {
		t.Errorf("OCICompartmentID = %q, want %q", loaded.OCICompartmentID, original.OCICompartmentID)
	}
	if loaded.CloudflareAccountID != original.CloudflareAccountID {
		t.Errorf("CloudflareAccountID = %q, want %q", loaded.CloudflareAccountID, original.CloudflareAccountID)
	}
	if loaded.Domain != original.Domain {
		t.Errorf("Domain = %q, want %q", loaded.Domain, original.Domain)
	}
	if loaded.GitHubOrg != original.GitHubOrg {
		t.Errorf("GitHubOrg = %q, want %q", loaded.GitHubOrg, original.GitHubOrg)
	}
	if loaded.GitHubUser != original.GitHubUser {
		t.Errorf("GitHubUser = %q, want %q", loaded.GitHubUser, original.GitHubUser)
	}
	if loaded.Repos.OCITFBootstrap != original.Repos.OCITFBootstrap {
		t.Errorf("Repos.OCITFBootstrap = %q, want %q", loaded.Repos.OCITFBootstrap, original.Repos.OCITFBootstrap)
	}
	if loaded.Repos.K8sOCIFoundation != original.Repos.K8sOCIFoundation {
		t.Errorf("Repos.K8sOCIFoundation = %q, want %q", loaded.Repos.K8sOCIFoundation, original.Repos.K8sOCIFoundation)
	}
	if loaded.Repos.GenesisOperator != original.Repos.GenesisOperator {
		t.Errorf("Repos.GenesisOperator = %q, want %q", loaded.Repos.GenesisOperator, original.Repos.GenesisOperator)
	}
	if loaded.Repos.Deploy != original.Repos.Deploy {
		t.Errorf("Repos.Deploy = %q, want %q", loaded.Repos.Deploy, original.Repos.Deploy)
	}
	if loaded.ClusterName != original.ClusterName {
		t.Errorf("ClusterName = %q, want %q", loaded.ClusterName, original.ClusterName)
	}
	if loaded.Environment != original.Environment {
		t.Errorf("Environment = %q, want %q", loaded.Environment, original.Environment)
	}
	if loaded.ProtocolImage != original.ProtocolImage {
		t.Errorf("ProtocolImage = %q, want %q", loaded.ProtocolImage, original.ProtocolImage)
	}
	if loaded.ProtocolVersion != original.ProtocolVersion {
		t.Errorf("ProtocolVersion = %q, want %q", loaded.ProtocolVersion, original.ProtocolVersion)
	}
	if loaded.GenesisImage != original.GenesisImage {
		t.Errorf("GenesisImage = %q, want %q", loaded.GenesisImage, original.GenesisImage)
	}
	if loaded.GenesisVersion != original.GenesisVersion {
		t.Errorf("GenesisVersion = %q, want %q", loaded.GenesisVersion, original.GenesisVersion)
	}
}

func TestSaveCreatesParentDirectories(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "a", "b", "c", "config.yaml")

	cfg := &Config{Environment: "test"}
	if err := Save(cfg, cfgPath); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		t.Fatal("expected config file to exist after Save()")
	}
}
