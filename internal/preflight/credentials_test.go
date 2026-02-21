package preflight

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/disentangle-network/launch/internal/exec"
)

// ---------------------------------------------------------------------------
// checkOCI
// ---------------------------------------------------------------------------

func TestCheckOCIOK(t *testing.T) {
	// Set HOME to a temp dir that contains .oci/config so the file-check passes.
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	ociDir := filepath.Join(tmp, ".oci")
	if err := os.MkdirAll(ociDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ociDir, "config"), []byte("[DEFAULT]\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	m := exec.NewMockExecutor()
	m.ExpectRun("oci iam region list --output json", `{"data":[]}`, nil)

	result := checkOCI(m)
	if result.Status != "ok" {
		t.Errorf("expected status ok, got %q (detail: %s)", result.Status, result.Detail)
	}
	m.AssertCalled(t, "oci iam region list --output json")
}

func TestCheckOCIMissingConfig(t *testing.T) {
	// Point HOME at an empty temp dir -- no .oci/config exists.
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	m := exec.NewMockExecutor()
	result := checkOCI(m)

	if result.Status != "missing" {
		t.Errorf("expected status missing, got %q (detail: %s)", result.Status, result.Detail)
	}
	// No commands should have been invoked since the config file is absent.
	m.AssertCallCount(t, 0)
}

func TestCheckOCIInvalid(t *testing.T) {
	// Create .oci/config so the file check passes, but the API call fails.
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	ociDir := filepath.Join(tmp, ".oci")
	if err := os.MkdirAll(ociDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ociDir, "config"), []byte("[DEFAULT]\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	m := exec.NewMockExecutor()
	m.ExpectRunWithResult(
		"oci iam region list --output json",
		&exec.Result{
			Command: "oci iam region list --output json",
			Stderr:  "ServiceError: 401 -- NotAuthenticated",
		},
		fmt.Errorf("exit status 1"),
	)

	result := checkOCI(m)
	if result.Status != "invalid" {
		t.Errorf("expected status invalid, got %q (detail: %s)", result.Status, result.Detail)
	}
}

// ---------------------------------------------------------------------------
// checkCloudflare
// ---------------------------------------------------------------------------

func TestCheckCloudflareOK(t *testing.T) {
	// Set the env var so ResolveToken returns immediately with this token.
	testToken := "cf-test-token-abc123"
	t.Setenv("CLOUDFLARE_API_TOKEN", testToken)
	// Clear other cloudflare-related env vars.
	t.Setenv("OP_CLOUDFLARE_REF", "")

	// Mock the curl verification call. checkCloudflare tries the /verify endpoint first.
	verifyCmd := "curl -s -o /dev/null -w %{http_code} -H Authorization: Bearer " + testToken + " https://api.cloudflare.com/client/v4/user/tokens/verify"
	m := exec.NewMockExecutor()
	m.ExpectRun(verifyCmd, "200", nil)

	result := checkCloudflare(m)
	if result.Status != "ok" {
		t.Errorf("expected status ok, got %q (detail: %s)", result.Status, result.Detail)
	}
	if result.Detail == "" {
		t.Error("expected non-empty detail")
	}
}

func TestCheckCloudflareMissing(t *testing.T) {
	// Clear all cloudflare-related env vars.
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	t.Setenv("OP_CLOUDFLARE_REF", "")
	// Ensure wrangler and op are not in PATH (they shouldn't be in CI,
	// but we strip PATH to be safe).
	t.Setenv("PATH", t.TempDir())

	m := exec.NewMockExecutor()
	result := checkCloudflare(m)

	if result.Status != "missing" {
		t.Errorf("expected status missing, got %q (detail: %s)", result.Status, result.Detail)
	}
	// No curl calls should have been made since there's no token.
	m.AssertCallCount(t, 0)
}

// ---------------------------------------------------------------------------
// checkGitHub
// ---------------------------------------------------------------------------

func TestCheckGitHubOK(t *testing.T) {
	m := exec.NewMockExecutor()
	m.ExpectRunWithResult(
		"gh auth status",
		&exec.Result{
			Command: "gh auth status",
			Stdout:  "github.com\n  Logged in to github.com account testuser (keyring)\n",
		},
		nil,
	)

	result := checkGitHub(m)
	if result.Status != "ok" {
		t.Errorf("expected status ok, got %q (detail: %s)", result.Status, result.Detail)
	}
	if result.Detail == "" || !contains(result.Detail, "Logged in") {
		t.Errorf("expected detail to contain 'Logged in', got %q", result.Detail)
	}
}

func TestCheckGitHubInvalid(t *testing.T) {
	m := exec.NewMockExecutor()
	m.ExpectRunWithResult(
		"gh auth status",
		&exec.Result{
			Command: "gh auth status",
			Stderr:  "not logged in",
		},
		fmt.Errorf("exit status 1"),
	)

	result := checkGitHub(m)
	if result.Status != "invalid" {
		t.Errorf("expected status invalid, got %q (detail: %s)", result.Status, result.Detail)
	}
}

// ---------------------------------------------------------------------------
// checkAgeKey
// ---------------------------------------------------------------------------

func TestCheckAgeKeyExists(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("SOPS_AGE_KEY_FILE", "")

	keyDir := filepath.Join(tmp, ".config", "sops", "age")
	if err := os.MkdirAll(keyDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(keyDir, "keys.txt"), []byte("AGE-SECRET-KEY-1FAKE\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	result := checkAgeKey()
	if result.Status != "ok" {
		t.Errorf("expected status ok, got %q (detail: %s)", result.Status, result.Detail)
	}
}

func TestCheckAgeKeyMissing(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("SOPS_AGE_KEY_FILE", "")

	result := checkAgeKey()
	if result.Status != "missing" {
		t.Errorf("expected status missing, got %q (detail: %s)", result.Status, result.Detail)
	}
}

func TestCheckAgeKeyEnvOverride(t *testing.T) {
	tmp := t.TempDir()
	keyFile := filepath.Join(tmp, "custom-age-key.txt")
	if err := os.WriteFile(keyFile, []byte("AGE-SECRET-KEY-1CUSTOM\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SOPS_AGE_KEY_FILE", keyFile)

	result := checkAgeKey()
	if result.Status != "ok" {
		t.Errorf("expected status ok, got %q (detail: %s)", result.Status, result.Detail)
	}
	if !contains(result.Detail, keyFile) {
		t.Errorf("expected detail to contain custom path %q, got %q", keyFile, result.Detail)
	}
}

// ---------------------------------------------------------------------------
// AllCredentialsOK
// ---------------------------------------------------------------------------

func TestAllCredentialsOKAllOK(t *testing.T) {
	checks := []CredentialCheck{
		{Name: "A", Status: "ok"},
		{Name: "B", Status: "ok"},
		{Name: "C", Status: "ok"},
	}
	if !AllCredentialsOK(checks) {
		t.Error("expected AllCredentialsOK to return true when all are ok")
	}
}

func TestAllCredentialsOKWithMissing(t *testing.T) {
	checks := []CredentialCheck{
		{Name: "A", Status: "ok"},
		{Name: "B", Status: "missing"},
		{Name: "C", Status: "ok"},
	}
	if !AllCredentialsOK(checks) {
		t.Error("expected AllCredentialsOK to return true when some are missing (missing is tolerated)")
	}
}

func TestAllCredentialsOKWithInvalid(t *testing.T) {
	checks := []CredentialCheck{
		{Name: "A", Status: "ok"},
		{Name: "B", Status: "invalid"},
		{Name: "C", Status: "ok"},
	}
	if AllCredentialsOK(checks) {
		t.Error("expected AllCredentialsOK to return false when one is invalid")
	}
}

// ---------------------------------------------------------------------------
// CheckCredentials (integration-level: verifies 4 results returned)
// ---------------------------------------------------------------------------

func TestCheckCredentialsReturns4(t *testing.T) {
	// Set up HOME with .oci/config so OCI doesn't short-circuit before the
	// executor call. Create the age key so that check passes too.
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("SOPS_AGE_KEY_FILE", "")
	// Clear cloudflare to get a predictable "missing" path.
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	t.Setenv("OP_CLOUDFLARE_REF", "")
	t.Setenv("PATH", t.TempDir()) // ensure wrangler/op not found

	ociDir := filepath.Join(tmp, ".oci")
	if err := os.MkdirAll(ociDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ociDir, "config"), []byte("[DEFAULT]\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	m := exec.NewMockExecutor()
	// OCI succeeds
	m.ExpectRun("oci iam region list --output json", `{"data":[]}`, nil)
	// GitHub succeeds
	m.ExpectRun("gh auth status", "Logged in to github.com", nil)

	results := CheckCredentials(m)
	if len(results) != 4 {
		t.Fatalf("expected 4 credential check results, got %d", len(results))
	}

	// Verify expected names in order.
	expectedNames := []string{"OCI API Key", "Cloudflare API Token", "GitHub CLI", "SOPS Age Key"}
	for i, name := range expectedNames {
		if results[i].Name != name {
			t.Errorf("result[%d]: expected name %q, got %q", i, name, results[i].Name)
		}
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
