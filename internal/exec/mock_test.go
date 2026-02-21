package exec

import (
	"fmt"
	"testing"
)

func TestMockExecutorRun(t *testing.T) {
	m := NewMockExecutor()
	m.ExpectRun("echo hello", "hello\n", nil)

	result, err := m.Run("echo", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Stdout != "hello\n" {
		t.Errorf("Stdout = %q, want %q", result.Stdout, "hello\n")
	}
	m.AssertCallCount(t, 1)
	m.AssertCalled(t, "echo hello")
}

func TestMockExecutorRunSilent(t *testing.T) {
	m := NewMockExecutor()
	m.ExpectRun("kubectl get pods", "NAME\npod-1\n", nil)

	result, err := m.RunSilent("kubectl", "get", "pods")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Stdout != "NAME\npod-1\n" {
		t.Errorf("Stdout = %q, want %q", result.Stdout, "NAME\npod-1\n")
	}
	if m.Calls[0].Method != "RunSilent" {
		t.Errorf("Method = %q, want %q", m.Calls[0].Method, "RunSilent")
	}
}

func TestMockExecutorDefaultResponse(t *testing.T) {
	m := NewMockExecutor()

	result, err := m.Run("unknown", "cmd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Command != "unknown cmd" {
		t.Errorf("Command = %q, want %q", result.Command, "unknown cmd")
	}
}

func TestMockExecutorError(t *testing.T) {
	m := NewMockExecutor()
	m.ExpectRun("fail cmd", "", fmt.Errorf("command not found"))

	_, err := m.Run("fail", "cmd")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "command not found" {
		t.Errorf("error = %q, want %q", err.Error(), "command not found")
	}
}

func TestMockExecutorSetDir(t *testing.T) {
	m := NewMockExecutor()
	m.SetDir("/tmp/test")
	if m.GetDir() != "/tmp/test" {
		t.Errorf("GetDir() = %q, want %q", m.GetDir(), "/tmp/test")
	}
}

func TestMockExecutorSetEnv(t *testing.T) {
	m := NewMockExecutor()
	m.SetEnv([]string{"FOO=bar"})
	if len(m.Env) != 1 || m.Env[0] != "FOO=bar" {
		t.Errorf("Env = %v, want [FOO=bar]", m.Env)
	}
}

func TestMockExecutorReset(t *testing.T) {
	m := NewMockExecutor()
	m.ExpectRun("echo test", "test\n", nil)
	_, _ = m.Run("echo", "test")
	m.AssertCallCount(t, 1)

	m.Reset()
	m.AssertCallCount(t, 0)

	// Responses survive reset
	result, _ := m.Run("echo", "test")
	if result.Stdout != "test\n" {
		t.Errorf("response should survive Reset, got Stdout=%q", result.Stdout)
	}
}

func TestMockCallCommandString(t *testing.T) {
	tests := []struct {
		call MockCall
		want string
	}{
		{MockCall{Name: "echo"}, "echo"},
		{MockCall{Name: "echo", Args: []string{"hello"}}, "echo hello"},
		{MockCall{Name: "kubectl", Args: []string{"get", "pods", "-n", "default"}}, "kubectl get pods -n default"},
	}
	for _, tt := range tests {
		got := tt.call.CommandString()
		if got != tt.want {
			t.Errorf("CommandString() = %q, want %q", got, tt.want)
		}
	}
}

func TestMockExecutorExpectRunWithResult(t *testing.T) {
	m := NewMockExecutor()
	m.ExpectRunWithResult("tofu output -raw cluster_id", &Result{
		Command:  "tofu output -raw cluster_id",
		Stdout:   "ocid1.cluster.oc1\n",
		ExitCode: 0,
	}, nil)

	result, err := m.RunSilent("tofu", "output", "-raw", "cluster_id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
	if result.Stdout != "ocid1.cluster.oc1\n" {
		t.Errorf("Stdout = %q, want %q", result.Stdout, "ocid1.cluster.oc1\n")
	}
}

func TestAssertCallCountMismatch(t *testing.T) {
	m := NewMockExecutor()
	_, _ = m.Run("echo", "a")
	_, _ = m.Run("echo", "b")

	// Use a sub-test to capture the expected failure without failing the parent.
	inner := &testing.T{}
	m.AssertCallCount(inner, 5) // wrong count -- exercises the error+log loop
}

func TestAssertCalledNotFound(t *testing.T) {
	m := NewMockExecutor()
	_, _ = m.Run("echo", "hello")

	inner := &testing.T{}
	m.AssertCalled(inner, "nonexistent cmd") // not found -- exercises error+log loop
}

func TestMockNoArgsCommand(t *testing.T) {
	m := NewMockExecutor()
	m.ExpectRun("pwd", "/tmp\n", nil)

	result, err := m.Run("pwd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Stdout != "/tmp\n" {
		t.Errorf("Stdout = %q, want %q", result.Stdout, "/tmp\n")
	}
	m.AssertCalled(t, "pwd")
}
