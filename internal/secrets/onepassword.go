package secrets

import (
	"fmt"
	"strings"

	"github.com/disentangle-network/launch/internal/exec"
)

func OpRead(ref string) (string, error) {
	if !exec.CommandExists("op") {
		return "", fmt.Errorf("1Password CLI (op) not installed")
	}

	runner := exec.NewRunner()
	result, err := runner.RunSilent("op", "read", ref)
	if err != nil {
		return "", fmt.Errorf("op read %s failed: %w", ref, err)
	}

	val := strings.TrimSpace(result.Stdout)
	if val == "" {
		return "", fmt.Errorf("op read %s returned empty value", ref)
	}
	return val, nil
}
