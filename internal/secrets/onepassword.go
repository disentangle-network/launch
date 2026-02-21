package secrets

import (
	"fmt"
	"strings"

	"github.com/disentangle-network/launch/internal/exec"
)

// commandExistsFunc is the function used to check if a command exists.
// It defaults to exec.CommandExists and can be overridden in tests.
var commandExistsFunc = exec.CommandExists

// newExecutorFunc is the function used to create a new Executor.
// It defaults to creating exec.NewRunner and can be overridden in tests.
var newExecutorFunc = func() exec.Executor {
	return exec.NewRunner()
}

// OpRead reads a secret reference from 1Password using the op CLI.
func OpRead(ref string) (string, error) {
	return opReadWith(ref, newExecutorFunc(), commandExistsFunc)
}

// opReadWith is the testable core that accepts injected dependencies.
func opReadWith(ref string, runner exec.Executor, cmdExists func(string) bool) (string, error) {
	if !cmdExists("op") {
		return "", fmt.Errorf("1Password CLI (op) not installed")
	}

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
