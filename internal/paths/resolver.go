// Package paths centralizes all filesystem path resolution for the launch CLI.
package paths

import (
	"os"
	"path/filepath"

	"github.com/disentangle-network/launch/internal/config"
)

// Resolver computes filesystem paths based on home directory and config.
type Resolver struct {
	HomeDir string
	Cfg     *config.Config
}

// New creates a Resolver using the real home directory.
func New(cfg *config.Config) (*Resolver, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		cfg = &config.Config{}
	}
	return &Resolver{HomeDir: home, Cfg: cfg}, nil
}

// NewWithHome creates a Resolver with an explicit home directory (for tests).
func NewWithHome(home string, cfg *config.Config) *Resolver {
	if cfg == nil {
		cfg = &config.Config{}
	}
	return &Resolver{HomeDir: home, Cfg: cfg}
}

// ConfigDir returns ~/.config/launch.
func (r *Resolver) ConfigDir() string {
	return filepath.Join(r.HomeDir, ".config", "launch")
}

// ConfigPath returns ~/.config/launch/config.yaml.
func (r *Resolver) ConfigPath() string {
	return filepath.Join(r.ConfigDir(), "config.yaml")
}

// AgeKeyFile returns the SOPS age key path.
// Checks SOPS_AGE_KEY_FILE env var first, then the default location.
func (r *Resolver) AgeKeyFile() string {
	if v := os.Getenv("SOPS_AGE_KEY_FILE"); v != "" {
		return v
	}
	return filepath.Join(r.HomeDir, ".config", "sops", "age", "keys.txt")
}

// NebulaCADir returns ~/.config/disentangle/nebula-ca.
func (r *Resolver) NebulaCADir() string {
	return filepath.Join(r.HomeDir, ".config", "disentangle", "nebula-ca")
}

// NebulaCAKey returns the CA key path within the nebula CA directory.
func (r *Resolver) NebulaCAKey() string {
	return filepath.Join(r.NebulaCADir(), "ca.key")
}

// NebulaCACert returns the CA cert path within the nebula CA directory.
func (r *Resolver) NebulaCACert() string {
	return filepath.Join(r.NebulaCADir(), "ca.crt")
}

// KubeGroupDir returns ~/.kube/<group>.
func (r *Resolver) KubeGroupDir(group string) string {
	return filepath.Join(r.HomeDir, ".kube", group)
}

// KubeGroupConfig returns ~/.kube/<group>/config.
func (r *Resolver) KubeGroupConfig(group string) string {
	return filepath.Join(r.KubeGroupDir(group), "config")
}

// FleetDir returns the fleet repository directory.
// Priority: override flag > config > default.
func (r *Resolver) FleetDir(override string) string {
	if override != "" {
		return override
	}
	if r.Cfg != nil && r.Cfg.FleetDir != "" {
		return r.Cfg.FleetDir
	}
	return filepath.Join(r.HomeDir, "DISENTANGLE-NETWORK", "fleet-deploy")
}

// FleetClusterDir returns <fleetDir>/clusters/<name>.
func (r *Resolver) FleetClusterDir(fleetDir, name string) string {
	return filepath.Join(fleetDir, "clusters", name)
}

// FleetSecretsDir returns <fleetDir>/secrets/<name>.
func (r *Resolver) FleetSecretsDir(fleetDir, name string) string {
	return filepath.Join(fleetDir, "secrets", name)
}

// InfraDir returns the infrastructure repo directory.
// Priority: override flag > config infra_dir > config repos.k8s_oci_foundation > empty.
func (r *Resolver) InfraDir(override string) string {
	if override != "" {
		return override
	}
	if r.Cfg != nil {
		if r.Cfg.InfraDir != "" {
			return r.Cfg.InfraDir
		}
		if r.Cfg.Repos.K8sOCIFoundation != "" {
			return r.Cfg.Repos.K8sOCIFoundation
		}
	}
	return ""
}

// InfraEnvDir returns <infraDir>/environments/<env>.
func (r *Resolver) InfraEnvDir(infraDir, env string) string {
	return filepath.Join(infraDir, "environments", env)
}

// OCIConfigPath returns ~/.oci/config.
func (r *Resolver) OCIConfigPath() string {
	return filepath.Join(r.HomeDir, ".oci", "config")
}
