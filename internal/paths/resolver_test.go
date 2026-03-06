package paths

import (
	"path/filepath"
	"testing"

	"github.com/disentangle-network/launch/internal/config"
)

func TestNewWithHome(t *testing.T) {
	r := NewWithHome("/home/test", nil)
	if r.HomeDir != "/home/test" {
		t.Errorf("HomeDir = %q, want %q", r.HomeDir, "/home/test")
	}
	if r.Cfg == nil {
		t.Fatal("Cfg should not be nil")
	}
}

func TestConfigDir(t *testing.T) {
	r := NewWithHome("/home/test", nil)
	want := filepath.Join("/home/test", ".config", "launch")
	if got := r.ConfigDir(); got != want {
		t.Errorf("ConfigDir() = %q, want %q", got, want)
	}
}

func TestConfigPath(t *testing.T) {
	r := NewWithHome("/home/test", nil)
	want := filepath.Join("/home/test", ".config", "launch", "config.yaml")
	if got := r.ConfigPath(); got != want {
		t.Errorf("ConfigPath() = %q, want %q", got, want)
	}
}

func TestAgeKeyFileDefault(t *testing.T) {
	t.Setenv("SOPS_AGE_KEY_FILE", "")
	r := NewWithHome("/home/test", nil)
	want := filepath.Join("/home/test", ".config", "sops", "age", "keys.txt")
	if got := r.AgeKeyFile(); got != want {
		t.Errorf("AgeKeyFile() = %q, want %q", got, want)
	}
}

func TestAgeKeyFileEnvOverride(t *testing.T) {
	t.Setenv("SOPS_AGE_KEY_FILE", "/custom/age.key")
	r := NewWithHome("/home/test", nil)
	if got := r.AgeKeyFile(); got != "/custom/age.key" {
		t.Errorf("AgeKeyFile() = %q, want %q", got, "/custom/age.key")
	}
}

func TestNebulaCADir(t *testing.T) {
	r := NewWithHome("/home/test", nil)
	want := filepath.Join("/home/test", ".config", "disentangle", "nebula-ca")
	if got := r.NebulaCADir(); got != want {
		t.Errorf("NebulaCADir() = %q, want %q", got, want)
	}
}

func TestNebulaCAKey(t *testing.T) {
	r := NewWithHome("/home/test", nil)
	want := filepath.Join("/home/test", ".config", "disentangle", "nebula-ca", "ca.key")
	if got := r.NebulaCAKey(); got != want {
		t.Errorf("NebulaCAKey() = %q, want %q", got, want)
	}
}

func TestNebulaCACert(t *testing.T) {
	r := NewWithHome("/home/test", nil)
	want := filepath.Join("/home/test", ".config", "disentangle", "nebula-ca", "ca.crt")
	if got := r.NebulaCACert(); got != want {
		t.Errorf("NebulaCACert() = %q, want %q", got, want)
	}
}

func TestKubeGroupDir(t *testing.T) {
	r := NewWithHome("/home/test", nil)
	want := filepath.Join("/home/test", ".kube", "disentangle")
	if got := r.KubeGroupDir("disentangle"); got != want {
		t.Errorf("KubeGroupDir() = %q, want %q", got, want)
	}
}

func TestKubeGroupConfig(t *testing.T) {
	r := NewWithHome("/home/test", nil)
	want := filepath.Join("/home/test", ".kube", "disentangle", "config")
	if got := r.KubeGroupConfig("disentangle"); got != want {
		t.Errorf("KubeGroupConfig() = %q, want %q", got, want)
	}
}

func TestFleetDirOverride(t *testing.T) {
	r := NewWithHome("/home/test", &config.Config{FleetDir: "/cfg/fleet"})
	if got := r.FleetDir("/override/fleet"); got != "/override/fleet" {
		t.Errorf("FleetDir(override) = %q, want %q", got, "/override/fleet")
	}
}

func TestFleetDirFromConfig(t *testing.T) {
	r := NewWithHome("/home/test", &config.Config{FleetDir: "/cfg/fleet"})
	if got := r.FleetDir(""); got != "/cfg/fleet" {
		t.Errorf("FleetDir('') = %q, want %q", got, "/cfg/fleet")
	}
}

func TestFleetDirDefault(t *testing.T) {
	r := NewWithHome("/home/test", nil)
	want := filepath.Join("/home/test", "DISENTANGLE-NETWORK", "fleet-deploy")
	if got := r.FleetDir(""); got != want {
		t.Errorf("FleetDir('') = %q, want %q", got, want)
	}
}

func TestFleetClusterDir(t *testing.T) {
	r := NewWithHome("/home/test", nil)
	want := filepath.Join("/fleet", "clusters", "dev")
	if got := r.FleetClusterDir("/fleet", "dev"); got != want {
		t.Errorf("FleetClusterDir() = %q, want %q", got, want)
	}
}

func TestFleetSecretsDir(t *testing.T) {
	r := NewWithHome("/home/test", nil)
	want := filepath.Join("/fleet", "secrets", "dev")
	if got := r.FleetSecretsDir("/fleet", "dev"); got != want {
		t.Errorf("FleetSecretsDir() = %q, want %q", got, want)
	}
}

func TestInfraDirOverride(t *testing.T) {
	r := NewWithHome("/home/test", nil)
	if got := r.InfraDir("/custom/infra"); got != "/custom/infra" {
		t.Errorf("InfraDir(override) = %q, want %q", got, "/custom/infra")
	}
}

func TestInfraDirFromConfig(t *testing.T) {
	r := NewWithHome("/home/test", &config.Config{InfraDir: "/cfg/infra"})
	if got := r.InfraDir(""); got != "/cfg/infra" {
		t.Errorf("InfraDir('') = %q, want %q", got, "/cfg/infra")
	}
}

func TestInfraDirFromRepos(t *testing.T) {
	r := NewWithHome("/home/test", &config.Config{
		Repos: config.Repos{K8sOCIFoundation: "/repos/k8s"},
	})
	if got := r.InfraDir(""); got != "/repos/k8s" {
		t.Errorf("InfraDir('') = %q, want %q", got, "/repos/k8s")
	}
}

func TestInfraDirEmpty(t *testing.T) {
	r := NewWithHome("/home/test", nil)
	if got := r.InfraDir(""); got != "" {
		t.Errorf("InfraDir('') = %q, want empty", got)
	}
}

func TestInfraEnvDir(t *testing.T) {
	r := NewWithHome("/home/test", nil)
	want := filepath.Join("/infra", "environments", "dev")
	if got := r.InfraEnvDir("/infra", "dev"); got != want {
		t.Errorf("InfraEnvDir() = %q, want %q", got, want)
	}
}

func TestOCIConfigPath(t *testing.T) {
	r := NewWithHome("/home/test", nil)
	want := filepath.Join("/home/test", ".oci", "config")
	if got := r.OCIConfigPath(); got != want {
		t.Errorf("OCIConfigPath() = %q, want %q", got, want)
	}
}

func TestNewWithConfig(t *testing.T) {
	cfg := &config.Config{FleetDir: "/test/fleet"}
	r, err := New(cfg)
	if err != nil {
		t.Fatalf("New() returned error: %v", err)
	}
	if r.HomeDir == "" {
		t.Error("HomeDir should not be empty")
	}
	if r.Cfg.FleetDir != "/test/fleet" {
		t.Errorf("Cfg.FleetDir = %q, want %q", r.Cfg.FleetDir, "/test/fleet")
	}
}

func TestNewWithNilConfig(t *testing.T) {
	r, err := New(nil)
	if err != nil {
		t.Fatalf("New(nil) returned error: %v", err)
	}
	if r.Cfg == nil {
		t.Fatal("Cfg should not be nil when New is called with nil")
	}
}

func TestNewReturnsErrorWhenHomeUnavailable(t *testing.T) {
	t.Setenv("HOME", "")
	_, err := New(nil)
	if err == nil {
		t.Fatal("New() should return error when HOME is not set")
	}
}

func TestInfraDirPrecedence(t *testing.T) {
	cfg := &config.Config{
		InfraDir: "/cfg/infra",
		Repos:    config.Repos{K8sOCIFoundation: "/repos/k8s"},
	}
	r := NewWithHome("/home/test", cfg)

	// Override flag takes priority over everything
	if got := r.InfraDir("/flag"); got != "/flag" {
		t.Errorf("InfraDir with flag = %q, want /flag", got)
	}

	// Config infra_dir takes priority over repos
	if got := r.InfraDir(""); got != "/cfg/infra" {
		t.Errorf("InfraDir with cfg = %q, want /cfg/infra", got)
	}

	// Repos used when infra_dir is empty
	cfg.InfraDir = ""
	if got := r.InfraDir(""); got != "/repos/k8s" {
		t.Errorf("InfraDir fallback = %q, want /repos/k8s", got)
	}
}
