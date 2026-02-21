package exec

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

type Result struct {
	Command  string
	ExitCode int
	Stdout   string
	Stderr   string
}

type Runner struct {
	Dir     string
	Env     []string
	Verbose bool
	DryRun  bool
	Stdout  io.Writer
	Stderr  io.Writer
}

func NewRunner() *Runner {
	return &Runner{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
}

// Run executes a command and streams output in real time.
func (r *Runner) Run(name string, args ...string) (*Result, error) {
	cmdStr := name + " " + strings.Join(args, " ")

	if r.DryRun {
		_, _ = fmt.Fprintf(r.Stdout, "[dry-run] %s\n", cmdStr)
		return &Result{Command: cmdStr, ExitCode: 0}, nil
	}

	if r.Verbose {
		_, _ = fmt.Fprintf(r.Stdout, "$ %s\n", cmdStr)
	}

	cmd := exec.Command(name, args...)
	if r.Dir != "" {
		cmd.Dir = r.Dir
	}
	if env := r.buildEnv(); env != nil {
		cmd.Env = env
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = io.MultiWriter(r.Stdout, &stdoutBuf)
	cmd.Stderr = io.MultiWriter(r.Stderr, &stderrBuf)

	err := cmd.Run()
	result := &Result{
		Command: cmdStr,
		Stdout:  stdoutBuf.String(),
		Stderr:  stderrBuf.String(),
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
		}
		return result, fmt.Errorf("command failed: %s: %w", cmdStr, err)
	}

	return result, nil
}

// RunSilent executes a command without streaming output (captures only).
func (r *Runner) RunSilent(name string, args ...string) (*Result, error) {
	cmdStr := name + " " + strings.Join(args, " ")

	if r.DryRun {
		return &Result{Command: cmdStr, ExitCode: 0}, nil
	}

	cmd := exec.Command(name, args...)
	if r.Dir != "" {
		cmd.Dir = r.Dir
	}
	if env := r.buildEnv(); env != nil {
		cmd.Env = env
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	result := &Result{
		Command: cmdStr,
		Stdout:  stdoutBuf.String(),
		Stderr:  stderrBuf.String(),
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
		}
		return result, fmt.Errorf("command failed: %s: %w", cmdStr, err)
	}

	return result, nil
}

// buildEnv merges r.Env into the current process environment.
// Runner values override inherited ones (last-set-wins).
func (r *Runner) buildEnv() []string {
	if len(r.Env) == 0 {
		return nil // nil means inherit
	}
	// Build a set of keys we're overriding
	overrides := make(map[string]bool, len(r.Env))
	for _, e := range r.Env {
		if k, _, ok := strings.Cut(e, "="); ok {
			overrides[k] = true
		}
	}
	// Keep inherited vars that aren't being overridden
	base := os.Environ()
	env := make([]string, 0, len(base)+len(r.Env))
	for _, e := range base {
		if k, _, ok := strings.Cut(e, "="); ok && overrides[k] {
			continue
		}
		env = append(env, e)
	}
	return append(env, r.Env...)
}

// SetDir sets the working directory for subsequent commands.
func (r *Runner) SetDir(dir string) { r.Dir = dir }

// SetEnv sets environment variables for subsequent commands.
func (r *Runner) SetEnv(env []string) { r.Env = env }

// GetDir returns the current working directory.
func (r *Runner) GetDir() string { return r.Dir }

// CommandExists checks if a command is available in PATH.
func CommandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
