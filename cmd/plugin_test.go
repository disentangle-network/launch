package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/disentangle-network/launch/internal/paths"
	"github.com/disentangle-network/launch/internal/plugin"
)

func TestPluginListNoPlugins(t *testing.T) {
	tmp := t.TempDir()
	// Override PATH so no real launch-* binaries are discovered.
	t.Setenv("PATH", tmp)

	p := paths.NewWithHome(tmp, nil)
	var buf bytes.Buffer

	err := PluginList(PluginListParams{
		Paths:  p,
		Stdout: &buf,
	})
	if err != nil {
		t.Fatalf("PluginList() returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "No plugins found") {
		t.Errorf("expected 'No plugins found', got %q", buf.String())
	}
}

func TestPluginListWithPlugins(t *testing.T) {
	tmp := t.TempDir()
	pluginDir := filepath.Join(tmp, ".config", "launch", "plugins")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Create a fake plugin
	if err := os.WriteFile(filepath.Join(pluginDir, "launch-test-tool"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	p := paths.NewWithHome(tmp, nil)
	var buf bytes.Buffer

	err := PluginList(PluginListParams{
		Paths:  p,
		Stdout: &buf,
	})
	if err != nil {
		t.Fatalf("PluginList() returned error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "Plugins:") {
		t.Errorf("expected 'Plugins:' header, got %q", output)
	}
	if !strings.Contains(output, "test-tool") {
		t.Errorf("expected 'test-tool' in output, got %q", output)
	}
}

func TestPluginSearchDirs(t *testing.T) {
	p := paths.NewWithHome("/home/test", nil)
	dirs := pluginSearchDirs(p)
	if len(dirs) == 0 {
		t.Fatal("pluginSearchDirs() returned empty")
	}
	// First entry should be the plugin directory
	want := filepath.Join("/home/test", ".config", "launch", "plugins")
	if dirs[0] != want {
		t.Errorf("pluginSearchDirs()[0] = %q, want %q", dirs[0], want)
	}
	// Should also include PATH entries
	if len(dirs) < 2 {
		t.Error("pluginSearchDirs() should include PATH entries")
	}
}

func TestPluginCommand(t *testing.T) {
	pl := plugin.Plugin{Name: "test-plug", Path: "/usr/local/bin/launch-test-plug"}
	cmd := pluginCommand(pl)
	if cmd.Use != "test-plug" {
		t.Errorf("pluginCommand().Use = %q, want %q", cmd.Use, "test-plug")
	}
	if !cmd.DisableFlagParsing {
		t.Error("pluginCommand() should have DisableFlagParsing = true")
	}
}

func TestRunPluginSuccess(t *testing.T) {
	tmp := t.TempDir()
	script := filepath.Join(tmp, "launch-hello")
	content := "#!/bin/sh\necho hello from plugin\n"
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}

	err := runPlugin(script, []string{})
	if err != nil {
		t.Fatalf("runPlugin() returned error: %v", err)
	}
}

func TestRunPluginWithArgs(t *testing.T) {
	tmp := t.TempDir()
	script := filepath.Join(tmp, "launch-echo")
	content := "#!/bin/sh\necho \"$@\"\n"
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}

	err := runPlugin(script, []string{"arg1", "arg2"})
	if err != nil {
		t.Fatalf("runPlugin() returned error: %v", err)
	}
}

func TestRunPluginNotFound(t *testing.T) {
	err := runPlugin("/nonexistent/launch-fake", []string{})
	if err == nil {
		t.Fatal("expected error for nonexistent plugin")
	}
}
