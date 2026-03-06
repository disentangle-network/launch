// Package plugin discovers external plugin executables following the
// git/kubectl convention of naming binaries "launch-<name>".
package plugin

import (
	"os"
	"path/filepath"
	"strings"
)

// Plugin represents a discovered external plugin.
type Plugin struct {
	Name string // e.g. "deploy" (from "launch-deploy" binary)
	Path string // absolute path to the executable
}

// Discover scans the given directories for executables named "launch-*".
// First match wins when the same plugin name appears in multiple dirs
// (earlier directories have higher priority).
// Non-executable entries and directories are skipped.
// Directories that don't exist or can't be read are silently skipped.
func Discover(dirs []string) []Plugin {
	seen := make(map[string]bool)
	var plugins []Plugin

	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if !strings.HasPrefix(name, "launch-") {
				continue
			}
			pluginName := strings.TrimPrefix(name, "launch-")
			if seen[pluginName] {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				continue
			}
			if info.Mode()&0111 == 0 {
				continue
			}
			seen[pluginName] = true
			plugins = append(plugins, Plugin{
				Name: pluginName,
				Path: filepath.Join(dir, name),
			})
		}
	}

	return plugins
}
