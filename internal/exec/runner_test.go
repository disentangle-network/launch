package exec

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunEchoCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	r := &Runner{
		Stdout: &stdout,
		Stderr: &stderr,
	}

	result, err := r.Run("echo", "hello", "world")
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}

	expected := "hello world\n"
	if result.Stdout != expected {
		t.Errorf("result.Stdout = %q, want %q", result.Stdout, expected)
	}

	// Run streams to the writer, so the buffer should also have output.
	if stdout.String() != expected {
		t.Errorf("streamed stdout = %q, want %q", stdout.String(), expected)
	}

	if result.Command != "echo hello world" {
		t.Errorf("Command = %q, want %q", result.Command, "echo hello world")
	}
}

func TestRunVerboseMode(t *testing.T) {
	var stdout, stderr bytes.Buffer
	r := &Runner{
		Stdout:  &stdout,
		Stderr:  &stderr,
		Verbose: true,
	}

	_, err := r.Run("echo", "verbose-test")
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "$ echo verbose-test") {
		t.Errorf("verbose output should contain command prefix, got %q", output)
	}
	if !strings.Contains(output, "verbose-test\n") {
		t.Errorf("verbose output should contain command output, got %q", output)
	}
}

func TestRunWithDir(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	r := &Runner{
		Dir:    dir,
		Stdout: &stdout,
		Stderr: &stderr,
	}

	result, err := r.Run("pwd")
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	got := strings.TrimSpace(result.Stdout)
	if got != dir {
		t.Errorf("pwd in Dir = %q, want %q", got, dir)
	}
}

func TestRunFailingCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	r := &Runner{
		Stdout: &stdout,
		Stderr: &stderr,
	}

	result, err := r.Run("false")
	if err == nil {
		t.Fatal("Run(\"false\") should return an error")
	}

	if result.ExitCode == 0 {
		t.Error("ExitCode should be non-zero for failing command")
	}
}

func TestRunSilentCapturesOutput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	r := &Runner{
		Stdout: &stdout,
		Stderr: &stderr,
	}

	result, err := r.RunSilent("echo", "silent-test")
	if err != nil {
		t.Fatalf("RunSilent() returned error: %v", err)
	}

	expected := "silent-test\n"
	if result.Stdout != expected {
		t.Errorf("result.Stdout = %q, want %q", result.Stdout, expected)
	}

	// RunSilent should NOT stream to the runner's Stdout writer.
	if stdout.String() != "" {
		t.Errorf("RunSilent should not stream to Stdout writer, got %q", stdout.String())
	}
}

func TestRunSilentFailingCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	r := &Runner{
		Stdout: &stdout,
		Stderr: &stderr,
	}

	result, err := r.RunSilent("false")
	if err == nil {
		t.Fatal("RunSilent(\"false\") should return an error")
	}

	if result.ExitCode == 0 {
		t.Error("ExitCode should be non-zero for failing command")
	}
}

func TestDryRunDoesNotExecute(t *testing.T) {
	var stdout, stderr bytes.Buffer
	r := &Runner{
		DryRun: true,
		Stdout: &stdout,
		Stderr: &stderr,
	}

	result, err := r.Run("echo", "should-not-run")
	if err != nil {
		t.Fatalf("DryRun Run() returned error: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("DryRun ExitCode = %d, want 0", result.ExitCode)
	}

	output := stdout.String()
	if !strings.Contains(output, "[dry-run]") {
		t.Errorf("DryRun output should contain [dry-run], got %q", output)
	}
	if !strings.Contains(output, "echo should-not-run") {
		t.Errorf("DryRun output should contain the command, got %q", output)
	}

	// The actual echo output should NOT appear (only the dry-run prefix line).
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 1 {
		t.Errorf("DryRun should produce exactly 1 line, got %d: %q", len(lines), output)
	}
}

func TestDryRunSilent(t *testing.T) {
	var stdout, stderr bytes.Buffer
	r := &Runner{
		DryRun: true,
		Stdout: &stdout,
		Stderr: &stderr,
	}

	result, err := r.RunSilent("echo", "dry-silent")
	if err != nil {
		t.Fatalf("DryRun RunSilent() returned error: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("DryRun ExitCode = %d, want 0", result.ExitCode)
	}

	if result.Command != "echo dry-silent" {
		t.Errorf("Command = %q, want %q", result.Command, "echo dry-silent")
	}

	// RunSilent in dry-run should not produce any output.
	if stdout.String() != "" {
		t.Errorf("DryRun RunSilent should not write to stdout, got %q", stdout.String())
	}
}

func TestCommandExistsTrue(t *testing.T) {
	if !CommandExists("echo") {
		t.Error("CommandExists(\"echo\") = false, want true")
	}
}

func TestCommandExistsFalse(t *testing.T) {
	if CommandExists("nonexistent_binary_xyz_launch_test") {
		t.Error("CommandExists(\"nonexistent_binary_xyz_launch_test\") = true, want false")
	}
}

func TestNewRunner(t *testing.T) {
	r := NewRunner()
	if r == nil {
		t.Fatal("NewRunner() returned nil")
	}
	if r.Stdout == nil {
		t.Error("NewRunner().Stdout should not be nil")
	}
	if r.Stderr == nil {
		t.Error("NewRunner().Stderr should not be nil")
	}
	if r.DryRun {
		t.Error("NewRunner().DryRun should default to false")
	}
	if r.Verbose {
		t.Error("NewRunner().Verbose should default to false")
	}
}

func TestRunnerSetDir(t *testing.T) {
	r := NewRunner()
	r.SetDir("/tmp/test-dir")
	if r.GetDir() != "/tmp/test-dir" {
		t.Errorf("GetDir() = %q, want %q", r.GetDir(), "/tmp/test-dir")
	}
}

func TestRunnerSetEnv(t *testing.T) {
	r := NewRunner()
	r.SetEnv([]string{"FOO=bar", "BAZ=qux"})
	if len(r.Env) != 2 {
		t.Errorf("len(Env) = %d, want 2", len(r.Env))
	}
	if r.Env[0] != "FOO=bar" {
		t.Errorf("Env[0] = %q, want %q", r.Env[0], "FOO=bar")
	}
}

func TestRunnerGetDirEmpty(t *testing.T) {
	r := NewRunner()
	if r.GetDir() != "" {
		t.Errorf("GetDir() = %q, want empty", r.GetDir())
	}
}

func TestRunnerImplementsExecutor(t *testing.T) {
	var _ Executor = NewRunner()
}

func TestRunWithEnv(t *testing.T) {
	var stdout, stderr bytes.Buffer
	r := &Runner{
		Stdout: &stdout,
		Stderr: &stderr,
		Env:    []string{"LAUNCH_TEST_VAR=hello123"},
	}

	result, err := r.Run("sh", "-c", "echo $LAUNCH_TEST_VAR")
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	got := strings.TrimSpace(result.Stdout)
	if got != "hello123" {
		t.Errorf("env var output = %q, want %q", got, "hello123")
	}
}
