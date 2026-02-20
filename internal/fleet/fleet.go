// Package fleet manages the fleet monorepo scaffold and cluster overlay generation.
package fleet

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

//go:embed templates/*
var templateFS embed.FS

// Presets maps resource size names to Helm values.
var Presets = map[string]ResourcePreset{
	"small": {
		CPULimit: "250m", MemoryLimit: "256Mi",
		CPURequest: "50m", MemoryRequest: "64Mi",
		PowDifficulty: 4,
	},
	"medium": {
		CPULimit: "500m", MemoryLimit: "512Mi",
		CPURequest: "100m", MemoryRequest: "128Mi",
		PowDifficulty: 8,
	},
	"large": {
		CPULimit: "2", MemoryLimit: "4Gi",
		CPURequest: "500m", MemoryRequest: "1Gi",
		PowDifficulty: 16,
	},
}

// ResourcePreset holds resource values for a deployment size.
type ResourcePreset struct {
	CPULimit      string
	MemoryLimit   string
	CPURequest    string
	MemoryRequest string
	PowDifficulty int
}

// ClusterConfig holds parameters for a cluster overlay.
type ClusterConfig struct {
	Name           string
	Arch           string // arm64, amd64
	Infra          string // cloud, bare-metal, local
	Nodes          int
	Resources      string // small, medium, large
	StorageClass   string
	NebulaMode     string // lighthouse, node, disabled
	NebulaPrefix   string // e.g. 10.42.0
	LighthouseAddr string // for non-lighthouse nodes
}

// InitFleetRepo scaffolds a new fleet monorepo at the given path.
func InitFleetRepo(outputDir, name string) error {
	dirs := []string{
		"clusters",
		"infrastructure/base",
		"infrastructure/overlays/cloud",
		"infrastructure/overlays/bare-metal",
		"infrastructure/overlays/local",
		"apps/base",
		"apps/overlays",
		"secrets",
	}

	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(outputDir, d), 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", d, err)
		}
	}

	// Copy embedded template files
	err := fs.WalkDir(templateFS, "templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		relPath := strings.TrimPrefix(path, "templates/")

		// Handle dot-prefixed files (embed can't have leading dots)
		relPath = strings.Replace(relPath, "dot-sops.yaml", ".sops.yaml", 1)
		relPath = strings.Replace(relPath, "dot-gitignore", ".gitignore", 1)

		destPath := filepath.Join(outputDir, relPath)
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}

		data, err := templateFS.ReadFile(path)
		if err != nil {
			return err
		}

		return os.WriteFile(destPath, data, 0644)
	})
	if err != nil {
		return fmt.Errorf("failed to copy templates: %w", err)
	}

	// Generate README
	readme := fmt.Sprintf(`# %s

Disentangle Network fleet repository. Managed by the launch CLI.

## Quick start

    launch cluster add <name> --nodes 3
    launch secrets init --cluster <name> --provider yubikey
    launch bootstrap --cluster <name>
    launch status

## Structure

    clusters/          Cluster-specific Kustomization CRs
    infrastructure/    Shared infrastructure (cert-manager, nebula-pq)
    apps/              Application deployments (Disentangle Helm chart)
    secrets/           SOPS-encrypted secrets (per-cluster)
`, name)

	if err := os.WriteFile(filepath.Join(outputDir, "README.md"), []byte(readme), 0644); err != nil {
		return fmt.Errorf("failed to write README: %w", err)
	}

	return nil
}

// AddCluster generates cluster overlay files in the fleet repo.
func AddCluster(fleetDir string, cfg ClusterConfig) error {
	preset, ok := Presets[cfg.Resources]
	if !ok {
		return fmt.Errorf("unknown resource preset: %s (valid: small, medium, large)", cfg.Resources)
	}

	// Create cluster directory
	clusterDir := filepath.Join(fleetDir, "clusters", cfg.Name)
	if err := os.MkdirAll(clusterDir, 0755); err != nil {
		return err
	}

	// Generate cluster-settings ConfigMap
	settingsTmpl := `apiVersion: v1
kind: ConfigMap
metadata:
  name: cluster-settings
  namespace: disentangle
data:
  cluster_name: "{{.Name}}"
  arch: "{{.Arch}}"
  infra: "{{.Infra}}"
  nodes: "{{.Nodes}}"
  resources: "{{.Resources}}"
  cpu_limit: "{{.CPULimit}}"
  memory_limit: "{{.MemoryLimit}}"
  cpu_request: "{{.CPURequest}}"
  memory_request: "{{.MemoryRequest}}"
  pow_difficulty: "{{.PowDifficulty}}"
  nebula_mode: "{{.NebulaMode}}"
  nebula_prefix: "{{.NebulaPrefix}}"
`
	settingsData := struct {
		ClusterConfig
		ResourcePreset
	}{cfg, preset}
	if err := renderTemplate(filepath.Join(clusterDir, "cluster-settings.yaml"), settingsTmpl, settingsData); err != nil {
		return err
	}

	// Generate infrastructure Kustomization CR
	infraKustomization := fmt.Sprintf(`apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: %s-infrastructure
  namespace: flux-system
spec:
  interval: 10m
  sourceRef:
    kind: GitRepository
    name: flux-system
  path: ./infrastructure/controllers
  prune: true
`, cfg.Name)

	if err := os.WriteFile(filepath.Join(clusterDir, "infrastructure.yaml"), []byte(infraKustomization), 0644); err != nil {
		return err
	}

	// Generate apps Kustomization CR
	appsKustomization := fmt.Sprintf(`apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: %s-apps
  namespace: flux-system
spec:
  interval: 5m
  sourceRef:
    kind: GitRepository
    name: flux-system
  path: ./apps/disentangle
  prune: true
  dependsOn:
    - name: %s-infrastructure
`, cfg.Name, cfg.Name)

	if err := os.WriteFile(filepath.Join(clusterDir, "apps.yaml"), []byte(appsKustomization), 0644); err != nil {
		return err
	}

	return nil
}

func renderTemplate(path, tmplStr string, data any) error {
	tmpl, err := template.New("").Parse(tmplStr)
	if err != nil {
		return fmt.Errorf("template parse error: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	return tmpl.Execute(f, data)
}
