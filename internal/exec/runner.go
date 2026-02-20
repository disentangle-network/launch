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
	if len(r.Env) > 0 {
		cmd.Env = append(os.Environ(), r.Env...)
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
	if len(r.Env) > 0 {
		cmd.Env = append(os.Environ(), r.Env...)
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

// CommandExists checks if a command is available in PATH.
func CommandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
