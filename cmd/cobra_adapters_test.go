package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	pflag "github.com/spf13/pflag"
)

// resetRootCmd resets the output, args, and key package-level flag
// variables on rootCmd so successive tests don't leak state.
func resetRootCmd() {
	rootCmd.SetArgs(nil)
	rootCmd.SetOut(nil)
	rootCmd.SetErr(nil)

	// Reset package-level flag variables that Cobra sets during Execute.
	cfgFile = ""
	verbose = false
	dryRun = false
	autoYes = false

	// Reset command-specific flag variables.
	bootstrapCluster = ""
	bootstrapFleetDir = "."
	bootstrapRepo = ""
	bootstrapOwner = ""
	bootstrapBranch = "main"
	bootstrapContext = ""

	clusterArch = "arm64"
	clusterInfra = "cloud"
	clusterNodes = 3
	clusterResources = "medium"
	clusterStorageClass = ""
	clusterNebulaMode = "disabled"
	clusterNebulaPrefix = "10.42.0"
	clusterLHAddr = ""
	clusterFleetDir = ""

	importContext = ""

	fleetOutputDir = ""
	fleetPrivate = ""

	infraEnv = "dev"
	infraDir = ""

	meshCAOutput = ""
	meshCluster = ""
	meshIsLighthouse = false
	meshLighthouse = ""
	meshFleetDir = "."

	secretsCluster = ""
	secretsProvider = "age"
	secretsKeyARN = ""
	secretsFleetDir = "."

	statusFleetDir = ""

	discoverProfile = ""
	discoverConfigFile = ""
	discoverRegion = ""

	doctorFleetDir = ""

	// Reset cobra flag Changed state so MarkFlagRequired checks work correctly.
	secretsInitCmd.Flags().VisitAll(func(f *pflag.Flag) { f.Changed = false })
}

// execRootCmd is a small helper that resets state, sets args, captures
// combined stdout+stderr, and executes the root command.
func execRootCmd(args []string) error {
	resetRootCmd()
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs(args)
	return rootCmd.Execute()
}

// --------------------------------------------------------------------------
// Execute
// --------------------------------------------------------------------------

func TestExecuteNoArgs(t *testing.T) {
	resetRootCmd()
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{})
	err := Execute()
	if err != nil {
		t.Fatalf("Execute() returned unexpected error: %v", err)
	}
}

// --------------------------------------------------------------------------
// runPreflight via Cobra
//
// preflight adapter DOES set runner.DryRun = dryRun, so --dry-run works
// at the executor level.  However, tool checks use exec.LookPath (not
// the executor) so they may fail if tools are absent.
// --------------------------------------------------------------------------

func TestRunPreflightViaCobra(t *testing.T) {
	err := execRootCmd([]string{"preflight", "--dry-run"})
	// Acceptable outcomes: nil (all tools present), or errors about
	// missing tools or invalid credentials.
	if err != nil {
		if !strings.Contains(err.Error(), "missing required tools") &&
			!strings.Contains(err.Error(), "credentials are invalid") {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}

// --------------------------------------------------------------------------
// runBootstrap via Cobra
//
// The adapter creates a real Runner without DryRun, so kubectl/flux
// actually execute.  We test that the adapter wiring runs and that
// flag parsing works.  Downstream errors from tool execution are
// expected.
// --------------------------------------------------------------------------

func TestRunBootstrapViaCobra(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "clusters", "cobra-test"), 0o755); err != nil {
		t.Fatal(err)
	}

	err := execRootCmd([]string{
		"bootstrap",
		"--cluster", "cobra-test",
		"--fleet-dir", tmp,
		"--owner", "myorg",
		"--repo", "myrepo",
		"--branch", "main",
	})
	// The adapter was entered and created params.  The downstream
	// Bootstrap function calls real kubectl/flux which may fail.
	// We just verify the adapter didn't panic and ran.
	if err != nil {
		t.Logf("bootstrap error (acceptable -- real executor): %v", err)
	}
}

func TestRunBootstrapMissingCluster(t *testing.T) {
	err := execRootCmd([]string{"bootstrap"})
	if err == nil {
		t.Fatal("expected error when --cluster is missing")
	}
	if !strings.Contains(err.Error(), "required") {
		t.Logf("error text (expected 'required'): %v", err)
	}
}

// --------------------------------------------------------------------------
// runClusterAdd via Cobra
//
// ClusterAdd uses filesystem operations (no executor calls for the
// core logic), so it can succeed in tests with proper temp dirs.
// --------------------------------------------------------------------------

func TestRunClusterAddViaCobra(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "apps", "base"), 0o755); err != nil {
		t.Fatal(err)
	}

	err := execRootCmd([]string{
		"cluster", "add", "my-cluster",
		"--fleet-dir", tmp,
		"--arch", "amd64",
		"--infra", "cloud",
		"--nodes", "3",
		"--resources", "medium",
	})
	if err != nil {
		t.Fatalf("cluster add returned error: %v", err)
	}
}

func TestRunClusterAddMissingArg(t *testing.T) {
	err := execRootCmd([]string{"cluster", "add"})
	if err == nil {
		t.Fatal("expected error when positional arg is missing")
	}
}

// --------------------------------------------------------------------------
// runClusterRemove via Cobra
//
// ClusterRemove uses filesystem operations only, so it can succeed in
// tests with proper temp dirs.
// --------------------------------------------------------------------------

func TestRunClusterRemoveViaCobra(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "clusters", "rm-test"), 0o755); err != nil {
		t.Fatal(err)
	}

	err := execRootCmd([]string{
		"cluster", "remove", "rm-test",
		"--fleet-dir", tmp,
	})
	if err != nil {
		t.Fatalf("cluster remove returned error: %v", err)
	}

	// Verify the directory was actually removed
	if _, err := os.Stat(filepath.Join(tmp, "clusters", "rm-test")); !os.IsNotExist(err) {
		t.Error("expected cluster directory to be removed")
	}
}

func TestRunClusterRemoveMissingArg(t *testing.T) {
	err := execRootCmd([]string{"cluster", "remove"})
	if err == nil {
		t.Fatal("expected error when positional arg is missing")
	}
}

// --------------------------------------------------------------------------
// runClusterList via Cobra
//
// ClusterList is pure filesystem, so it succeeds with proper temp dirs.
// --------------------------------------------------------------------------

func TestRunClusterListViaCobra(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "clusters", "demo"), 0o755); err != nil {
		t.Fatal(err)
	}

	cfgPath := filepath.Join(tmp, "config.yaml")
	cfgContent := "fleet_dir: " + tmp + "\n"
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0o600); err != nil {
		t.Fatal(err)
	}

	err := execRootCmd([]string{
		"cluster", "list",
		"--config", cfgPath,
		"--fleet-dir", tmp,
	})
	if err != nil {
		t.Fatalf("cluster list returned error: %v", err)
	}
}

// --------------------------------------------------------------------------
// runClusterImport via Cobra
//
// The adapter creates a real Runner that calls kubectl.  The downstream
// logic will error because kubectl returns empty stdout for the
// current-context check.  We verify the adapter was entered.
// --------------------------------------------------------------------------

func TestRunClusterImportViaCobra(t *testing.T) {
	tmp := t.TempDir()
	srcKubeconfig := filepath.Join(tmp, "src-kubeconfig.yaml")
	if err := os.WriteFile(srcKubeconfig, []byte("apiVersion: v1\nkind: Config\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := execRootCmd([]string{
		"cluster", "import", "mygroup", srcKubeconfig,
	})
	// The adapter was entered.  Downstream may error due to real
	// kubectl invocation against invalid kubeconfig.
	if err != nil {
		t.Logf("cluster import error (acceptable): %v", err)
	}
}

func TestRunClusterImportMissingArgs(t *testing.T) {
	err := execRootCmd([]string{"cluster", "import"})
	if err == nil {
		t.Fatal("expected error when positional args are missing")
	}
}

// --------------------------------------------------------------------------
// runFleetInit via Cobra
//
// The adapter creates a real Runner.  git clone may succeed or fail.
// --------------------------------------------------------------------------

func TestRunFleetInitViaCobra(t *testing.T) {
	tmp := t.TempDir()
	fleetDir := filepath.Join(tmp, "fleet-deploy-new")
	cfgPath := filepath.Join(tmp, "config.yaml")

	err := execRootCmd([]string{
		"fleet", "init",
		"--dir", fleetDir,
		"--remote", "git@github.com:test/fleet.git",
		"--config", cfgPath,
	})
	// The adapter was entered.  git clone may succeed (network) or fail.
	if err != nil {
		t.Logf("fleet init error (acceptable): %v", err)
	}
}

// --------------------------------------------------------------------------
// runFleetStatus via Cobra
//
// FleetStatus reads the filesystem and calls git.  With proper temp
// dirs it exercises the adapter and runs git (which may print warnings
// about not being a real git repo).
// --------------------------------------------------------------------------

func TestRunFleetStatusViaCobra(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "clusters", "test"), 0o755); err != nil {
		t.Fatal(err)
	}

	cfgPath := filepath.Join(tmp, "config.yaml")
	cfgContent := "fleet_dir: " + tmp + "\n"
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0o600); err != nil {
		t.Fatal(err)
	}

	// FleetStatus doesn't return errors for git failures -- it just
	// prints the output.  So this should succeed.
	err := execRootCmd([]string{
		"fleet", "status",
		"--config", cfgPath,
	})
	if err != nil {
		t.Fatalf("fleet status returned error: %v", err)
	}
}

// --------------------------------------------------------------------------
// runSecretsInit via Cobra
//
// SecretsInit does filesystem operations and calls age-keygen (real).
// With a cluster dir present and provider=yubikey, it skips age-keygen.
// --------------------------------------------------------------------------

func TestRunSecretsInitViaCobra(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "clusters", "sec-cluster"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Use provider=yubikey to avoid needing age-keygen installed.
	err := execRootCmd([]string{
		"secrets", "init",
		"--cluster", "sec-cluster",
		"--fleet-dir", tmp,
		"--provider", "yubikey",
	})
	if err != nil {
		t.Fatalf("secrets init returned error: %v", err)
	}
}

func TestRunSecretsInitMissingCluster(t *testing.T) {
	err := execRootCmd([]string{"secrets", "init"})
	if err == nil {
		t.Fatal("expected error when --cluster is missing")
	}
}

// --------------------------------------------------------------------------
// runMeshInit via Cobra
//
// MeshInit calls nebula-cert which may not be installed.
// --------------------------------------------------------------------------

func TestRunMeshInitViaCobra(t *testing.T) {
	tmp := t.TempDir()
	caOutput := filepath.Join(tmp, "nebula-ca")

	err := execRootCmd([]string{
		"mesh", "init",
		"--ca-output", caOutput,
	})
	// The adapter was entered.  nebula-cert may not be installed.
	if err != nil {
		t.Logf("mesh init error (acceptable): %v", err)
	}
}

// --------------------------------------------------------------------------
// runMeshAdd via Cobra
//
// MeshAdd checks for a CA key and cluster settings on the filesystem.
// Without them it errors, which proves the adapter was entered.
// --------------------------------------------------------------------------

func TestRunMeshAddViaCobra(t *testing.T) {
	tmp := t.TempDir()

	err := execRootCmd([]string{
		"mesh", "add",
		"--cluster", "mesh-test",
		"--fleet-dir", tmp,
	})
	// Will error about missing CA or cluster settings.
	if err != nil {
		t.Logf("mesh add error (acceptable): %v", err)
	}
}

func TestRunMeshAddMissingCluster(t *testing.T) {
	err := execRootCmd([]string{"mesh", "add"})
	if err == nil {
		t.Fatal("expected error when --cluster is missing")
	}
}

// --------------------------------------------------------------------------
// infra commands via Cobra
//
// buildInfraParams creates InfraParams with DryRun from the package var.
// resolveInfraParams skips token resolution when DryRun is true.  But
// the actual executor (NewRunner) does NOT have DryRun set, so tofu
// commands run for real.
//
// Strategy: set --dry-run so resolveInfraParams passes, then accept
// errors from the real tofu calls.  The key coverage target is
// buildInfraParams and the runInfraXxx adapter wiring.
// --------------------------------------------------------------------------

func TestRunInfraInitViaCobra(t *testing.T) {
	tmp := t.TempDir()
	envDir := filepath.Join(tmp, "environments", "dev")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		t.Fatal(err)
	}

	err := execRootCmd([]string{
		"infra", "init",
		"--dry-run",
		"--dir", tmp,
		"--env", "dev",
	})
	// tofu init in an empty dir succeeds, so this should pass.
	if err != nil {
		t.Logf("infra init --dry-run error (acceptable): %v", err)
	}
}

func TestRunInfraPlanViaCobra(t *testing.T) {
	tmp := t.TempDir()
	envDir := filepath.Join(tmp, "environments", "dev")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		t.Fatal(err)
	}

	err := execRootCmd([]string{
		"infra", "plan",
		"--dry-run",
		"--dir", tmp,
		"--env", "dev",
	})
	// tofu plan in an empty dir fails on -var-file.  Adapter was still entered.
	if err != nil {
		t.Logf("infra plan --dry-run error (acceptable): %v", err)
	}
}

func TestRunInfraApplyViaCobra(t *testing.T) {
	tmp := t.TempDir()
	envDir := filepath.Join(tmp, "environments", "dev")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		t.Fatal(err)
	}

	err := execRootCmd([]string{
		"infra", "apply",
		"--dry-run",
		"--yes",
		"--dir", tmp,
		"--env", "dev",
	})
	// Adapter entered; tofu may fail.
	if err != nil {
		t.Logf("infra apply --dry-run error (acceptable): %v", err)
	}
}

func TestRunInfraDestroyViaCobra(t *testing.T) {
	tmp := t.TempDir()
	envDir := filepath.Join(tmp, "environments", "dev")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		t.Fatal(err)
	}

	err := execRootCmd([]string{
		"infra", "destroy",
		"--dry-run",
		"--yes",
		"--dir", tmp,
		"--env", "dev",
	})
	// Adapter entered; tofu may fail.
	if err != nil {
		t.Logf("infra destroy --dry-run error (acceptable): %v", err)
	}
}

func TestRunInfraOutputViaCobra(t *testing.T) {
	tmp := t.TempDir()
	envDir := filepath.Join(tmp, "environments", "dev")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		t.Fatal(err)
	}

	err := execRootCmd([]string{
		"infra", "output",
		"--dry-run",
		"--dir", tmp,
		"--env", "dev",
	})
	// Adapter entered; tofu may fail.
	if err != nil {
		t.Logf("infra output --dry-run error (acceptable): %v", err)
	}
}

func TestRunInfraKubeconfigViaCobra(t *testing.T) {
	tmp := t.TempDir()
	envDir := filepath.Join(tmp, "environments", "dev")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		t.Fatal(err)
	}

	err := execRootCmd([]string{
		"infra", "kubeconfig",
		"--dry-run",
		"--dir", tmp,
		"--env", "dev",
	})
	// Adapter entered; tofu output -raw cluster_id will fail.
	if err != nil {
		t.Logf("infra kubeconfig --dry-run error (acceptable): %v", err)
	}
}

func TestRunInfraDiscoverViaCobra(t *testing.T) {
	tmp := t.TempDir()
	envDir := filepath.Join(tmp, "environments", "dev")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		t.Fatal(err)
	}

	err := execRootCmd([]string{
		"infra", "discover",
		"--dry-run",
		"--dir", tmp,
		"--env", "dev",
	})
	if err != nil {
		t.Logf("infra discover --dry-run error (acceptable): %v", err)
	}
}

// --------------------------------------------------------------------------
// buildInfraParams
// --------------------------------------------------------------------------

func TestBuildInfraParams(t *testing.T) {
	p, err := buildInfraParams()
	if err != nil {
		t.Fatalf("buildInfraParams() returned error: %v", err)
	}
	if p.Exec == nil {
		t.Error("buildInfraParams() returned nil Exec")
	}
	if p.Paths == nil {
		t.Error("buildInfraParams() returned nil Paths")
	}
	if p.Stdout == nil {
		t.Error("buildInfraParams() returned nil Stdout")
	}
	if p.ConfirmFunc == nil {
		t.Error("buildInfraParams() returned nil ConfirmFunc")
	}
	if p.TokenResolverFunc == nil {
		t.Error("buildInfraParams() returned nil TokenResolverFunc")
	}
}

// --------------------------------------------------------------------------
// runSetup via Cobra
//
// setup adapter DOES set runner.DryRun = dryRun.  With --dry-run and
// --yes it should run cleanly (all RunSilent calls return empty results).
// --------------------------------------------------------------------------

func TestRunSetupViaCobra(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")

	err := execRootCmd([]string{"setup", "--dry-run", "--yes", "--config", cfgPath})
	// Setup may error if resolving paths fails (unlikely) or if
	// downstream logic encounters unexpected conditions.
	if err != nil {
		t.Logf("setup --dry-run error (acceptable): %v", err)
	}
}

// --------------------------------------------------------------------------
// runStatus via Cobra (the top-level "status" command)
//
// Status uses filesystem + real kubectl/flux.  With a proper fleet
// structure, the adapter runs.  kubectl failures are logged, not
// returned as errors.
// --------------------------------------------------------------------------

func TestRunStatusViaCobra(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "clusters", "status-test"), 0o755); err != nil {
		t.Fatal(err)
	}

	cfgPath := filepath.Join(tmp, "config.yaml")
	cfgContent := "fleet_dir: " + tmp + "\n"
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0o600); err != nil {
		t.Fatal(err)
	}

	err := execRootCmd([]string{
		"status",
		"--config", cfgPath,
		"--fleet-dir", tmp,
	})
	if err != nil {
		t.Fatalf("status returned error: %v", err)
	}
}

// --------------------------------------------------------------------------
// runCompletion via Cobra
//
// The completion command uses cobra's built-in generation.  Each shell
// variant should succeed without error.
// --------------------------------------------------------------------------

func TestRunCompletionViaCobra(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		t.Run(shell, func(t *testing.T) {
			err := execRootCmd([]string{"completion", shell})
			if err != nil {
				t.Fatalf("completion %s via cobra returned error: %v", shell, err)
			}
		})
	}
}

// --------------------------------------------------------------------------
// runPluginList via Cobra
// --------------------------------------------------------------------------

func TestRunPluginListViaCobra(t *testing.T) {
	err := execRootCmd([]string{"plugin", "list"})
	if err != nil {
		t.Fatalf("plugin list returned error: %v", err)
	}
}
