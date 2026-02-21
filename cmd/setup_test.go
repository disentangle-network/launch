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

// ---------------------------------------------------------------------------
// Common test fixtures for discoverVault tests
// ---------------------------------------------------------------------------

const (
	testCompartmentID  = "ocid1.compartment.oc1..testcomp"
	testVaultID        = "ocid1.vault.oc1..aaaaaaaatest"
	testVaultID2       = "ocid1.vault.oc1..aaaaaaabbbbb"
	testVaultEndpoint  = "https://test-management.kms.us-phoenix-1.oci.com"
	testVaultEndpoint2 = "https://test2-management.kms.us-phoenix-1.oci.com"
	testKeyID          = "ocid1.key.oc1..aaaaaaaatest1"
	testKeyID2         = "ocid1.key.oc1..aaaaaaaatest2"
)

// vaultListCmd returns the full command string for vault list.
func vaultListCmd() string {
	return fmt.Sprintf(
		`oci kms management vault list --all --compartment-id %s --output json --query data[?\"lifecycle-state\"=='ACTIVE']`,
		testCompartmentID,
	)
}

// vaultCreateCmd returns the full command string for vault create.
func vaultCreateCmd(compartment string) string {
	return fmt.Sprintf(
		"oci kms management vault create --compartment-id %s --display-name launch-vault --vault-type DEFAULT --output json --wait-for-state ACTIVE",
		compartment,
	)
}

// keyListCmd returns the full command string for key list.
func keyListCmd() string {
	return fmt.Sprintf(
		`oci kms management key list --all --compartment-id %s --endpoint %s --output json --query data[?\"lifecycle-state\"=='ENABLED']`,
		testCompartmentID, testVaultEndpoint,
	)
}

// keyCreateCmd returns the full command string for key create.
func keyCreateCmd(compartment, endpoint string) string {
	return fmt.Sprintf(
		`oci kms management key create --compartment-id %s --endpoint %s --display-name launch-key --key-shape {"algorithm":"AES","length":32} --output json --wait-for-state ENABLED`,
		compartment, endpoint,
	)
}

// oneVaultJSON returns JSON for a single vault using testVaultID and testVaultEndpoint.
func oneVaultJSON() string {
	return fmt.Sprintf(`[{"id":"%s","display-name":"my-vault","management-endpoint":"%s"}]`, testVaultID, testVaultEndpoint)
}

// oneKeyJSON returns JSON for a single key.
func oneKeyJSON(id, name, algo string) string {
	return fmt.Sprintf(`[{"id":"%s","display-name":"%s","algorithm":"%s"}]`, id, name, algo)
}

// ---------------------------------------------------------------------------
// discoverVault unit tests
// ---------------------------------------------------------------------------

func TestDiscoverVaultExistingVaultAndKey(t *testing.T) {
	mock := exec.NewMockExecutor()
	cfg := &config.Config{OCICompartmentID: testCompartmentID}
	var buf bytes.Buffer

	// One vault, one key
	mock.ExpectRun(vaultListCmd(),
		oneVaultJSON(), nil)
	mock.ExpectRun(keyListCmd(),
		oneKeyJSON(testKeyID, "my-key", "AES"), nil)

	err := discoverVault(mock, &buf, func(string) bool { return false }, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.OCIVaultID != testVaultID {
		t.Errorf("expected OCIVaultID %q, got %q", testVaultID, cfg.OCIVaultID)
	}
	if cfg.OCIVaultEndpoint != testVaultEndpoint {
		t.Errorf("expected OCIVaultEndpoint %q, got %q", testVaultEndpoint, cfg.OCIVaultEndpoint)
	}
	if cfg.OCIVaultKeyOCID != testKeyID {
		t.Errorf("expected OCIVaultKeyOCID %q, got %q", testKeyID, cfg.OCIVaultKeyOCID)
	}

	out := buf.String()
	if !strings.Contains(out, "my-vault") {
		t.Errorf("expected vault name in output, got:\n%s", out)
	}
	if !strings.Contains(out, "my-key") {
		t.Errorf("expected key name in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Vault key OCID saved to config.") {
		t.Errorf("expected save confirmation, got:\n%s", out)
	}
}

func TestDiscoverVaultNoVaultsCreateOne(t *testing.T) {
	mock := exec.NewMockExecutor()
	cfg := &config.Config{OCICompartmentID: testCompartmentID}
	var buf bytes.Buffer

	// Empty vault list
	mock.ExpectRun(vaultListCmd(), "[]", nil)

	// Vault create response
	vaultCreateJSON := fmt.Sprintf(
		`{"data":{"id":"%s","management-endpoint":"%s"}}`,
		testVaultID, testVaultEndpoint,
	)
	mock.ExpectRun(vaultCreateCmd(testCompartmentID), vaultCreateJSON, nil)

	// Empty key list (for the created vault)
	mock.ExpectRun(keyListCmd(), "[]", nil)

	// Key create response
	keyCreateJSON := fmt.Sprintf(`{"data":{"id":"%s"}}`, testKeyID)
	mock.ExpectRun(keyCreateCmd(testCompartmentID, testVaultEndpoint), keyCreateJSON, nil)

	// confirmFn always returns true (create vault + create key)
	err := discoverVault(mock, &buf, func(string) bool { return true }, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.OCIVaultID != testVaultID {
		t.Errorf("expected OCIVaultID %q, got %q", testVaultID, cfg.OCIVaultID)
	}
	if cfg.OCIVaultEndpoint != testVaultEndpoint {
		t.Errorf("expected OCIVaultEndpoint %q, got %q", testVaultEndpoint, cfg.OCIVaultEndpoint)
	}
	if cfg.OCIVaultKeyOCID != testKeyID {
		t.Errorf("expected OCIVaultKeyOCID %q, got %q", testKeyID, cfg.OCIVaultKeyOCID)
	}

	out := buf.String()
	if !strings.Contains(out, "No active vaults found.") {
		t.Errorf("expected no-vaults message, got:\n%s", out)
	}
	if !strings.Contains(out, "Vault created.") {
		t.Errorf("expected vault-created message, got:\n%s", out)
	}
	if !strings.Contains(out, "No enabled keys found") {
		t.Errorf("expected no-keys message, got:\n%s", out)
	}
	if !strings.Contains(out, "Key created:") {
		t.Errorf("expected key-created message, got:\n%s", out)
	}
}

func TestDiscoverVaultNoVaultsDecline(t *testing.T) {
	mock := exec.NewMockExecutor()
	cfg := &config.Config{OCICompartmentID: testCompartmentID}
	var buf bytes.Buffer

	// Empty vault list
	mock.ExpectRun(vaultListCmd(), "[]", nil)

	// Decline creation
	err := discoverVault(mock, &buf, func(string) bool { return false }, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.OCIVaultKeyOCID != "" {
		t.Errorf("expected empty OCIVaultKeyOCID, got %q", cfg.OCIVaultKeyOCID)
	}
	if cfg.OCIVaultID != "" {
		t.Errorf("expected empty OCIVaultID, got %q", cfg.OCIVaultID)
	}
}

func TestDiscoverVaultMultipleVaults(t *testing.T) {
	mock := exec.NewMockExecutor()
	cfg := &config.Config{OCICompartmentID: testCompartmentID}
	var buf bytes.Buffer

	// Two vaults
	twoVaultsJSON := fmt.Sprintf(
		`[{"id":"%s","display-name":"vault-alpha","management-endpoint":"%s"},{"id":"%s","display-name":"vault-beta","management-endpoint":"%s"}]`,
		testVaultID, testVaultEndpoint, testVaultID2, testVaultEndpoint2,
	)
	mock.ExpectRun(vaultListCmd(), twoVaultsJSON, nil)

	// Key list for first vault (the one that gets selected)
	mock.ExpectRun(keyListCmd(),
		oneKeyJSON(testKeyID, "alpha-key", "AES"), nil)

	err := discoverVault(mock, &buf, func(string) bool { return false }, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Available vaults:") {
		t.Errorf("expected 'Available vaults:' header, got:\n%s", out)
	}
	if !strings.Contains(out, "vault-alpha") {
		t.Errorf("expected vault-alpha in listing, got:\n%s", out)
	}
	if !strings.Contains(out, "vault-beta") {
		t.Errorf("expected vault-beta in listing, got:\n%s", out)
	}
	if !strings.Contains(out, "Using vault: vault-alpha") {
		t.Errorf("expected first vault selected, got:\n%s", out)
	}

	// First vault should be selected
	if cfg.OCIVaultID != testVaultID {
		t.Errorf("expected OCIVaultID %q, got %q", testVaultID, cfg.OCIVaultID)
	}
	if cfg.OCIVaultEndpoint != testVaultEndpoint {
		t.Errorf("expected endpoint %q, got %q", testVaultEndpoint, cfg.OCIVaultEndpoint)
	}

	// Key list must have been called with the first vault's endpoint
	mock.AssertCalled(t, keyListCmd())
}

func TestDiscoverVaultMultipleKeys(t *testing.T) {
	mock := exec.NewMockExecutor()
	cfg := &config.Config{OCICompartmentID: testCompartmentID}
	var buf bytes.Buffer

	// One vault
	mock.ExpectRun(vaultListCmd(),
		oneVaultJSON(), nil)

	// Two keys
	twoKeysJSON := fmt.Sprintf(
		`[{"id":"%s","display-name":"key-one","algorithm":"AES"},{"id":"%s","display-name":"key-two","algorithm":"RSA"}]`,
		testKeyID, testKeyID2,
	)
	mock.ExpectRun(keyListCmd(), twoKeysJSON, nil)

	err := discoverVault(mock, &buf, func(string) bool { return false }, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Available keys:") {
		t.Errorf("expected 'Available keys:' header, got:\n%s", out)
	}
	if !strings.Contains(out, "key-one") {
		t.Errorf("expected key-one in listing, got:\n%s", out)
	}
	if !strings.Contains(out, "key-two") {
		t.Errorf("expected key-two in listing, got:\n%s", out)
	}
	if !strings.Contains(out, "Using key: key-one") {
		t.Errorf("expected first key selected, got:\n%s", out)
	}

	// First key should be selected
	if cfg.OCIVaultKeyOCID != testKeyID {
		t.Errorf("expected OCIVaultKeyOCID %q, got %q", testKeyID, cfg.OCIVaultKeyOCID)
	}
}

func TestDiscoverVaultListError(t *testing.T) {
	mock := exec.NewMockExecutor()
	cfg := &config.Config{OCICompartmentID: testCompartmentID}
	var buf bytes.Buffer

	// Vault list fails
	mock.ExpectRunWithResult(vaultListCmd(), &exec.Result{
		Command: vaultListCmd(),
		Stderr:  "Service error\n",
	}, fmt.Errorf("exit status 1"))

	err := discoverVault(mock, &buf, func(string) bool { return false }, cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "listing vaults") {
		t.Errorf("expected 'listing vaults' in error, got: %v", err)
	}
}

func TestDiscoverVaultInvalidJSON(t *testing.T) {
	mock := exec.NewMockExecutor()
	cfg := &config.Config{OCICompartmentID: testCompartmentID}
	var buf bytes.Buffer

	// Vault list returns invalid JSON
	mock.ExpectRun(vaultListCmd(), "not-valid-json{{{", nil)

	err := discoverVault(mock, &buf, func(string) bool { return false }, cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "parsing vault list") {
		t.Errorf("expected 'parsing vault list' in error, got: %v", err)
	}
}

func TestSetupOCIConfigured(t *testing.T) {
	mock := exec.NewMockExecutor()
	tmp := t.TempDir()
	resolver := paths.NewWithHome(tmp, nil)

	// OCI region list succeeds
	mock.ExpectRun("oci iam region list --output json", `[{"name":"us-ashburn-1"}]`, nil)
	// Compartment list returns tenancy OCID
	mock.ExpectRun(
		`oci iam compartment list --query data[0]."compartment-id" --raw-output`,
		"ocid1.tenancy.oc1..aaaaexample\n", nil,
	)
	// gh auth status succeeds (needed for GitHub section)
	mock.ExpectRunWithResult("gh auth status", &exec.Result{
		Command: "gh auth status",
		Stdout:  "Logged in to github.com as user (token) - repo scope\n",
	}, nil)

	cfgPath := filepath.Join(tmp, "config.yaml")

	var buf bytes.Buffer
	err := Setup(SetupParams{
		Exec:        mock,
		Paths:       resolver,
		Stdout:      &buf,
		CfgFile:     cfgPath,
		ConfirmFunc: func(string) bool { return false },
	})
	if err != nil {
		t.Fatalf("Setup returned error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "OCI CLI configured and authenticated.") {
		t.Errorf("expected OCI configured message, got:\n%s", out)
	}
	if !strings.Contains(out, "Auto-detected tenancy: ocid1.tenancy.oc1..aaaaexample") {
		t.Errorf("expected tenancy auto-detection, got:\n%s", out)
	}
}

func TestSetupOCINotConfigured(t *testing.T) {
	mock := exec.NewMockExecutor()
	tmp := t.TempDir()
	resolver := paths.NewWithHome(tmp, nil)

	// OCI region list fails
	mock.ExpectRunWithResult("oci iam region list --output json", &exec.Result{
		Command: "oci iam region list --output json",
		Stderr:  "not configured\n",
	}, os.ErrNotExist)
	// gh auth status succeeds
	mock.ExpectRunWithResult("gh auth status", &exec.Result{
		Command: "gh auth status",
		Stdout:  "Logged in to github.com as user (token) - repo scope\n",
	}, nil)

	var buf bytes.Buffer
	err := Setup(SetupParams{
		Exec:        mock,
		Paths:       resolver,
		Stdout:      &buf,
		CfgFile:     filepath.Join(tmp, "config.yaml"),
		ConfirmFunc: func(string) bool { return false },
	})
	if err != nil {
		t.Fatalf("Setup returned error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "OCI CLI not configured.") {
		t.Errorf("expected OCI not configured message, got:\n%s", out)
	}
}

func TestSetupAgeKeyExists(t *testing.T) {
	mock := exec.NewMockExecutor()
	tmp := t.TempDir()

	// Create the age key file at the resolver's path
	resolver := paths.NewWithHome(tmp, nil)
	keyDir := filepath.Dir(resolver.AgeKeyFile())
	if err := os.MkdirAll(keyDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(resolver.AgeKeyFile(), []byte("AGE-SECRET-KEY-1TEST"), 0o600); err != nil {
		t.Fatal(err)
	}

	// OCI region list fails (skip OCI section)
	mock.ExpectRunWithResult("oci iam region list --output json", &exec.Result{
		Command: "oci iam region list --output json",
		Stderr:  "not configured\n",
	}, os.ErrNotExist)
	// gh auth status succeeds
	mock.ExpectRunWithResult("gh auth status", &exec.Result{
		Command: "gh auth status",
		Stdout:  "Logged in to github.com as user (token) - repo scope\n",
	}, nil)

	var buf bytes.Buffer
	err := Setup(SetupParams{
		Exec:        mock,
		Paths:       resolver,
		Stdout:      &buf,
		CfgFile:     filepath.Join(tmp, "config.yaml"),
		ConfirmFunc: func(string) bool { return false },
	})
	if err != nil {
		t.Fatalf("Setup returned error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Age key found at") {
		t.Errorf("expected 'Age key found' message, got:\n%s", out)
	}
}

func TestSetupAgeKeyMissing(t *testing.T) {
	// Clear SOPS_AGE_KEY_FILE so the resolver uses the tmp-based path
	// instead of a real key file on disk.
	t.Setenv("SOPS_AGE_KEY_FILE", "")

	mock := exec.NewMockExecutor()
	tmp := t.TempDir()
	resolver := paths.NewWithHome(tmp, nil)

	// OCI region list fails (skip OCI section)
	mock.ExpectRunWithResult("oci iam region list --output json", &exec.Result{
		Command: "oci iam region list --output json",
		Stderr:  "not configured\n",
	}, os.ErrNotExist)
	// gh auth status succeeds
	mock.ExpectRunWithResult("gh auth status", &exec.Result{
		Command: "gh auth status",
		Stdout:  "Logged in to github.com as user (token) - repo scope\n",
	}, nil)

	var buf bytes.Buffer
	err := Setup(SetupParams{
		Exec:        mock,
		Paths:       resolver,
		Stdout:      &buf,
		CfgFile:     filepath.Join(tmp, "config.yaml"),
		ConfirmFunc: func(string) bool { return false },
	})
	if err != nil {
		t.Fatalf("Setup returned error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "No age key") {
		t.Errorf("expected 'No age key' message, got:\n%s", out)
	}
}

func TestSetupSavesConfig(t *testing.T) {
	mock := exec.NewMockExecutor()
	tmp := t.TempDir()
	resolver := paths.NewWithHome(tmp, nil)

	// OCI region list succeeds -- this triggers compartment auto-discovery
	// which sets configChanged = true when a tenancy is found.
	mock.ExpectRun("oci iam region list --output json", `[{"name":"us-ashburn-1"}]`, nil)
	mock.ExpectRun(
		`oci iam compartment list --query data[0]."compartment-id" --raw-output`,
		"ocid1.tenancy.oc1..aaaaexample\n", nil,
	)
	// gh auth status succeeds
	mock.ExpectRunWithResult("gh auth status", &exec.Result{
		Command: "gh auth status",
		Stdout:  "Logged in to github.com as user (token) - repo scope\n",
	}, nil)

	cfgPath := filepath.Join(tmp, "config.yaml")

	var buf bytes.Buffer
	err := Setup(SetupParams{
		Exec:        mock,
		Paths:       resolver,
		Stdout:      &buf,
		CfgFile:     cfgPath,
		ConfirmFunc: func(string) bool { return false },
	})
	if err != nil {
		t.Fatalf("Setup returned error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Config saved to") {
		t.Errorf("expected config saved message, got:\n%s", out)
	}

	// Verify the config file was actually written
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		t.Fatal("config file was not created")
	}

	// Verify content
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("failed to load saved config: %v", err)
	}
	if cfg.OCICompartmentID != "ocid1.tenancy.oc1..aaaaexample" {
		t.Errorf("expected compartment ID 'ocid1.tenancy.oc1..aaaaexample', got %q", cfg.OCICompartmentID)
	}
}

func TestSetupCloudflareSection(t *testing.T) {
	mock := exec.NewMockExecutor()
	tmp := t.TempDir()
	resolver := paths.NewWithHome(tmp, nil)

	// OCI region list fails (skip OCI section)
	mock.ExpectRunWithResult("oci iam region list --output json", &exec.Result{
		Command: "oci iam region list --output json",
		Stderr:  "not configured\n",
	}, os.ErrNotExist)
	// gh auth status succeeds
	mock.ExpectRunWithResult("gh auth status", &exec.Result{
		Command: "gh auth status",
		Stdout:  "Logged in to github.com as user (token) - repo scope\n",
	}, nil)

	var buf bytes.Buffer
	err := Setup(SetupParams{
		Exec:        mock,
		Paths:       resolver,
		Stdout:      &buf,
		CfgFile:     filepath.Join(tmp, "config.yaml"),
		ConfirmFunc: func(string) bool { return false },
	})
	if err != nil {
		t.Fatalf("Setup returned error: %v", err)
	}

	out := buf.String()
	// The Cloudflare section header should always appear
	if !strings.Contains(out, "--- Cloudflare ---") {
		t.Errorf("expected Cloudflare section header, got:\n%s", out)
	}
	// Setup complete should appear at the end
	if !strings.Contains(out, "Setup complete.") {
		t.Errorf("expected 'Setup complete.' in output, got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Additional Setup coverage tests
// ---------------------------------------------------------------------------

func TestSetupOCIConfiguredEmptyCompartment(t *testing.T) {
	// OCI is authenticated but compartment list returns empty string.
	// This exercises the inner branch where tenancy == "" so configChanged stays false.
	mock := exec.NewMockExecutor()
	tmp := t.TempDir()
	resolver := paths.NewWithHome(tmp, nil)

	mock.ExpectRun("oci iam region list --output json", `[{"name":"us-ashburn-1"}]`, nil)
	// Compartment list returns empty
	mock.ExpectRun(
		`oci iam compartment list --query data[0]."compartment-id" --raw-output`,
		"  \n", nil,
	)
	mock.ExpectRunWithResult("gh auth status", &exec.Result{
		Command: "gh auth status",
		Stdout:  "Logged in to github.com as user (token) - repo scope\n",
	}, nil)

	var buf bytes.Buffer
	err := Setup(SetupParams{
		Exec:        mock,
		Paths:       resolver,
		Stdout:      &buf,
		CfgFile:     filepath.Join(tmp, "config.yaml"),
		ConfirmFunc: func(string) bool { return false },
	})
	if err != nil {
		t.Fatalf("Setup returned error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "OCI CLI configured and authenticated.") {
		t.Errorf("expected OCI configured message, got:\n%s", out)
	}
	// Should NOT contain auto-detected tenancy since it was empty
	if strings.Contains(out, "Auto-detected tenancy:") {
		t.Error("should not auto-detect tenancy when compartment list returns empty")
	}
}

func TestSetupOCINotConfiguredConfirmSetup(t *testing.T) {
	// OCI not configured, user confirms yes, oci setup config runs but fails.
	mock := exec.NewMockExecutor()
	tmp := t.TempDir()
	resolver := paths.NewWithHome(tmp, nil)

	mock.ExpectRunWithResult("oci iam region list --output json", &exec.Result{
		Command: "oci iam region list --output json",
		Stderr:  "not configured\n",
	}, os.ErrNotExist)
	// oci setup config fails
	mock.ExpectRunWithResult("oci setup config", &exec.Result{
		Command: "oci setup config",
		Stderr:  "setup failed\n",
	}, fmt.Errorf("exit status 1"))
	mock.ExpectRunWithResult("gh auth status", &exec.Result{
		Command: "gh auth status",
		Stdout:  "Logged in to github.com as user (token) - repo scope\n",
	}, nil)

	var buf bytes.Buffer
	err := Setup(SetupParams{
		Exec:        mock,
		Paths:       resolver,
		Stdout:      &buf,
		CfgFile:     filepath.Join(tmp, "config.yaml"),
		ConfirmFunc: func(string) bool { return true }, // confirm all
	})
	if err != nil {
		t.Fatalf("Setup returned error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "OCI CLI not configured.") {
		t.Errorf("expected OCI not configured message, got:\n%s", out)
	}
	if !strings.Contains(out, "WARNING: oci setup config failed") {
		t.Errorf("expected oci setup config failure warning, got:\n%s", out)
	}
}

func TestSetupGitHubMissingRepoScope(t *testing.T) {
	// GitHub auth succeeds but output does not contain "repo" scope.
	mock := exec.NewMockExecutor()
	tmp := t.TempDir()
	resolver := paths.NewWithHome(tmp, nil)

	mock.ExpectRunWithResult("oci iam region list --output json", &exec.Result{
		Command: "oci iam region list --output json",
		Stderr:  "not configured\n",
	}, os.ErrNotExist)
	// GitHub auth output missing "repo" scope
	mock.ExpectRunWithResult("gh auth status", &exec.Result{
		Command: "gh auth status",
		Stdout:  "Logged in to github.com as user (token)\n",
	}, nil)

	var buf bytes.Buffer
	err := Setup(SetupParams{
		Exec:        mock,
		Paths:       resolver,
		Stdout:      &buf,
		CfgFile:     filepath.Join(tmp, "config.yaml"),
		ConfirmFunc: func(string) bool { return false },
	})
	if err != nil {
		t.Fatalf("Setup returned error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "'repo' scope may be missing") {
		t.Errorf("expected repo scope warning, got:\n%s", out)
	}
}

func TestSetupGitHubNotAuthenticated(t *testing.T) {
	// GitHub auth fails, user confirms login, login fails.
	mock := exec.NewMockExecutor()
	tmp := t.TempDir()
	resolver := paths.NewWithHome(tmp, nil)

	mock.ExpectRunWithResult("oci iam region list --output json", &exec.Result{
		Command: "oci iam region list --output json",
		Stderr:  "not configured\n",
	}, os.ErrNotExist)
	// GitHub not authenticated
	mock.ExpectRunWithResult("gh auth status", &exec.Result{
		Command: "gh auth status",
		Stderr:  "not logged in\n",
	}, fmt.Errorf("exit status 1"))
	// gh auth login fails
	mock.ExpectRunWithResult("gh auth login", &exec.Result{
		Command: "gh auth login",
		Stderr:  "login failed\n",
	}, fmt.Errorf("exit status 1"))

	var buf bytes.Buffer
	err := Setup(SetupParams{
		Exec:        mock,
		Paths:       resolver,
		Stdout:      &buf,
		CfgFile:     filepath.Join(tmp, "config.yaml"),
		ConfirmFunc: func(string) bool { return true },
	})
	if err != nil {
		t.Fatalf("Setup returned error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "GitHub CLI not authenticated.") {
		t.Errorf("expected GitHub not authenticated message, got:\n%s", out)
	}
	if !strings.Contains(out, "WARNING: gh auth login failed") {
		t.Errorf("expected gh auth login failure warning, got:\n%s", out)
	}
}

func TestSetupGitHubNotAuthenticatedDecline(t *testing.T) {
	// GitHub auth fails, user declines login.
	mock := exec.NewMockExecutor()
	tmp := t.TempDir()
	resolver := paths.NewWithHome(tmp, nil)

	mock.ExpectRunWithResult("oci iam region list --output json", &exec.Result{
		Command: "oci iam region list --output json",
		Stderr:  "not configured\n",
	}, os.ErrNotExist)
	mock.ExpectRunWithResult("gh auth status", &exec.Result{
		Command: "gh auth status",
		Stderr:  "not logged in\n",
	}, fmt.Errorf("exit status 1"))

	var buf bytes.Buffer
	err := Setup(SetupParams{
		Exec:        mock,
		Paths:       resolver,
		Stdout:      &buf,
		CfgFile:     filepath.Join(tmp, "config.yaml"),
		ConfirmFunc: func(string) bool { return false },
	})
	if err != nil {
		t.Fatalf("Setup returned error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "GitHub CLI not authenticated.") {
		t.Errorf("expected GitHub not authenticated message, got:\n%s", out)
	}
}

func TestSetupVaultKeyAlreadyConfigured(t *testing.T) {
	// Vault key is already in config, should just display it and skip discovery.
	mock := exec.NewMockExecutor()
	tmp := t.TempDir()
	resolver := paths.NewWithHome(tmp, nil)

	mock.ExpectRunWithResult("oci iam region list --output json", &exec.Result{
		Command: "oci iam region list --output json",
		Stderr:  "not configured\n",
	}, os.ErrNotExist)
	mock.ExpectRunWithResult("gh auth status", &exec.Result{
		Command: "gh auth status",
		Stdout:  "Logged in to github.com as user (token) - repo scope\n",
	}, nil)

	cfgPath := filepath.Join(tmp, "config.yaml")
	// Pre-write a config with vault key
	cfg := &config.Config{
		OCIVaultKeyOCID: "ocid1.key.oc1..aaaa1234567890123456",
	}
	if err := config.Save(cfg, cfgPath); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	err := Setup(SetupParams{
		Exec:        mock,
		Paths:       resolver,
		Stdout:      &buf,
		CfgFile:     cfgPath,
		ConfirmFunc: func(string) bool { return false },
	})
	if err != nil {
		t.Fatalf("Setup returned error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Vault key configured: ...") {
		t.Errorf("expected vault key configured message, got:\n%s", out)
	}
}

func TestSetupVaultDiscoveryTriggered(t *testing.T) {
	// OCI configured + compartment set + no vault key -> triggers discoverVault.
	mock := exec.NewMockExecutor()
	tmp := t.TempDir()
	resolver := paths.NewWithHome(tmp, nil)

	mock.ExpectRun("oci iam region list --output json", `[{"name":"us-ashburn-1"}]`, nil)
	// Compartment list returns tenancy
	mock.ExpectRun(
		`oci iam compartment list --query data[0]."compartment-id" --raw-output`,
		testCompartmentID+"\n", nil,
	)
	mock.ExpectRunWithResult("gh auth status", &exec.Result{
		Command: "gh auth status",
		Stdout:  "Logged in to github.com as user (token) - repo scope\n",
	}, nil)

	// discoverVault: vault list returns one vault with one key
	mock.ExpectRun(
		fmt.Sprintf(`oci kms management vault list --all --compartment-id %s --output json --query data[?\"lifecycle-state\"=='ACTIVE']`, testCompartmentID),
		oneVaultJSON(), nil)
	mock.ExpectRun(
		fmt.Sprintf(`oci kms management key list --all --compartment-id %s --endpoint %s --output json --query data[?\"lifecycle-state\"=='ENABLED']`, testCompartmentID, testVaultEndpoint),
		oneKeyJSON(testKeyID, "my-key", "AES"), nil)

	cfgPath := filepath.Join(tmp, "config.yaml")

	var buf bytes.Buffer
	err := Setup(SetupParams{
		Exec:        mock,
		Paths:       resolver,
		Stdout:      &buf,
		CfgFile:     cfgPath,
		ConfirmFunc: func(string) bool { return false },
	})
	if err != nil {
		t.Fatalf("Setup returned error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Discovering OCI vaults...") {
		t.Errorf("expected vault discovery message, got:\n%s", out)
	}
	if !strings.Contains(out, "Config saved to") {
		t.Errorf("expected config saved, got:\n%s", out)
	}
}

func TestSetupVaultDiscoveryFails(t *testing.T) {
	// OCI configured + compartment set + discoverVault returns error.
	mock := exec.NewMockExecutor()
	tmp := t.TempDir()
	resolver := paths.NewWithHome(tmp, nil)

	mock.ExpectRun("oci iam region list --output json", `[{"name":"us-ashburn-1"}]`, nil)
	mock.ExpectRun(
		`oci iam compartment list --query data[0]."compartment-id" --raw-output`,
		testCompartmentID+"\n", nil,
	)
	mock.ExpectRunWithResult("gh auth status", &exec.Result{
		Command: "gh auth status",
		Stdout:  "Logged in to github.com as user (token) - repo scope\n",
	}, nil)

	// discoverVault: vault list fails
	mock.ExpectRunWithResult(
		fmt.Sprintf(`oci kms management vault list --all --compartment-id %s --output json --query data[?\"lifecycle-state\"=='ACTIVE']`, testCompartmentID),
		&exec.Result{Stderr: "error"},
		fmt.Errorf("service error"),
	)

	cfgPath := filepath.Join(tmp, "config.yaml")

	var buf bytes.Buffer
	err := Setup(SetupParams{
		Exec:        mock,
		Paths:       resolver,
		Stdout:      &buf,
		CfgFile:     cfgPath,
		ConfirmFunc: func(string) bool { return false },
	})
	if err != nil {
		t.Fatalf("Setup returned error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Could not auto-discover vault") {
		t.Errorf("expected vault discovery failure message, got:\n%s", out)
	}
	if !strings.Contains(out, "To provision manually:") {
		t.Errorf("expected manual provision instructions, got:\n%s", out)
	}
}

func TestSetupOCICompartmentListFails(t *testing.T) {
	// OCI is authenticated but compartment list command fails.
	mock := exec.NewMockExecutor()
	tmp := t.TempDir()
	resolver := paths.NewWithHome(tmp, nil)

	mock.ExpectRun("oci iam region list --output json", `[{"name":"us-ashburn-1"}]`, nil)
	// Compartment list fails
	mock.ExpectRunWithResult(
		`oci iam compartment list --query data[0]."compartment-id" --raw-output`,
		&exec.Result{Stderr: "error"},
		fmt.Errorf("exit status 1"),
	)
	mock.ExpectRunWithResult("gh auth status", &exec.Result{
		Command: "gh auth status",
		Stdout:  "Logged in to github.com as user (token) - repo scope\n",
	}, nil)

	var buf bytes.Buffer
	err := Setup(SetupParams{
		Exec:        mock,
		Paths:       resolver,
		Stdout:      &buf,
		CfgFile:     filepath.Join(tmp, "config.yaml"),
		ConfirmFunc: func(string) bool { return false },
	})
	if err != nil {
		t.Fatalf("Setup returned error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "OCI CLI configured and authenticated.") {
		t.Errorf("expected OCI configured message, got:\n%s", out)
	}
	// Should not contain auto-detected tenancy since compartment list failed
	if strings.Contains(out, "Auto-detected tenancy:") {
		t.Error("should not auto-detect tenancy when compartment list fails")
	}
}

func TestSetupNoConfigChange(t *testing.T) {
	// No config changes should mean no "Config saved" message.
	// Pre-populate the config with a CloudflareAccountID so that
	// cloudflare.ResolveAccountID() (which may succeed on this machine)
	// does not trigger configChanged.
	mock := exec.NewMockExecutor()
	tmp := t.TempDir()
	resolver := paths.NewWithHome(tmp, nil)

	cfgPath := filepath.Join(tmp, "config.yaml")
	presetCfg := &config.Config{
		CloudflareAccountID: "already-set-account-id",
	}
	if err := config.Save(presetCfg, cfgPath); err != nil {
		t.Fatal(err)
	}

	mock.ExpectRunWithResult("oci iam region list --output json", &exec.Result{
		Command: "oci iam region list --output json",
		Stderr:  "not configured\n",
	}, os.ErrNotExist)
	mock.ExpectRunWithResult("gh auth status", &exec.Result{
		Command: "gh auth status",
		Stdout:  "Logged in to github.com as user (token) - repo scope\n",
	}, nil)

	var buf bytes.Buffer
	err := Setup(SetupParams{
		Exec:        mock,
		Paths:       resolver,
		Stdout:      &buf,
		CfgFile:     cfgPath,
		ConfirmFunc: func(string) bool { return false },
	})
	if err != nil {
		t.Fatalf("Setup returned error: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, "Config saved to") {
		t.Error("should not save config when nothing changed")
	}
	if !strings.Contains(out, "Setup complete.") {
		t.Errorf("expected Setup complete message, got:\n%s", out)
	}
}

func TestDiscoverVaultKeyListError(t *testing.T) {
	mock := exec.NewMockExecutor()
	cfg := &config.Config{OCICompartmentID: testCompartmentID}
	var buf bytes.Buffer

	// Vault list succeeds
	mock.ExpectRun(vaultListCmd(),
		oneVaultJSON(), nil)
	// Key list fails
	mock.ExpectRunWithResult(keyListCmd(), &exec.Result{
		Command: keyListCmd(),
		Stderr:  "key list error\n",
	}, fmt.Errorf("exit status 1"))

	err := discoverVault(mock, &buf, func(string) bool { return false }, cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "listing keys") {
		t.Errorf("expected 'listing keys' in error, got: %v", err)
	}
}

func TestDiscoverVaultKeyListInvalidJSON(t *testing.T) {
	mock := exec.NewMockExecutor()
	cfg := &config.Config{OCICompartmentID: testCompartmentID}
	var buf bytes.Buffer

	mock.ExpectRun(vaultListCmd(),
		oneVaultJSON(), nil)
	mock.ExpectRun(keyListCmd(), "invalid-json{{{", nil)

	err := discoverVault(mock, &buf, func(string) bool { return false }, cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "parsing key list") {
		t.Errorf("expected 'parsing key list' in error, got: %v", err)
	}
}

func TestDiscoverVaultCreateVaultFails(t *testing.T) {
	mock := exec.NewMockExecutor()
	cfg := &config.Config{OCICompartmentID: testCompartmentID}
	var buf bytes.Buffer

	mock.ExpectRun(vaultListCmd(), "[]", nil)
	mock.ExpectRunWithResult(vaultCreateCmd(testCompartmentID), &exec.Result{
		Stderr: "create error",
	}, fmt.Errorf("exit status 1"))

	err := discoverVault(mock, &buf, func(string) bool { return true }, cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "creating vault") {
		t.Errorf("expected 'creating vault' in error, got: %v", err)
	}
}

func TestDiscoverVaultCreateVaultInvalidJSON(t *testing.T) {
	mock := exec.NewMockExecutor()
	cfg := &config.Config{OCICompartmentID: testCompartmentID}
	var buf bytes.Buffer

	mock.ExpectRun(vaultListCmd(), "[]", nil)
	mock.ExpectRun(vaultCreateCmd(testCompartmentID), "not-json{{{", nil)

	err := discoverVault(mock, &buf, func(string) bool { return true }, cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "parsing created vault") {
		t.Errorf("expected 'parsing created vault' in error, got: %v", err)
	}
}

func TestDiscoverVaultCreateKeyFails(t *testing.T) {
	mock := exec.NewMockExecutor()
	cfg := &config.Config{OCICompartmentID: testCompartmentID}
	var buf bytes.Buffer

	mock.ExpectRun(vaultListCmd(),
		oneVaultJSON(), nil)
	mock.ExpectRun(keyListCmd(), "[]", nil)
	mock.ExpectRunWithResult(keyCreateCmd(testCompartmentID, testVaultEndpoint), &exec.Result{
		Stderr: "create error",
	}, fmt.Errorf("exit status 1"))

	err := discoverVault(mock, &buf, func(string) bool { return true }, cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "creating key") {
		t.Errorf("expected 'creating key' in error, got: %v", err)
	}
}

func TestDiscoverVaultCreateKeyInvalidJSON(t *testing.T) {
	mock := exec.NewMockExecutor()
	cfg := &config.Config{OCICompartmentID: testCompartmentID}
	var buf bytes.Buffer

	mock.ExpectRun(vaultListCmd(),
		oneVaultJSON(), nil)
	mock.ExpectRun(keyListCmd(), "[]", nil)
	mock.ExpectRun(keyCreateCmd(testCompartmentID, testVaultEndpoint), "not-json{{{", nil)

	err := discoverVault(mock, &buf, func(string) bool { return true }, cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "parsing created key") {
		t.Errorf("expected 'parsing created key' in error, got: %v", err)
	}
}

func TestDiscoverVaultNoKeysDecline(t *testing.T) {
	mock := exec.NewMockExecutor()
	cfg := &config.Config{OCICompartmentID: testCompartmentID}
	var buf bytes.Buffer

	mock.ExpectRun(vaultListCmd(),
		oneVaultJSON(), nil)
	mock.ExpectRun(keyListCmd(), "[]", nil)

	// Decline key creation
	err := discoverVault(mock, &buf, func(string) bool { return false }, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.OCIVaultKeyOCID != "" {
		t.Errorf("expected empty key OCID when declined, got %q", cfg.OCIVaultKeyOCID)
	}
}
