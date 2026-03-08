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

// mockToken returns a TokenResolverFunc that returns a fixed token.
func mockToken(token, source string) func() (string, string, error) {
	return func() (string, string, error) {
		return token, source, nil
	}
}

// newTestInfraParams creates InfraParams wired to a mock executor and temp dir.
// The temp dir is pre-populated with environments/<env>/ so resolveInfraParams succeeds.
func newTestInfraParams(t *testing.T) (InfraParams, *exec.MockExecutor, string) {
	t.Helper()

	tmp := t.TempDir()
	envDir := filepath.Join(tmp, "environments", "dev")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		t.Fatalf("creating env dir: %v", err)
	}

	mock := exec.NewMockExecutor()
	cfg := &config.Config{}
	pr := paths.NewWithHome(tmp, cfg)

	var buf bytes.Buffer
	p := InfraParams{
		Exec:              mock,
		Paths:             pr,
		Stdout:            &buf,
		Env:               "dev",
		Dir:               tmp,
		DryRun:            false,
		ConfirmFunc:       func(string) bool { return true },
		TokenResolverFunc: mockToken("test-cf-token", "mock"),
	}
	return p, mock, tmp
}

func TestInfraInit(t *testing.T) {
	p, mock, _ := newTestInfraParams(t)
	mock.ExpectRun("tofu init -upgrade", "", nil)

	if err := InfraInit(p); err != nil {
		t.Fatalf("InfraInit: %v", err)
	}

	mock.AssertCalled(t, "tofu init -upgrade")
	mock.AssertCallCount(t, 1)
}

func TestInfraPlan(t *testing.T) {
	p, mock, _ := newTestInfraParams(t)
	mock.ExpectRun("tofu init -upgrade", "", nil)
	mock.ExpectRun("tofu plan -var-file=terraform.tfvars -out=tfplan", "", nil)

	if err := InfraPlan(p); err != nil {
		t.Fatalf("InfraPlan: %v", err)
	}

	mock.AssertCalled(t, "tofu init -upgrade")
	mock.AssertCalled(t, "tofu plan -var-file=terraform.tfvars -out=tfplan")
	mock.AssertCallCount(t, 2)
}

func TestInfraApply(t *testing.T) {
	p, mock, tmp := newTestInfraParams(t)

	// Create a tfplan file so apply does not run plan first.
	envDir := filepath.Join(tmp, "environments", "dev")
	planFile := filepath.Join(envDir, "tfplan")
	if err := os.WriteFile(planFile, []byte("plan"), 0o644); err != nil {
		t.Fatalf("writing tfplan: %v", err)
	}

	mock.ExpectRun("tofu apply tfplan", "", nil)
	mock.ExpectRun("tofu output -json", "", nil)

	p.ConfirmFunc = func(string) bool { return true }

	if err := InfraApply(p); err != nil {
		t.Fatalf("InfraApply: %v", err)
	}

	mock.AssertCalled(t, "tofu apply tfplan")
	mock.AssertCalled(t, "tofu output -json")
	mock.AssertCallCount(t, 2)
}

func TestInfraApplyNoConfirm(t *testing.T) {
	p, mock, tmp := newTestInfraParams(t)

	// Create a tfplan file.
	envDir := filepath.Join(tmp, "environments", "dev")
	planFile := filepath.Join(envDir, "tfplan")
	if err := os.WriteFile(planFile, []byte("plan"), 0o644); err != nil {
		t.Fatalf("writing tfplan: %v", err)
	}

	p.ConfirmFunc = func(string) bool { return false }

	if err := InfraApply(p); err != nil {
		t.Fatalf("InfraApply: %v", err)
	}

	// apply should NOT have been called
	mock.AssertCallCount(t, 0)

	out := p.Stdout.(*bytes.Buffer).String() //nolint:errcheck // test type assertion
	if !strings.Contains(out, "Cancelled") {
		t.Errorf("expected output to contain 'Cancelled', got: %s", out)
	}
}

func TestInfraApplyNoPlan(t *testing.T) {
	p, mock, _ := newTestInfraParams(t)

	// No tfplan file exists, so apply should run plan first.
	mock.ExpectRun("tofu plan -var-file=terraform.tfvars -out=tfplan", "", nil)
	mock.ExpectRun("tofu apply tfplan", "", nil)
	mock.ExpectRun("tofu output -json", "", nil)

	p.ConfirmFunc = func(string) bool { return true }

	if err := InfraApply(p); err != nil {
		t.Fatalf("InfraApply: %v", err)
	}

	mock.AssertCalled(t, "tofu plan -var-file=terraform.tfvars -out=tfplan")
	mock.AssertCalled(t, "tofu apply tfplan")
	mock.AssertCalled(t, "tofu output -json")
	mock.AssertCallCount(t, 3)

	out := p.Stdout.(*bytes.Buffer).String() //nolint:errcheck // test type assertion
	if !strings.Contains(out, "No plan found") {
		t.Errorf("expected output to contain 'No plan found', got: %s", out)
	}
}

func TestInfraDestroy(t *testing.T) {
	p, mock, _ := newTestInfraParams(t)
	mock.ExpectRun("tofu destroy -var-file=terraform.tfvars", "", nil)

	p.ConfirmFunc = func(string) bool { return true }

	if err := InfraDestroy(p); err != nil {
		t.Fatalf("InfraDestroy: %v", err)
	}

	mock.AssertCalled(t, "tofu destroy -var-file=terraform.tfvars")
	mock.AssertCallCount(t, 1)
}

func TestInfraDestroyNoConfirm(t *testing.T) {
	p, mock, _ := newTestInfraParams(t)

	p.ConfirmFunc = func(string) bool { return false }

	if err := InfraDestroy(p); err != nil {
		t.Fatalf("InfraDestroy: %v", err)
	}

	mock.AssertCallCount(t, 0)

	out := p.Stdout.(*bytes.Buffer).String() //nolint:errcheck // test type assertion
	if !strings.Contains(out, "Cancelled") {
		t.Errorf("expected output to contain 'Cancelled', got: %s", out)
	}
}

func TestInfraOutput(t *testing.T) {
	p, mock, _ := newTestInfraParams(t)
	mock.ExpectRun("tofu output -json", "", nil)

	if err := InfraOutput(p); err != nil {
		t.Fatalf("InfraOutput: %v", err)
	}

	mock.AssertCalled(t, "tofu output -json")
	mock.AssertCallCount(t, 1)
}

func TestInfraKubeconfig(t *testing.T) {
	p, mock, tmp := newTestInfraParams(t)

	clusterID := "ocid1.cluster.oc1.phx.aaaa"
	mock.ExpectRun("tofu output -raw cluster_id", clusterID+"\n", nil)

	kubeconfigPath := filepath.Join(tmp, "kubeconfig")
	expectedOCICmd := "oci ce cluster create-kubeconfig" +
		" --cluster-id " + clusterID +
		" --file " + kubeconfigPath +
		" --region us-phoenix-1" +
		" --token-version 2.0.0" +
		" --kube-endpoint PUBLIC_ENDPOINT"
	mock.ExpectRun(expectedOCICmd, "", nil)

	if err := InfraKubeconfig(p); err != nil {
		t.Fatalf("InfraKubeconfig: %v", err)
	}

	mock.AssertCalled(t, "tofu output -raw cluster_id")
	mock.AssertCalled(t, expectedOCICmd)
	mock.AssertCallCount(t, 2)

	out := p.Stdout.(*bytes.Buffer).String() //nolint:errcheck // test type assertion
	if !strings.Contains(out, "Kubeconfig saved to") {
		t.Errorf("expected 'Kubeconfig saved to' in output, got: %s", out)
	}
}

func TestInfraDirResolution(t *testing.T) {
	tmp := t.TempDir()
	envDir := filepath.Join(tmp, "environments", "dev")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		t.Fatalf("creating env dir: %v", err)
	}

	tests := []struct {
		name      string
		flagDir   string
		cfgInfra  string
		cfgRepos  string
		wantInfra string
	}{
		{
			name:      "flag override wins",
			flagDir:   tmp,
			cfgInfra:  "/some/other",
			cfgRepos:  "/yet/another",
			wantInfra: tmp,
		},
		{
			name:      "config infra_dir used when no flag",
			flagDir:   "",
			cfgInfra:  tmp,
			cfgRepos:  "/yet/another",
			wantInfra: tmp,
		},
		{
			name:      "config repos fallback",
			flagDir:   "",
			cfgInfra:  "",
			cfgRepos:  tmp,
			wantInfra: tmp,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				InfraDir: tt.cfgInfra,
				Repos:    config.Repos{K8sOCIFoundation: tt.cfgRepos},
			}
			pr := paths.NewWithHome(tmp, cfg)

			mock := exec.NewMockExecutor()

			var buf bytes.Buffer
			p := InfraParams{
				Exec:              mock,
				Paths:             pr,
				Stdout:            &buf,
				Env:               "dev",
				Dir:               tt.flagDir,
				DryRun:            true,
				TokenResolverFunc: mockToken("t", "s"),
			}

			if err := InfraOutput(p); err != nil {
				t.Fatalf("InfraOutput: %v", err)
			}

			gotDir := mock.Dir
			if gotDir != envDir {
				t.Errorf("expected executor dir %q, got %q", envDir, gotDir)
			}
		})
	}
}

func TestInfraEnvDirNotFound(t *testing.T) {
	tmp := t.TempDir()
	// Do NOT create environments/staging.

	cfg := &config.Config{}
	pr := paths.NewWithHome(tmp, cfg)
	mock := exec.NewMockExecutor()

	var buf bytes.Buffer
	p := InfraParams{
		Exec:              mock,
		Paths:             pr,
		Stdout:            &buf,
		Env:               "staging",
		Dir:               tmp,
		DryRun:            true,
		TokenResolverFunc: mockToken("t", "s"),
	}

	err := InfraInit(p)
	if err == nil {
		t.Fatal("expected error for missing env dir, got nil")
	}
	if !strings.Contains(err.Error(), "staging") {
		t.Errorf("expected error to mention 'staging', got: %s", err.Error())
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected error to mention 'not found', got: %s", err.Error())
	}
}

func TestInfraTokenResolution(t *testing.T) {
	p, mock, _ := newTestInfraParams(t)
	p.TokenResolverFunc = mockToken("my-secret-token", "test-source")
	mock.ExpectRun("tofu output -json", "", nil)

	if err := InfraOutput(p); err != nil {
		t.Fatalf("InfraOutput: %v", err)
	}

	// Verify the token was set via SetEnv.
	if len(mock.Env) == 0 {
		t.Fatal("expected env to be set on mock executor")
	}
	found := false
	for _, e := range mock.Env {
		if strings.Contains(e, "TF_VAR_cloudflare_api_token=my-secret-token") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected TF_VAR_cloudflare_api_token in env, got: %v", mock.Env)
	}

	out := p.Stdout.(*bytes.Buffer).String() //nolint:errcheck // test type assertion
	if !strings.Contains(out, "test-source") {
		t.Errorf("expected output to mention token source 'test-source', got: %s", out)
	}
}

// ---------------------------------------------------------------------------
// Additional Infra coverage tests
// ---------------------------------------------------------------------------

func mockTokenError(errMsg string) func() (string, string, error) {
	return func() (string, string, error) {
		return "", "", fmt.Errorf("%s", errMsg)
	}
}

func TestInfraTokenResolutionError(t *testing.T) {
	p, _, _ := newTestInfraParams(t)
	p.TokenResolverFunc = mockTokenError("no cloudflare token found")

	err := InfraOutput(p)
	if err == nil {
		t.Fatal("expected error for token resolution failure, got nil")
	}
	if !strings.Contains(err.Error(), "cloudflare token") {
		t.Errorf("expected 'cloudflare token' in error, got: %v", err)
	}
}

func TestInfraConfirmNilFunc(t *testing.T) {
	// infraConfirm with nil ConfirmFunc should default to true (auto-yes).
	p := InfraParams{ConfirmFunc: nil}
	if !infraConfirm(p, "test?") {
		t.Error("infraConfirm with nil ConfirmFunc should return true")
	}
}

func TestInfraConfirmWithFunc(t *testing.T) {
	// infraConfirm with a provided ConfirmFunc should delegate.
	p := InfraParams{ConfirmFunc: func(string) bool { return false }}
	if infraConfirm(p, "test?") {
		t.Error("infraConfirm should return false when ConfirmFunc returns false")
	}
}

func TestInfraInitFails(t *testing.T) {
	p, mock, _ := newTestInfraParams(t)
	mock.ExpectRunWithResult("tofu init -upgrade",
		&exec.Result{Stderr: "init failed"},
		fmt.Errorf("exit status 1"),
	)

	err := InfraInit(p)
	if err == nil {
		t.Fatal("expected error for init failure, got nil")
	}
}

func TestInfraPlanInitFails(t *testing.T) {
	p, mock, _ := newTestInfraParams(t)
	mock.ExpectRunWithResult("tofu init -upgrade",
		&exec.Result{Stderr: "init failed"},
		fmt.Errorf("exit status 1"),
	)

	err := InfraPlan(p)
	if err == nil {
		t.Fatal("expected error for plan init failure, got nil")
	}
}

func TestInfraPlanPlanFails(t *testing.T) {
	p, mock, _ := newTestInfraParams(t)
	mock.ExpectRun("tofu init -upgrade", "", nil)
	mock.ExpectRunWithResult("tofu plan -var-file=terraform.tfvars -out=tfplan",
		&exec.Result{Stderr: "plan failed"},
		fmt.Errorf("exit status 1"),
	)

	err := InfraPlan(p)
	if err == nil {
		t.Fatal("expected error for plan failure, got nil")
	}
}

func TestInfraApplyPlanFailsNoPlanFile(t *testing.T) {
	// No tfplan file, so apply runs plan first, which fails.
	p, mock, _ := newTestInfraParams(t)
	mock.ExpectRunWithResult("tofu plan -var-file=terraform.tfvars -out=tfplan",
		&exec.Result{Stderr: "plan failed"},
		fmt.Errorf("exit status 1"),
	)
	p.ConfirmFunc = func(string) bool { return true }

	err := InfraApply(p)
	if err == nil {
		t.Fatal("expected error for plan failure during apply, got nil")
	}
}

func TestInfraApplyApplyFails(t *testing.T) {
	p, mock, tmp := newTestInfraParams(t)

	// Create tfplan file
	envDir := filepath.Join(tmp, "environments", "dev")
	if err := os.WriteFile(filepath.Join(envDir, "tfplan"), []byte("plan"), 0o644); err != nil {
		t.Fatal(err)
	}

	mock.ExpectRunWithResult("tofu apply tfplan",
		&exec.Result{Stderr: "apply failed"},
		fmt.Errorf("exit status 1"),
	)
	p.ConfirmFunc = func(string) bool { return true }

	err := InfraApply(p)
	if err == nil {
		t.Fatal("expected error for apply failure, got nil")
	}
}

func TestInfraApplyOutputFails(t *testing.T) {
	p, mock, tmp := newTestInfraParams(t)

	envDir := filepath.Join(tmp, "environments", "dev")
	if err := os.WriteFile(filepath.Join(envDir, "tfplan"), []byte("plan"), 0o644); err != nil {
		t.Fatal(err)
	}

	mock.ExpectRun("tofu apply tfplan", "", nil)
	mock.ExpectRunWithResult("tofu output -json",
		&exec.Result{Stderr: "output failed"},
		fmt.Errorf("exit status 1"),
	)
	p.ConfirmFunc = func(string) bool { return true }

	err := InfraApply(p)
	if err == nil {
		t.Fatal("expected error for output failure after apply, got nil")
	}
}

func TestInfraOutputNoInfraDir(t *testing.T) {
	// resolveInfraParams fails because infra dir is empty.
	tmp := t.TempDir()
	cfg := &config.Config{}
	pr := paths.NewWithHome(tmp, cfg)
	mock := exec.NewMockExecutor()

	var buf bytes.Buffer
	p := InfraParams{
		Exec:              mock,
		Paths:             pr,
		Stdout:            &buf,
		Env:               "dev",
		Dir:               "", // no dir override
		DryRun:            true,
		TokenResolverFunc: mockToken("t", "s"),
	}

	err := InfraOutput(p)
	if err == nil {
		t.Fatal("expected error for missing infra dir, got nil")
	}
	if !strings.Contains(err.Error(), "could not find k8s-oci-foundation") {
		t.Errorf("expected infra dir error, got: %v", err)
	}
}

func TestInfraOutputTofuFails(t *testing.T) {
	p, mock, _ := newTestInfraParams(t)
	mock.ExpectRunWithResult("tofu output -json",
		&exec.Result{Stderr: "output error"},
		fmt.Errorf("exit status 1"),
	)

	err := InfraOutput(p)
	if err == nil {
		t.Fatal("expected error for tofu output failure, got nil")
	}
}

func TestInfraKubeconfigClusterIDFails(t *testing.T) {
	p, mock, _ := newTestInfraParams(t)
	mock.ExpectRunWithResult("tofu output -raw cluster_id",
		&exec.Result{Stderr: "no output"},
		fmt.Errorf("exit status 1"),
	)

	err := InfraKubeconfig(p)
	if err == nil {
		t.Fatal("expected error for missing cluster_id, got nil")
	}
	if !strings.Contains(err.Error(), "no cluster_id output found") {
		t.Errorf("expected cluster_id error, got: %v", err)
	}
}

func TestInfraKubeconfigOCIFails(t *testing.T) {
	p, mock, tmp := newTestInfraParams(t)

	clusterID := "ocid1.cluster.oc1.phx.bbbb"
	mock.ExpectRun("tofu output -raw cluster_id", clusterID+"\n", nil)

	kubeconfigPath := filepath.Join(tmp, "kubeconfig")
	ociCmd := "oci ce cluster create-kubeconfig" +
		" --cluster-id " + clusterID +
		" --file " + kubeconfigPath +
		" --region us-phoenix-1" +
		" --token-version 2.0.0" +
		" --kube-endpoint PUBLIC_ENDPOINT"
	mock.ExpectRunWithResult(ociCmd,
		&exec.Result{Stderr: "oci error"},
		fmt.Errorf("exit status 1"),
	)

	err := InfraKubeconfig(p)
	if err == nil {
		t.Fatal("expected error for OCI kubeconfig failure, got nil")
	}
}

func TestInfraKubeconfigCustomRegion(t *testing.T) {
	p, mock, tmp := newTestInfraParams(t)
	p.Region = "eu-amsterdam-1"

	clusterID := "ocid1.cluster.oc1.ams.cccc"
	mock.ExpectRun("tofu output -raw cluster_id", clusterID+"\n", nil)

	kubeconfigPath := filepath.Join(tmp, "kubeconfig")
	expectedOCICmd := "oci ce cluster create-kubeconfig" +
		" --cluster-id " + clusterID +
		" --file " + kubeconfigPath +
		" --region eu-amsterdam-1" +
		" --token-version 2.0.0" +
		" --kube-endpoint PUBLIC_ENDPOINT"
	mock.ExpectRun(expectedOCICmd, "", nil)

	if err := InfraKubeconfig(p); err != nil {
		t.Fatalf("InfraKubeconfig: %v", err)
	}

	mock.AssertCalled(t, expectedOCICmd)

	out := p.Stdout.(*bytes.Buffer).String() //nolint:errcheck // test type assertion
	if !strings.Contains(out, "Kubeconfig saved to") {
		t.Errorf("expected 'Kubeconfig saved to' in output, got: %s", out)
	}
}

// ---------------------------------------------------------------------------
// Dry-run tests: verify no tofu commands are executed
// ---------------------------------------------------------------------------

func TestInfraInitDryRun(t *testing.T) {
	p, mock, _ := newTestInfraParams(t)
	p.DryRun = true

	if err := InfraInit(p); err != nil {
		t.Fatalf("InfraInit dry-run: %v", err)
	}

	// No tofu commands should have been called
	for _, c := range mock.Calls {
		if strings.HasPrefix(c.CommandString(), "tofu") {
			t.Errorf("dry-run should not execute tofu, got: %s", c.CommandString())
		}
	}

	out := p.Stdout.(*bytes.Buffer).String()
	if !strings.Contains(out, "[dry-run] tofu init") {
		t.Errorf("expected dry-run init message, got: %s", out)
	}
}

func TestInfraPlanDryRun(t *testing.T) {
	p, mock, _ := newTestInfraParams(t)
	p.DryRun = true

	if err := InfraPlan(p); err != nil {
		t.Fatalf("InfraPlan dry-run: %v", err)
	}

	for _, c := range mock.Calls {
		if strings.HasPrefix(c.CommandString(), "tofu") {
			t.Errorf("dry-run should not execute tofu, got: %s", c.CommandString())
		}
	}

	out := p.Stdout.(*bytes.Buffer).String()
	if !strings.Contains(out, "[dry-run] tofu plan") {
		t.Errorf("expected dry-run plan message, got: %s", out)
	}
}

func TestInfraApplyDryRun(t *testing.T) {
	p, mock, _ := newTestInfraParams(t)
	p.DryRun = true

	if err := InfraApply(p); err != nil {
		t.Fatalf("InfraApply dry-run: %v", err)
	}

	for _, c := range mock.Calls {
		if strings.HasPrefix(c.CommandString(), "tofu") {
			t.Errorf("dry-run should not execute tofu, got: %s", c.CommandString())
		}
	}

	out := p.Stdout.(*bytes.Buffer).String()
	if !strings.Contains(out, "[dry-run] tofu apply") {
		t.Errorf("expected dry-run apply message, got: %s", out)
	}
}

func TestInfraDestroyDryRun(t *testing.T) {
	p, mock, _ := newTestInfraParams(t)
	p.DryRun = true

	if err := InfraDestroy(p); err != nil {
		t.Fatalf("InfraDestroy dry-run: %v", err)
	}

	for _, c := range mock.Calls {
		if strings.HasPrefix(c.CommandString(), "tofu") {
			t.Errorf("dry-run should not execute tofu, got: %s", c.CommandString())
		}
	}

	out := p.Stdout.(*bytes.Buffer).String()
	if !strings.Contains(out, "[dry-run] tofu destroy") {
		t.Errorf("expected dry-run destroy message, got: %s", out)
	}
}

func TestInfraOutputDryRun(t *testing.T) {
	p, mock, _ := newTestInfraParams(t)
	p.DryRun = true

	if err := InfraOutput(p); err != nil {
		t.Fatalf("InfraOutput dry-run: %v", err)
	}

	for _, c := range mock.Calls {
		if strings.HasPrefix(c.CommandString(), "tofu") {
			t.Errorf("dry-run should not execute tofu, got: %s", c.CommandString())
		}
	}

	out := p.Stdout.(*bytes.Buffer).String()
	if !strings.Contains(out, "[dry-run] tofu output") {
		t.Errorf("expected dry-run output message, got: %s", out)
	}
}

func TestInfraKubeconfigDryRun(t *testing.T) {
	p, mock, _ := newTestInfraParams(t)
	p.DryRun = true

	if err := InfraKubeconfig(p); err != nil {
		t.Fatalf("InfraKubeconfig dry-run: %v", err)
	}

	for _, c := range mock.Calls {
		cmd := c.CommandString()
		if strings.HasPrefix(cmd, "tofu") || strings.HasPrefix(cmd, "oci") {
			t.Errorf("dry-run should not execute tofu/oci, got: %s", cmd)
		}
	}

	out := p.Stdout.(*bytes.Buffer).String()
	if !strings.Contains(out, "[dry-run] tofu output -raw cluster_id") {
		t.Errorf("expected dry-run cluster_id message, got: %s", out)
	}
	if !strings.Contains(out, "[dry-run] oci ce cluster create-kubeconfig") {
		t.Errorf("expected dry-run oci message, got: %s", out)
	}
}

func TestInfraDestroyFails(t *testing.T) {
	p, mock, _ := newTestInfraParams(t)
	mock.ExpectRunWithResult("tofu destroy -var-file=terraform.tfvars",
		&exec.Result{Stderr: "destroy error"},
		fmt.Errorf("exit status 1"),
	)
	p.ConfirmFunc = func(string) bool { return true }

	err := InfraDestroy(p)
	if err == nil {
		t.Fatal("expected error for destroy failure, got nil")
	}
}

// --------------------------------------------------------------------------
// InfraDiscover
// --------------------------------------------------------------------------

func newTestDiscoverParams(t *testing.T) (InfraDiscoverParams, *exec.MockExecutor, string) {
	t.Helper()

	tmp := t.TempDir()
	envDir := filepath.Join(tmp, "environments", "dev")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		t.Fatalf("creating env dir: %v", err)
	}

	mock := exec.NewMockExecutor()
	cfg := &config.Config{}
	pr := paths.NewWithHome(tmp, cfg)

	var buf bytes.Buffer
	p := InfraDiscoverParams{
		Exec:   mock,
		Paths:  pr,
		Stdout: &buf,
		Env:    "dev",
		Dir:    tmp,
	}
	return p, mock, tmp
}

const discoverJSON = `{
  "compartment_id": "ocid1.tenancy.oc1..test",
  "tenancy": {
    "id": "ocid1.tenancy.oc1..test",
    "name": "test-tenancy",
    "home_region": "us-phoenix-1"
  },
  "availability_domains": [
    {"name": "AD-1"},
    {"name": "AD-2"}
  ],
  "shapes": [],
  "images": [],
  "vcns": [],
  "block_volumes": [],
  "limits": []
}`

func TestInfraDiscover(t *testing.T) {
	p, mock, tmp := newTestDiscoverParams(t)
	mock.ExpectRun("oci-tf-bootstrap --json --always-free --oke", discoverJSON, nil)

	if err := InfraDiscover(p); err != nil {
		t.Fatalf("InfraDiscover: %v", err)
	}

	mock.AssertCalled(t, "oci-tf-bootstrap --json --always-free --oke")

	// Verify terraform.tfvars was written
	tfvarsPath := filepath.Join(tmp, "environments", "dev", "terraform.tfvars")
	data, err := os.ReadFile(tfvarsPath)
	if err != nil {
		t.Fatalf("failed to read terraform.tfvars: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, `tenancy_ocid   = "ocid1.tenancy.oc1..test"`) {
		t.Error("terraform.tfvars should contain tenancy_ocid")
	}
	if !strings.Contains(content, `region         = "us-phoenix-1"`) {
		t.Error("terraform.tfvars should contain region")
	}
	if !strings.Contains(content, `environment = "dev"`) {
		t.Error("terraform.tfvars should contain environment")
	}
}

func TestInfraDiscoverWithProfile(t *testing.T) {
	p, mock, _ := newTestDiscoverParams(t)
	p.Profile = "PROD"
	p.Region = "us-ashburn-1"
	p.Compartment = "ocid1.compartment.oc1..custom"
	mock.ExpectRun(
		"oci-tf-bootstrap --json --always-free --oke --profile PROD --region us-ashburn-1 --compartment ocid1.compartment.oc1..custom",
		discoverJSON, nil)

	if err := InfraDiscover(p); err != nil {
		t.Fatalf("InfraDiscover: %v", err)
	}

	// Region override should be used in tfvars
	tfvarsPath := filepath.Join(p.Dir, "environments", "dev", "terraform.tfvars")
	data, err := os.ReadFile(tfvarsPath)
	if err != nil {
		t.Fatalf("failed to read terraform.tfvars: %v", err)
	}
	if !strings.Contains(string(data), `region         = "us-ashburn-1"`) {
		t.Error("terraform.tfvars should use overridden region")
	}
}

func TestInfraDiscoverDryRun(t *testing.T) {
	p, _, tmp := newTestDiscoverParams(t)
	p.DryRun = true

	if err := InfraDiscover(p); err != nil {
		t.Fatalf("InfraDiscover dry-run: %v", err)
	}

	// Verify no terraform.tfvars was written
	tfvarsPath := filepath.Join(tmp, "environments", "dev", "terraform.tfvars")
	if _, err := os.Stat(tfvarsPath); !os.IsNotExist(err) {
		t.Error("dry-run should not write terraform.tfvars")
	}
}

func TestInfraDiscoverBootstrapFails(t *testing.T) {
	p, mock, _ := newTestDiscoverParams(t)
	mock.ExpectRun("oci-tf-bootstrap --json --always-free --oke", "",
		fmt.Errorf("oci-tf-bootstrap: command not found"))

	err := InfraDiscover(p)
	if err == nil {
		t.Fatal("expected error when oci-tf-bootstrap fails")
	}
	if !strings.Contains(err.Error(), "oci-tf-bootstrap failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestInfraDiscoverBadJSON(t *testing.T) {
	p, mock, _ := newTestDiscoverParams(t)
	mock.ExpectRun("oci-tf-bootstrap --json --always-free --oke", "{not valid json", nil)

	err := InfraDiscover(p)
	if err == nil {
		t.Fatal("expected error for bad JSON")
	}
	if !strings.Contains(err.Error(), "parsing oci-tf-bootstrap output") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestInfraDiscoverHeaderBeforeJSON(t *testing.T) {
	p, mock, _ := newTestDiscoverParams(t)
	// oci-tf-bootstrap prints header/progress info before the JSON object
	headerOutput := "oci-tf-bootstrap\n  Profile: DEFAULT\n  Config: ~/.oci/config\n  Region: us-phoenix-1\n" + discoverJSON
	mock.ExpectRun("oci-tf-bootstrap --json --always-free --oke", headerOutput, nil)

	if err := InfraDiscover(p); err != nil {
		t.Fatalf("InfraDiscover with header: %v", err)
	}

	// Should still produce valid tfvars
	tfvarsPath := filepath.Join(p.Dir, "environments", "dev", "terraform.tfvars")
	data, err := os.ReadFile(tfvarsPath)
	if err != nil {
		t.Fatalf("failed to read terraform.tfvars: %v", err)
	}
	if !strings.Contains(string(data), `region         = "us-phoenix-1"`) {
		t.Error("terraform.tfvars should contain region from JSON, not header")
	}
}

func TestInfraDiscoverNoJSON(t *testing.T) {
	p, mock, _ := newTestDiscoverParams(t)
	mock.ExpectRun("oci-tf-bootstrap --json --always-free --oke", "no json at all", nil)

	err := InfraDiscover(p)
	if err == nil {
		t.Fatal("expected error when output has no JSON")
	}
	if !strings.Contains(err.Error(), "oci-tf-bootstrap produced no JSON output") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestInfraDiscoverNoInfraDir(t *testing.T) {
	mock := exec.NewMockExecutor()
	pr := paths.NewWithHome("/nonexistent", nil)
	var buf bytes.Buffer

	err := InfraDiscover(InfraDiscoverParams{
		Exec:   mock,
		Paths:  pr,
		Stdout: &buf,
		Env:    "dev",
	})
	if err == nil {
		t.Fatal("expected error when infra dir not found")
	}
	if !strings.Contains(err.Error(), "could not find k8s-oci-foundation") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestInfraDiscoverCreatesEnvDir(t *testing.T) {
	tmp := t.TempDir()
	// Don't pre-create environments/staging -- discover should create it
	mock := exec.NewMockExecutor()
	cfg := &config.Config{}
	pr := paths.NewWithHome(tmp, cfg)
	var buf bytes.Buffer

	mock.ExpectRun("oci-tf-bootstrap --json --always-free --oke", discoverJSON, nil)

	err := InfraDiscover(InfraDiscoverParams{
		Exec:   mock,
		Paths:  pr,
		Stdout: &buf,
		Env:    "staging",
		Dir:    tmp,
	})
	if err != nil {
		t.Fatalf("InfraDiscover: %v", err)
	}

	// Verify environments/staging was created with terraform.tfvars
	tfvarsPath := filepath.Join(tmp, "environments", "staging", "terraform.tfvars")
	if _, err := os.Stat(tfvarsPath); os.IsNotExist(err) {
		t.Error("discover should create env dir and terraform.tfvars")
	}
}

func TestInfraDiscoverWithConfigValues(t *testing.T) {
	tmp := t.TempDir()
	envDir := filepath.Join(tmp, "environments", "dev")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write a launch config with Cloudflare values
	cfgPath := filepath.Join(tmp, "launch-config.yaml")
	cfgContent := "domain: example.com\ncloudflare_account_id: cf-account-123\n"
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0o600); err != nil {
		t.Fatal(err)
	}

	mock := exec.NewMockExecutor()
	pr := paths.NewWithHome(tmp, nil)
	var buf bytes.Buffer

	mock.ExpectRun("oci-tf-bootstrap --json --always-free --oke", discoverJSON, nil)

	err := InfraDiscover(InfraDiscoverParams{
		Exec:    mock,
		Paths:   pr,
		Stdout:  &buf,
		Env:     "dev",
		Dir:     tmp,
		CfgFile: cfgPath,
	})
	if err != nil {
		t.Fatalf("InfraDiscover: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(envDir, "terraform.tfvars"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, `cloudflare_domain      = "example.com"`) {
		t.Error("terraform.tfvars should contain domain from launch config")
	}
	if !strings.Contains(content, `cloudflare_account_id  = "cf-account-123"`) {
		t.Error("terraform.tfvars should contain cloudflare account ID from launch config")
	}
}
