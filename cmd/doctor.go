package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/disentangle-network/launch/internal/config"
	"github.com/disentangle-network/launch/internal/exec"
	"github.com/disentangle-network/launch/internal/paths"
	"github.com/disentangle-network/launch/internal/preflight"
	"github.com/spf13/cobra"
)

var doctorFleetDir string

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose common issues with the deployment pipeline",
	Long: `Run a comprehensive health check that validates configuration,
tools, fleet repository state, cluster connectivity, secrets,
and mesh CA presence. Reports issues with actionable fix suggestions.`,
	RunE: runDoctor,
}

func init() {
	rootCmd.AddCommand(doctorCmd)
	doctorCmd.Flags().StringVar(&doctorFleetDir, "fleet-dir", "", "Path to fleet repository root")
}

// DoctorParams holds dependencies for the doctor command.
type DoctorParams struct {
	Exec        exec.Executor
	Paths       *paths.Resolver
	Stdout      io.Writer
	FleetDir    string
	CfgFile     string
	Verbose     bool
	ToolChecker func() DoctorCheck // optional; nil = use checkTools()
}

// DoctorCheck holds the result of a single diagnostic check.
type DoctorCheck struct {
	Name   string
	Status string // "ok", "warn", "fail"
	Detail string
	Fix    string // suggested fix command or action
}

// Doctor runs a comprehensive health check across all layers.
func Doctor(p DoctorParams) error {
	fmt.Fprintln(p.Stdout, "==> Doctor: checking deployment pipeline health...")
	fmt.Fprintln(p.Stdout)

	var checks []DoctorCheck

	// 1. Config file
	checks = append(checks, checkConfig(p))

	// 2. Required tools
	if p.ToolChecker != nil {
		checks = append(checks, p.ToolChecker())
	} else {
		checks = append(checks, checkTools())
	}

	// 3. Credentials
	checks = append(checks, checkCredentials(p))

	// 4. Fleet repo
	fleetDir := p.Paths.FleetDir(p.FleetDir)
	fleetCheck, clusterNames := checkFleetRepo(fleetDir)
	checks = append(checks, fleetCheck)

	// 5. Clusters
	checks = append(checks, checkClusters(fleetDir, clusterNames))

	// 6. Secrets
	checks = append(checks, checkSecrets(fleetDir, clusterNames))

	// 7. Mesh CA
	checks = append(checks, checkMeshCA(p))

	// Print results table
	for _, c := range checks {
		status := statusLabel(c.Status)
		fmt.Fprintf(p.Stdout, "  %-16s %-6s %s\n", c.Name, status, c.Detail)
	}

	// Collect fixes
	var fixes []string
	for _, c := range checks {
		if c.Fix != "" && c.Status != "ok" {
			fixes = append(fixes, c.Fix)
		}
	}

	if len(fixes) > 0 {
		fmt.Fprintln(p.Stdout)
		fmt.Fprintln(p.Stdout, "Suggested fixes:")
		for i, f := range fixes {
			fmt.Fprintf(p.Stdout, "  %d. %s\n", i+1, f)
		}
	}

	// Summary
	passed, warnings, failures := 0, 0, 0
	for _, c := range checks {
		switch c.Status {
		case "ok":
			passed++
		case "warn":
			warnings++
		case "fail":
			failures++
		}
	}

	fmt.Fprintln(p.Stdout)
	fmt.Fprintf(p.Stdout, "%d checks passed, %d warnings, %d failures.\n", passed, warnings, failures)

	if failures > 0 {
		return fmt.Errorf("doctor found %d failing check(s)", failures)
	}
	return nil
}

func statusLabel(status string) string {
	switch status {
	case "ok":
		return "OK"
	case "warn":
		return "WARN"
	case "fail":
		return "FAIL"
	default:
		return "????"
	}
}

func checkConfig(p DoctorParams) DoctorCheck {
	check := DoctorCheck{Name: "Config"}

	cfgPath := p.CfgFile
	if cfgPath == "" {
		cfgPath = p.Paths.ConfigPath()
	}

	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		check.Status = "fail"
		check.Detail = fmt.Sprintf("%s not found", cfgPath)
		check.Fix = "launch-disentangle setup"
		return check
	}

	_, err := config.Load(p.CfgFile)
	if err != nil {
		check.Status = "fail"
		check.Detail = fmt.Sprintf("failed to load: %v", err)
		check.Fix = "launch-disentangle setup"
		return check
	}

	check.Status = "ok"
	check.Detail = fmt.Sprintf("%s loaded", cfgPath)
	return check
}

func checkTools() DoctorCheck {
	check := DoctorCheck{Name: "Tools"}

	results := preflight.CheckTools()
	available := 0
	for _, r := range results {
		if r.Available {
			available++
		}
	}

	total := len(results)
	if available == total {
		check.Status = "ok"
		check.Detail = fmt.Sprintf("%d/%d required tools available", available, total)
	} else {
		missing := total - available
		allRequiredPresent := preflight.HasAllRequired(results)
		if allRequiredPresent {
			check.Status = "warn"
			check.Detail = fmt.Sprintf("%d/%d tools available (%d optional missing)", available, total, missing)
		} else {
			check.Status = "fail"
			check.Detail = fmt.Sprintf("%d/%d tools available (%d missing)", available, total, missing)
		}
		check.Fix = "launch-disentangle preflight"
	}

	return check
}

func checkCredentials(p DoctorParams) DoctorCheck {
	check := DoctorCheck{Name: "Credentials"}

	results := preflight.CheckCredentials(p.Exec)
	valid := 0
	for _, c := range results {
		if c.Status == "ok" {
			valid++
		}
	}

	total := len(results)
	if valid == total {
		check.Status = "ok"
		check.Detail = fmt.Sprintf("%d/%d credentials valid", valid, total)
	} else {
		missing := total - valid
		check.Status = "warn"
		check.Detail = fmt.Sprintf("%d/%d credentials valid (%d missing)", valid, total, missing)
		check.Fix = "launch-disentangle setup"
	}

	return check
}

// checkFleetRepo checks fleet directory presence, .git, and apps/base.
// Returns the check result and the list of cluster directory names found.
func checkFleetRepo(fleetDir string) (DoctorCheck, []string) {
	check := DoctorCheck{Name: "Fleet repo"}
	var clusterNames []string

	if _, err := os.Stat(fleetDir); os.IsNotExist(err) {
		check.Status = "fail"
		check.Detail = fmt.Sprintf("%s not found", fleetDir)
		check.Fix = "launch-disentangle fleet init"
		return check, nil
	}

	gitDir := filepath.Join(fleetDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		check.Status = "fail"
		check.Detail = fmt.Sprintf("%s exists but is not a git repository", fleetDir)
		check.Fix = "launch-disentangle fleet init"
		return check, nil
	}

	appsBase := filepath.Join(fleetDir, "apps", "base")
	if _, err := os.Stat(appsBase); os.IsNotExist(err) {
		check.Status = "warn"
		check.Detail = fmt.Sprintf("%s missing apps/base directory", fleetDir)
		check.Fix = "launch-disentangle fleet init"
		return check, nil
	}

	// Count clusters
	clustersDir := filepath.Join(fleetDir, "clusters")
	entries, err := os.ReadDir(clustersDir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				clusterNames = append(clusterNames, e.Name())
			}
		}
	}

	check.Status = "ok"
	check.Detail = fmt.Sprintf("%s (%d clusters)", fleetDir, len(clusterNames))
	return check, clusterNames
}

func checkClusters(fleetDir string, clusterNames []string) DoctorCheck {
	check := DoctorCheck{Name: "Clusters"}

	if len(clusterNames) == 0 {
		check.Status = "warn"
		check.Detail = "no clusters configured"
		check.Fix = "launch-disentangle cluster add <name>"
		return check
	}

	configured := 0
	for _, name := range clusterNames {
		settingsPath := filepath.Join(fleetDir, "clusters", name, "cluster-settings.yaml")
		if _, err := os.Stat(settingsPath); err == nil {
			configured++
		}
	}

	total := len(clusterNames)
	if configured == total {
		check.Status = "ok"
		check.Detail = fmt.Sprintf("%d/%d clusters have settings", configured, total)
	} else {
		check.Status = "warn"
		check.Detail = fmt.Sprintf("%d/%d clusters have settings", configured, total)
		check.Fix = "launch-disentangle cluster add <name>"
	}

	return check
}

func checkSecrets(fleetDir string, clusterNames []string) DoctorCheck {
	check := DoctorCheck{Name: "Secrets"}

	if len(clusterNames) == 0 {
		check.Status = "warn"
		check.Detail = "no clusters to check"
		check.Fix = "launch-disentangle cluster add <name>"
		return check
	}

	configured := 0
	var unconfigured []string
	for _, name := range clusterNames {
		genesisPath := filepath.Join(fleetDir, "secrets", name, "genesis-config.yaml")
		if _, err := os.Stat(genesisPath); err == nil {
			configured++
		} else {
			unconfigured = append(unconfigured, name)
		}
	}

	total := len(clusterNames)
	if configured == total {
		check.Status = "ok"
		check.Detail = fmt.Sprintf("%d/%d clusters have secrets configured", configured, total)
	} else {
		check.Status = "warn"
		check.Detail = fmt.Sprintf("%d/%d clusters have secrets configured", configured, total)
		if len(unconfigured) > 0 {
			check.Fix = fmt.Sprintf("launch-disentangle secrets init --cluster %s", unconfigured[0])
		}
	}

	return check
}

func checkMeshCA(p DoctorParams) DoctorCheck {
	check := DoctorCheck{Name: "Mesh CA"}

	caKeyPath := p.Paths.NebulaCAKey()
	if _, err := os.Stat(caKeyPath); os.IsNotExist(err) {
		check.Status = "fail"
		check.Detail = "No CA found"
		check.Fix = "launch-disentangle mesh init"
		return check
	}

	check.Status = "ok"
	check.Detail = fmt.Sprintf("CA key present at %s", caKeyPath)
	return check
}

func runDoctor(cmd *cobra.Command, args []string) error {
	runner := exec.NewRunner()
	runner.Verbose = verbose
	runner.DryRun = dryRun
	cfg, _ := config.Load(cfgFile)
	p := paths.NewWithHome("", cfg)
	if home, err := os.UserHomeDir(); err == nil {
		p = paths.NewWithHome(home, cfg)
	}

	return Doctor(DoctorParams{
		Exec:     runner,
		Paths:    p,
		Stdout:   os.Stdout,
		FleetDir: doctorFleetDir,
		CfgFile:  cfgFile,
		Verbose:  verbose,
	})
}
