package cmd

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/disentangle-network/launch/internal/config"
	"github.com/disentangle-network/launch/internal/exec"
	"github.com/disentangle-network/launch/internal/hints"
	"github.com/disentangle-network/launch/internal/paths"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var meshCmd = &cobra.Command{
	Use:   "mesh",
	Short: "Manage the nebula-pq overlay mesh",
	Long:  "Generate PQ certificates and manage the nebula mesh across clusters.",
}

var meshCAOutput string

var meshInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate a new nebula-pq CA certificate",
	Long: `Generate a post-quantum CA certificate using ML-DSA-87.
The CA key is stored locally and must never be committed to git.`,
	RunE: runMeshInit,
}

var (
	meshCluster      string
	meshIsLighthouse bool
	meshLighthouse   string
	meshFleetDir     string
)

var meshAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Generate nebula host certificates for a cluster",
	Long: `Generate post-quantum host certificates for each node in a cluster,
generate K8s Secret manifests and Helm values overlays, and wire lighthouse
addresses into the nebula configuration.`,
	Example: `  launch-disentangle mesh add --cluster edge-1
  launch-disentangle mesh add --cluster edge-1 --lighthouse 10.42.0.1=lighthouse.example.com:4242`,
	RunE: runMeshAdd,
}

func init() {
	rootCmd.AddCommand(meshCmd)
	meshCmd.AddCommand(meshInitCmd)
	meshCmd.AddCommand(meshAddCmd)

	meshInitCmd.Flags().StringVar(&meshCAOutput, "ca-output", "", "Output directory for CA files (default: ~/.config/disentangle/nebula-ca/)")

	meshAddCmd.Flags().StringVar(&meshCluster, "cluster", "", "Cluster name (required)")
	meshAddCmd.Flags().BoolVar(&meshIsLighthouse, "is-lighthouse", false, "This cluster runs a lighthouse node")
	meshAddCmd.Flags().StringVar(&meshLighthouse, "lighthouse", "", "Lighthouse VPN IP and real address (vpnIP=host:port)")
	meshAddCmd.Flags().StringVar(&meshFleetDir, "fleet-dir", ".", "Path to fleet repository root")
	_ = meshAddCmd.MarkFlagRequired("cluster")
}

// MeshInitParams holds all dependencies for MeshInit.
type MeshInitParams struct {
	Exec     exec.Executor
	Paths    *paths.Resolver
	Stdout   io.Writer
	CAOutput string // flag override for CA output directory
	CfgFile  string // config file path
}

// MeshInit generates a new nebula-pq CA certificate.
func MeshInit(p MeshInitParams) error {
	caDir := p.CAOutput
	if caDir == "" {
		caDir = p.Paths.NebulaCADir()
	}

	if err := os.MkdirAll(caDir, 0700); err != nil {
		return err
	}

	caKeyPath := filepath.Join(caDir, "ca.key")
	caCrtPath := filepath.Join(caDir, "ca.crt")

	if _, err := os.Stat(caKeyPath); err == nil {
		return fmt.Errorf("CA key already exists at %s (remove to regenerate)", caKeyPath)
	}

	// Use domain from config for CA name, fall back to github_org
	caName := "disentangle"
	cfg, _ := config.Load(p.CfgFile)
	if cfg != nil {
		if cfg.Domain != "" {
			caName = cfg.Domain
		} else if cfg.GitHubOrg != "" {
			caName = cfg.GitHubOrg
		}
	}

	fmt.Fprintln(p.Stdout, "Generating nebula-pq CA certificate (ML-DSA-87)...")

	_, err := p.Exec.Run("nebula-cert", "ca",
		"-curve", "PQ",
		"-name", caName,
		"-out-key", caKeyPath,
		"-out-crt", caCrtPath,
	)
	if err != nil {
		return fmt.Errorf("failed to generate CA: %w", err)
	}

	fmt.Fprintf(p.Stdout, "\nCA certificate generated:\n")
	fmt.Fprintf(p.Stdout, "  Key:  %s (keep secret, never commit to git)\n", caKeyPath)
	fmt.Fprintf(p.Stdout, "  Cert: %s\n", caCrtPath)
	hints.Fprint(p.Stdout, []hints.NextStep{
		{Command: "mesh add --cluster <name>", Description: "Generate host certs for a cluster"},
	})

	return nil
}

// MeshAddParams holds all dependencies for MeshAdd.
type MeshAddParams struct {
	Exec         exec.Executor
	Paths        *paths.Resolver
	Stdout       io.Writer
	Cluster      string
	FleetDir     string
	IsLighthouse bool
	Lighthouse   string // format: "vpnIP=host:port"
}

// parseLighthouse splits "10.42.0.1=lighthouse.example.com:4242" into vpnIP and realAddr.
func parseLighthouse(s string) (vpnIP, realAddr string, err error) {
	parts := strings.SplitN(s, "=", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid lighthouse format %q (expected vpnIP=host:port)", s)
	}
	return parts[0], parts[1], nil
}

// meshCertFile tracks a generated cert/key pair for Secret manifest generation.
type meshCertFile struct {
	name string
	key  string
	crt  string
}

// clusterSettingsConfigMap represents the Kubernetes ConfigMap structure
// used in cluster-settings.yaml files.
type clusterSettingsConfigMap struct {
	Data struct {
		Nodes        string `yaml:"nodes"`
		NebulaPrefix string `yaml:"nebula_prefix"`
	} `yaml:"data"`
}

// MeshAdd generates nebula host certificates for each node in a cluster,
// writes a K8s Secret manifest, and generates a Helm values overlay.
func MeshAdd(p MeshAddParams) error {
	caKeyPath := p.Paths.NebulaCAKey()
	caCrtPath := p.Paths.NebulaCACert()

	if _, err := os.Stat(caKeyPath); os.IsNotExist(err) {
		return fmt.Errorf("no CA found at %s (run 'launch mesh init' first)", p.Paths.NebulaCADir())
	}

	// Parse lighthouse flag if provided
	var lighthouseVPNIP, lighthouseRealAddr string
	if p.Lighthouse != "" {
		var err error
		lighthouseVPNIP, lighthouseRealAddr, err = parseLighthouse(p.Lighthouse)
		if err != nil {
			return err
		}
	}

	// Read cluster settings to get node count and nebula prefix
	clusterDir := p.Paths.FleetClusterDir(p.FleetDir, p.Cluster)
	settingsPath := filepath.Join(clusterDir, "cluster-settings.yaml")
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		return fmt.Errorf("cluster '%s' not found (run 'launch cluster add %s' first)", p.Cluster, p.Cluster)
	}

	// Parse cluster-settings.yaml to get node count
	settingsData, err := os.ReadFile(filepath.Clean(settingsPath))
	if err != nil {
		return fmt.Errorf("reading cluster settings: %w", err)
	}

	var settings clusterSettingsConfigMap
	if err := yaml.Unmarshal(settingsData, &settings); err != nil {
		return fmt.Errorf("parsing cluster settings: %w", err)
	}

	nodeCount := 1
	if settings.Data.Nodes != "" {
		n, err := strconv.Atoi(settings.Data.Nodes)
		if err != nil {
			return fmt.Errorf("invalid node count %q in cluster settings: %w", settings.Data.Nodes, err)
		}
		nodeCount = n
	}

	nebulaPrefix := "10.42.0"
	if settings.Data.NebulaPrefix != "" {
		nebulaPrefix = settings.Data.NebulaPrefix
	}

	// Create secrets directory for this cluster
	secretsDir := p.Paths.FleetSecretsDir(p.FleetDir, p.Cluster)
	if err := os.MkdirAll(secretsDir, 0750); err != nil {
		return err
	}

	fmt.Fprintf(p.Stdout, "Generating nebula-pq host certificates for cluster '%s' (%d nodes)...\n", p.Cluster, nodeCount)

	groups := "disentangle"
	if p.IsLighthouse {
		groups = "disentangle,lighthouse"
	}

	// Track cert files for the K8s Secret manifest
	var certs []meshCertFile

	for i := 1; i <= nodeCount; i++ {
		certName := fmt.Sprintf("%s-node%d", p.Cluster, i)
		certKeyPath := filepath.Join(secretsDir, fmt.Sprintf("%s.key", certName))
		certCrtPath := filepath.Join(secretsDir, fmt.Sprintf("%s.crt", certName))
		ip := fmt.Sprintf("%s.%d/16", nebulaPrefix, i)

		_, err := p.Exec.Run("nebula-cert", "sign",
			"-ca-key", caKeyPath,
			"-ca-crt", caCrtPath,
			"-name", certName,
			"-networks", ip,
			"-groups", groups,
			"-out-key", certKeyPath,
			"-out-crt", certCrtPath,
		)
		if err != nil {
			return fmt.Errorf("failed to generate host cert for %s: %w", certName, err)
		}

		certs = append(certs, meshCertFile{name: certName, key: certKeyPath, crt: certCrtPath})
	}

	// Generate K8s Secret manifest
	if err := writeCertSecret(caCrtPath, certs, secretsDir); err != nil {
		return fmt.Errorf("writing cert secret manifest: %w", err)
	}

	// Generate values overlay
	mode := "node"
	if p.IsLighthouse {
		mode = "lighthouse"
	}
	if err := writeValuesOverlay(clusterDir, mode, lighthouseVPNIP, lighthouseRealAddr); err != nil {
		return fmt.Errorf("writing values overlay: %w", err)
	}

	fmt.Fprintf(p.Stdout, "\nHost certificates generated in %s\n", secretsDir)
	fmt.Fprintf(p.Stdout, "Secret manifest: %s\n", filepath.Join(secretsDir, "nebula-certs.yaml"))
	fmt.Fprintf(p.Stdout, "Values overlay:  %s\n", filepath.Join(clusterDir, "nebula-values.yaml"))
	hints.Fprint(p.Stdout, []hints.NextStep{
		{Command: "bootstrap --cluster " + p.Cluster, Description: "Bootstrap FluxCD"},
		{Command: "status", Description: "Check deployment health"},
	})

	return nil
}

// writeCertSecret writes a K8s Secret manifest containing the CA cert and
// per-node certs/keys as base64-encoded data entries.
func writeCertSecret(caCrtPath string, certs []meshCertFile, secretsDir string) error {
	caCrtData, err := os.ReadFile(filepath.Clean(caCrtPath))
	if err != nil {
		return fmt.Errorf("reading CA cert: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("apiVersion: v1\nkind: Secret\nmetadata:\n  name: nebula-certs\n  namespace: disentangle\ntype: Opaque\ndata:\n")
	sb.WriteString(fmt.Sprintf("  ca.crt: %s\n", base64.StdEncoding.EncodeToString(caCrtData)))

	for _, c := range certs {
		crtData, err := os.ReadFile(filepath.Clean(c.crt))
		if err != nil {
			return fmt.Errorf("reading cert %s: %w", c.crt, err)
		}
		keyData, err := os.ReadFile(filepath.Clean(c.key))
		if err != nil {
			return fmt.Errorf("reading key %s: %w", c.key, err)
		}
		sb.WriteString(fmt.Sprintf("  %s.crt: %s\n", c.name, base64.StdEncoding.EncodeToString(crtData)))
		sb.WriteString(fmt.Sprintf("  %s.key: %s\n", c.name, base64.StdEncoding.EncodeToString(keyData)))
	}

	return os.WriteFile(filepath.Join(secretsDir, "nebula-certs.yaml"), []byte(sb.String()), 0600)
}

// writeValuesOverlay writes a Helm values overlay for the nebula chart.
func writeValuesOverlay(clusterDir, mode, lighthouseVPNIP, lighthouseRealAddr string) error {
	values := map[string]any{
		"nebula": map[string]any{
			"enabled": true,
			"mode":    mode,
		},
	}

	nebulaMap := values["nebula"].(map[string]any)

	if lighthouseVPNIP != "" && lighthouseRealAddr != "" {
		nebulaMap["staticHostMap"] = map[string]string{
			lighthouseVPNIP: lighthouseRealAddr,
		}
		nebulaMap["relay"] = map[string]any{
			"relays": []string{lighthouseVPNIP},
		}
	}

	out, err := yaml.Marshal(values)
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(clusterDir, "nebula-values.yaml"), out, 0644)
}

func runMeshInit(cmd *cobra.Command, args []string) error {
	runner := exec.NewRunner()
	cfg, _ := config.Load(cfgFile)
	p := paths.NewWithHome("", cfg)
	if home, err := os.UserHomeDir(); err == nil {
		p = paths.NewWithHome(home, cfg)
	}

	return MeshInit(MeshInitParams{
		Exec:     runner,
		Paths:    p,
		Stdout:   os.Stdout,
		CAOutput: meshCAOutput,
		CfgFile:  cfgFile,
	})
}

func runMeshAdd(cmd *cobra.Command, args []string) error {
	runner := exec.NewRunner()
	cfg, _ := config.Load(cfgFile)
	p := paths.NewWithHome("", cfg)
	if home, err := os.UserHomeDir(); err == nil {
		p = paths.NewWithHome(home, cfg)
	}

	return MeshAdd(MeshAddParams{
		Exec:         runner,
		Paths:        p,
		Stdout:       os.Stdout,
		Cluster:      meshCluster,
		FleetDir:     meshFleetDir,
		IsLighthouse: meshIsLighthouse,
		Lighthouse:   meshLighthouse,
	})
}
