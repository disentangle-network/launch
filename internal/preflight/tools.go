package preflight

import (
	"fmt"
	"strings"

	"github.com/disentangle-network/launch/internal/exec"
)

type ToolCheck struct {
	Name     string
	Command  string
	Args     []string
	Required bool
}

type ToolResult struct {
	Name      string
	Available bool
	Version   string
	Required  bool
	Error     string
}

var RequiredTools = []ToolCheck{
	{Name: "oci", Command: "oci", Args: []string{"--version"}, Required: true},
	{Name: "terraform", Command: "terraform", Args: []string{"--version"}, Required: true},
	{Name: "kubectl", Command: "kubectl", Args: []string{"version", "--client", "--short"}, Required: true},
	{Name: "helm", Command: "helm", Args: []string{"version", "--short"}, Required: true},
	{Name: "flux", Command: "flux", Args: []string{"--version"}, Required: true},
	{Name: "gh", Command: "gh", Args: []string{"--version"}, Required: true},
	{Name: "task", Command: "task", Args: []string{"--version"}, Required: true},
	{Name: "sops", Command: "sops", Args: []string{"--version"}, Required: false},
	{Name: "age", Command: "age", Args: []string{"--version"}, Required: false},
	{Name: "genesis", Command: "genesis", Args: []string{"version"}, Required: false},
}

func CheckTools() []ToolResult {
	runner := exec.NewRunner()
	var results []ToolResult

	for _, tool := range RequiredTools {
		result := ToolResult{
			Name:     tool.Name,
			Required: tool.Required,
		}

		if !exec.CommandExists(tool.Command) {
			result.Available = false
			result.Error = "not found in PATH"
			results = append(results, result)
			continue
		}

		out, err := runner.RunSilent(tool.Command, tool.Args...)
		if err != nil {
			result.Available = true
			result.Version = "unknown"
			result.Error = fmt.Sprintf("version check failed: %v", err)
		} else {
			result.Available = true
			version := strings.TrimSpace(out.Stdout)
			if version == "" {
				version = strings.TrimSpace(out.Stderr)
			}
			// Take first line only
			if idx := strings.IndexByte(version, '\n'); idx >= 0 {
				version = version[:idx]
			}
			result.Version = version
		}

		results = append(results, result)
	}

	return results
}

func HasAllRequired(results []ToolResult) bool {
	for _, r := range results {
		if r.Required && !r.Available {
			return false
		}
	}
	return true
}

func MissingRequired(results []ToolResult) []string {
	var missing []string
	for _, r := range results {
		if r.Required && !r.Available {
			missing = append(missing, r.Name)
		}
	}
	return missing
}
