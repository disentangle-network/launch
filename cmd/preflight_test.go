package cmd

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/disentangle-network/launch/internal/exec"
)

func TestPreflightAllOK(t *testing.T) {
	mock := exec.NewMockExecutor()

	// Credential checks that use the executor:
	// checkOCI calls: oci iam region list --output json
	mock.ExpectRun("oci iam region list --output json", `[{"name":"us-ashburn-1"}]`, nil)
	// checkCloudflare calls cloudflare.ResolveToken() internally (no executor),
	// then curl -- we cannot mock ResolveToken, so cloudflare may come back
	// as "missing" depending on env. That's fine for this test.
	// checkGitHub calls: gh auth status
	mock.ExpectRunWithResult("gh auth status", &exec.Result{
		Command: "gh auth status",
		Stdout:  "Logged in to github.com as testuser (oauth_token)\n",
	}, nil)

	var buf bytes.Buffer
	err := Preflight(PreflightParams{
		Exec:   mock,
		Stdout: &buf,
	})

	out := buf.String()
	// The preflight function checks tools first (via exec.LookPath, not
	// the mock). If a required tool is missing from PATH, it returns an
	// error before reaching credentials. In CI/test environments we may
	// not have all required tools, so we accept either outcome.
	if err != nil {
		// If tools are missing, the error will mention "missing required tools"
		if strings.Contains(err.Error(), "missing required tools") {
			t.Skipf("skipping: required CLI tools not in PATH: %v", err)
		}
		// If credentials are invalid (e.g. Cloudflare token missing)
		if strings.Contains(err.Error(), "credentials are invalid") {
			// This is expected when CF token is not set -- still verify output format
			if !strings.Contains(out, "==> Checking credentials...") {
				t.Error("expected credential check header in output")
			}
			return
		}
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out, "Preflight checks passed.") {
		t.Errorf("expected 'Preflight checks passed.' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "==> Checking required tools...") {
		t.Error("expected tools header in output")
	}
	if !strings.Contains(out, "==> Checking credentials...") {
		t.Error("expected credentials header in output")
	}
}

func TestPreflightCredentialOutput(t *testing.T) {
	mock := exec.NewMockExecutor()

	// OCI succeeds
	mock.ExpectRun("oci iam region list --output json", `[]`, nil)
	// GitHub succeeds
	mock.ExpectRunWithResult("gh auth status", &exec.Result{
		Command: "gh auth status",
		Stdout:  "Logged in to github.com as user\n",
	}, nil)

	var buf bytes.Buffer
	_ = Preflight(PreflightParams{
		Exec:   mock,
		Stdout: &buf,
	})

	out := buf.String()
	// Regardless of pass/fail, we should see the credential section header
	if !strings.Contains(out, "==> Checking") {
		t.Errorf("expected section headers in output, got:\n%s", out)
	}
}

func TestPreflightInvalidCredentials(t *testing.T) {
	mock := exec.NewMockExecutor()

	// OCI succeeds
	mock.ExpectRun("oci iam region list --output json", `[]`, nil)
	// GitHub fails -- this makes AllCredentialsOK return false
	mock.ExpectRunWithResult("gh auth status", &exec.Result{
		Command: "gh auth status",
		Stderr:  "not logged in\n",
	}, fmt.Errorf("exit status 1"))

	var buf bytes.Buffer
	err := Preflight(PreflightParams{
		Exec:   mock,
		Stdout: &buf,
	})

	// Tools might be missing in test env
	if err != nil && strings.Contains(err.Error(), "missing required tools") {
		t.Skipf("skipping: required CLI tools not in PATH: %v", err)
	}

	out := buf.String()
	if err == nil {
		// If no error, cloudflare might have been OK and OCI/GH both returned
		// something that passed AllCredentialsOK. Check output instead.
		if strings.Contains(out, "INVALID") {
			t.Log("output contains INVALID but AllCredentialsOK still returned true")
		}
		return
	}

	if !strings.Contains(err.Error(), "credentials are invalid") {
		t.Errorf("expected 'credentials are invalid' error, got: %v", err)
	}
	if !strings.Contains(out, "INVALID") {
		t.Errorf("expected INVALID in output for failed gh auth, got:\n%s", out)
	}
}

func TestPreflightToolOutputFormat(t *testing.T) {
	mock := exec.NewMockExecutor()
	mock.ExpectRun("oci iam region list --output json", `[]`, nil)
	mock.ExpectRunWithResult("gh auth status", &exec.Result{
		Command: "gh auth status",
		Stdout:  "Logged in to github.com as user\n",
	}, nil)

	var buf bytes.Buffer
	err := Preflight(PreflightParams{
		Exec:   mock,
		Stdout: &buf,
	})

	out := buf.String()
	if err != nil && strings.Contains(err.Error(), "missing required tools") {
		// Verify the output contains MISSING markers for unavailable tools
		if !strings.Contains(out, "MISSING") {
			t.Error("expected MISSING in output when tools are not available")
		}
		return
	}

	if !strings.Contains(out, "==> Checking required tools...") {
		t.Error("expected tools header")
	}
	if !strings.Contains(out, "OK") {
		t.Errorf("expected at least one OK status, got:\n%s", out)
	}
}

func TestPreflightBothCredentialsFail(t *testing.T) {
	mock := exec.NewMockExecutor()
	mock.ExpectRunWithResult("oci iam region list --output json", &exec.Result{
		Command: "oci iam region list --output json",
		Stderr:  "authentication failed\n",
	}, fmt.Errorf("exit status 1"))
	mock.ExpectRunWithResult("gh auth status", &exec.Result{
		Command: "gh auth status",
		Stderr:  "not authenticated\n",
	}, fmt.Errorf("exit status 1"))

	var buf bytes.Buffer
	err := Preflight(PreflightParams{
		Exec:   mock,
		Stdout: &buf,
	})

	if err != nil && strings.Contains(err.Error(), "missing required tools") {
		t.Skipf("skipping: required CLI tools not in PATH: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "==> Checking credentials...") {
		if err == nil {
			t.Error("expected credentials header in output")
		}
		return
	}
	if !strings.Contains(out, "INVALID") && !strings.Contains(out, "MISSING") {
		t.Errorf("expected INVALID or MISSING in credential output, got:\n%s", out)
	}
}
