package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/disentangle-network/launch/internal/config"
	"github.com/disentangle-network/launch/internal/paths"
	"github.com/disentangle-network/launch/internal/plugin"
	"github.com/spf13/cobra"
)

var pluginCmd = &cobra.Command{
	Use:   "plugin",
	Short: "Manage external plugins",
	Long:  "Discover and manage external launch-* plugins on PATH and in the plugin directory.",
}

var pluginListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List discovered plugins",
	Long:    "List all external plugins discovered on PATH and in ~/.config/launch/plugins.",
	Example: "  launch-disentangle plugin list",
	RunE:    runPluginList,
}

func init() {
	rootCmd.AddCommand(pluginCmd)
	pluginCmd.AddCommand(pluginListCmd)
}

// PluginListParams holds dependencies for PluginList.
type PluginListParams struct {
	Paths  *paths.Resolver
	Stdout io.Writer
}

// PluginList discovers and prints all available plugins.
func PluginList(p PluginListParams) error {
	dirs := pluginSearchDirs(p.Paths)
	plugins := plugin.Discover(dirs)

	if len(plugins) == 0 {
		fmt.Fprintln(p.Stdout, "No plugins found.")
		fmt.Fprintf(p.Stdout, "Install plugins by placing launch-* executables in PATH or %s\n", p.Paths.PluginDir())
		return nil
	}

	fmt.Fprintln(p.Stdout, "Plugins:")
	for _, pl := range plugins {
		fmt.Fprintf(p.Stdout, "  %-20s %s\n", pl.Name, pl.Path)
	}

	return nil
}

func runPluginList(cmd *cobra.Command, args []string) error {
	cfg, _ := config.Load(cfgFile)
	p := paths.NewWithHome("", cfg)
	if home, err := os.UserHomeDir(); err == nil {
		p = paths.NewWithHome(home, cfg)
	}

	return PluginList(PluginListParams{
		Paths:  p,
		Stdout: os.Stdout,
	})
}

// pluginSearchDirs returns the search directories for plugin discovery.
// Plugin directory comes first (highest priority), then PATH entries.
func pluginSearchDirs(p *paths.Resolver) []string {
	dirs := []string{p.PluginDir()}
	if pathEnv := os.Getenv("PATH"); pathEnv != "" {
		dirs = append(dirs, filepath.SplitList(pathEnv)...)
	}
	return dirs
}

// pluginCommand creates a cobra.Command that delegates to an external plugin binary.
func pluginCommand(pl plugin.Plugin) *cobra.Command {
	return &cobra.Command{
		Use:                pl.Name,
		Short:              fmt.Sprintf("Plugin: %s", pl.Name),
		DisableFlagParsing: true,
		SilenceUsage:       true,
		SilenceErrors:      true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPlugin(pl.Path, args)
		},
	}
}

// runPlugin executes an external plugin binary, forwarding args and
// connecting stdin/stdout/stderr.
func runPlugin(path string, args []string) error {
	pluginExec := exec.Command(path, args...) // #nosec G204
	pluginExec.Stdin = os.Stdin
	pluginExec.Stdout = os.Stdout
	pluginExec.Stderr = os.Stderr

	// Pass launch state via environment
	pluginExec.Env = os.Environ()
	if cfgFile != "" {
		pluginExec.Env = append(pluginExec.Env, "LAUNCH_CONFIG="+cfgFile)
	}
	if verbose {
		pluginExec.Env = append(pluginExec.Env, "LAUNCH_VERBOSE=1")
	}
	if dryRun {
		pluginExec.Env = append(pluginExec.Env, "LAUNCH_DRY_RUN=1")
	}

	err := pluginExec.Run()
	if err != nil {
		// Preserve the plugin's exit code
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				os.Exit(status.ExitStatus())
			}
		}
		return fmt.Errorf("plugin %s: %w", filepath.Base(path), err)
	}
	return nil
}
