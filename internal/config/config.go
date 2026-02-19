package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Repos struct {
	OCITFBootstrap   string `yaml:"oci_tf_bootstrap"`
	K8sOCIFoundation string `yaml:"k8s_oci_foundation"`
	GenesisOperator  string `yaml:"genesis_operator"`
	Deploy           string `yaml:"deploy"`
}

type Config struct {
	// OCI
	OCIRegion        string `yaml:"oci_region"`
	OCICompartmentID string `yaml:"oci_compartment_id"`

	// Cloudflare
	CloudflareAccountID string `yaml:"cloudflare_account_id"`
	Domain              string `yaml:"domain"`

	// GitHub
	GitHubOrg  string `yaml:"github_org"`
	GitHubUser string `yaml:"github_user"`

	// Repos
	Repos Repos `yaml:"repos"`

	// Cluster
	ClusterName string `yaml:"cluster_name"`
	Environment string `yaml:"environment"`

	// Images
	ProtocolImage   string `yaml:"protocol_image"`
	ProtocolVersion string `yaml:"protocol_version"`
	GenesisImage    string `yaml:"genesis_image"`
	GenesisVersion  string `yaml:"genesis_version"`
}

func DefaultConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, ".config", "launch"), nil
}

func DefaultConfigPath() (string, error) {
	dir, err := DefaultConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// Load reads config from the given path, or searches default locations.
// Priority: explicit path > .launch.yaml (cwd) > ~/.config/launch/config.yaml
func Load(path string) (*Config, error) {
	if path != "" {
		return loadFromFile(path)
	}

	// Check local .launch.yaml
	if _, err := os.Stat(".launch.yaml"); err == nil {
		return loadFromFile(".launch.yaml")
	}

	// Check default location
	defaultPath, err := DefaultConfigPath()
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(defaultPath); err == nil {
		return loadFromFile(defaultPath)
	}

	// Return empty config if no file found
	return &Config{}, nil
}

func loadFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}
	return &cfg, nil
}

// Save writes config to the given path (creates parent dirs).
func Save(cfg *Config, path string) error {
	if path == "" {
		var err error
		path, err = DefaultConfigPath()
		if err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing config %s: %w", path, err)
	}
	return nil
}
