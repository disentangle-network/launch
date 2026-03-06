package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverFindsPlugins(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"launch-foo", "launch-bar"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	plugins := Discover([]string{dir})
	if len(plugins) != 2 {
		t.Fatalf("got %d plugins, want 2", len(plugins))
	}

	// os.ReadDir returns entries sorted by name, so bar comes before foo.
	if plugins[0].Name != "bar" {
		t.Errorf("plugins[0].Name = %q, want %q", plugins[0].Name, "bar")
	}
	if plugins[0].Path != filepath.Join(dir, "launch-bar") {
		t.Errorf("plugins[0].Path = %q, want %q", plugins[0].Path, filepath.Join(dir, "launch-bar"))
	}
	if plugins[1].Name != "foo" {
		t.Errorf("plugins[1].Name = %q, want %q", plugins[1].Name, "foo")
	}
	if plugins[1].Path != filepath.Join(dir, "launch-foo") {
		t.Errorf("plugins[1].Path = %q, want %q", plugins[1].Path, filepath.Join(dir, "launch-foo"))
	}
}

func TestDiscoverSkipsNonExecutable(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "launch-nope"), []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	plugins := Discover([]string{dir})
	if len(plugins) != 0 {
		t.Errorf("got %d plugins, want 0", len(plugins))
	}
}

func TestDiscoverSkipsDirectories(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "launch-subdir"), 0o755); err != nil {
		t.Fatal(err)
	}

	plugins := Discover([]string{dir})
	if len(plugins) != 0 {
		t.Errorf("got %d plugins, want 0", len(plugins))
	}
}

func TestDiscoverDeduplicates(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	for _, dir := range []string{dir1, dir2} {
		if err := os.WriteFile(filepath.Join(dir, "launch-dup"), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	plugins := Discover([]string{dir1, dir2})
	if len(plugins) != 1 {
		t.Fatalf("got %d plugins, want 1", len(plugins))
	}
	if plugins[0].Path != filepath.Join(dir1, "launch-dup") {
		t.Errorf("Path = %q, want from first dir %q", plugins[0].Path, filepath.Join(dir1, "launch-dup"))
	}
}

func TestDiscoverEmptyDir(t *testing.T) {
	dir := t.TempDir()

	plugins := Discover([]string{dir})
	if len(plugins) != 0 {
		t.Errorf("got %d plugins, want 0", len(plugins))
	}
}

func TestDiscoverNonexistentDir(t *testing.T) {
	plugins := Discover([]string{"/nonexistent/path"})
	if len(plugins) != 0 {
		t.Errorf("got %d plugins, want 0", len(plugins))
	}
}

func TestDiscoverMultipleDirs(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir1, "launch-alpha"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir2, "launch-beta"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	plugins := Discover([]string{dir1, dir2})
	if len(plugins) != 2 {
		t.Fatalf("got %d plugins, want 2", len(plugins))
	}

	names := map[string]bool{}
	for _, p := range plugins {
		names[p.Name] = true
	}
	if !names["alpha"] {
		t.Error("missing plugin alpha")
	}
	if !names["beta"] {
		t.Error("missing plugin beta")
	}
}

func TestDiscoverSkipsNonPrefixed(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"other-tool", "notlaunch-foo"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	plugins := Discover([]string{dir})
	if len(plugins) != 0 {
		t.Errorf("got %d plugins, want 0", len(plugins))
	}
}
