package secrets

import (
	"fmt"
	"testing"

	"github.com/disentangle-network/launch/internal/exec"
)

// --- Tests for opReadWith (internal testable core) ---

func TestOpReadWith_OpNotInstalled(t *testing.T) {
	mock := exec.NewMockExecutor()
	cmdExists := func(name string) bool { return false }

	_, err := opReadWith("op://vault/item/field", mock, cmdExists)
	if err == nil {
		t.Fatal("expected error when op not installed")
	}
	if err.Error() != "1Password CLI (op) not installed" {
		t.Errorf("unexpected error: %v", err)
	}
	mock.AssertCallCount(t, 0) // should not attempt to run op
}

func TestOpReadWith_RunSilentError(t *testing.T) {
	mock := exec.NewMockExecutor()
	mock.ExpectRun("op read op://vault/item/field", "", fmt.Errorf("exit status 1"))
	cmdExists := func(name string) bool { return true }

	_, err := opReadWith("op://vault/item/field", mock, cmdExists)
	if err == nil {
		t.Fatal("expected error when RunSilent fails")
	}
	expected := "op read op://vault/item/field failed: exit status 1"
	if err.Error() != expected {
		t.Errorf("got error %q, want %q", err.Error(), expected)
	}
	mock.AssertCallCount(t, 1)
	mock.AssertCalled(t, "op read op://vault/item/field")
}

func TestOpReadWith_EmptyStdout(t *testing.T) {
	mock := exec.NewMockExecutor()
	mock.ExpectRun("op read op://vault/item/field", "", nil)
	cmdExists := func(name string) bool { return true }

	_, err := opReadWith("op://vault/item/field", mock, cmdExists)
	if err == nil {
		t.Fatal("expected error when stdout is empty")
	}
	expected := "op read op://vault/item/field returned empty value"
	if err.Error() != expected {
		t.Errorf("got error %q, want %q", err.Error(), expected)
	}
}

func TestOpReadWith_WhitespaceOnlyStdout(t *testing.T) {
	mock := exec.NewMockExecutor()
	mock.ExpectRun("op read op://vault/item/field", "   \n\t  \n", nil)
	cmdExists := func(name string) bool { return true }

	_, err := opReadWith("op://vault/item/field", mock, cmdExists)
	if err == nil {
		t.Fatal("expected error when stdout is whitespace only")
	}
	expected := "op read op://vault/item/field returned empty value"
	if err.Error() != expected {
		t.Errorf("got error %q, want %q", err.Error(), expected)
	}
}

func TestOpReadWith_Success(t *testing.T) {
	mock := exec.NewMockExecutor()
	mock.ExpectRun("op read op://vault/item/field", "my-secret-value\n", nil)
	cmdExists := func(name string) bool { return true }

	val, err := opReadWith("op://vault/item/field", mock, cmdExists)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "my-secret-value" {
		t.Errorf("got %q, want %q", val, "my-secret-value")
	}
	mock.AssertCallCount(t, 1)
	mock.AssertCalled(t, "op read op://vault/item/field")
}

func TestOpReadWith_SuccessTrimsWhitespace(t *testing.T) {
	mock := exec.NewMockExecutor()
	mock.ExpectRun("op read op://vault/item/field", "  secret-with-spaces  \n", nil)
	cmdExists := func(name string) bool { return true }

	val, err := opReadWith("op://vault/item/field", mock, cmdExists)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "secret-with-spaces" {
		t.Errorf("got %q, want %q", val, "secret-with-spaces")
	}
}

func TestOpReadWith_DifferentRef(t *testing.T) {
	ref := "op://DevVault/api-key/credential"
	mock := exec.NewMockExecutor()
	mock.ExpectRun("op read "+ref, "abc123", nil)
	cmdExists := func(name string) bool { return true }

	val, err := opReadWith(ref, mock, cmdExists)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "abc123" {
		t.Errorf("got %q, want %q", val, "abc123")
	}
}

func TestOpReadWith_CmdExistsChecksOpSpecifically(t *testing.T) {
	mock := exec.NewMockExecutor()
	mock.ExpectRun("op read op://v/i/f", "val", nil)

	var checkedName string
	cmdExists := func(name string) bool {
		checkedName = name
		return true
	}

	_, err := opReadWith("op://v/i/f", mock, cmdExists)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if checkedName != "op" {
		t.Errorf("cmdExists was called with %q, want %q", checkedName, "op")
	}
}

// --- Tests for the public OpRead function (via package-level vars) ---

func TestOpRead_UsesPackageLevelVars(t *testing.T) {
	// Save originals and restore after test.
	origCmdExists := commandExistsFunc
	origNewExec := newExecutorFunc
	t.Cleanup(func() {
		commandExistsFunc = origCmdExists
		newExecutorFunc = origNewExec
	})

	mock := exec.NewMockExecutor()
	mock.ExpectRun("op read op://test/item/field", "injected-secret", nil)

	commandExistsFunc = func(name string) bool { return true }
	newExecutorFunc = func() exec.Executor { return mock }

	val, err := OpRead("op://test/item/field")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "injected-secret" {
		t.Errorf("got %q, want %q", val, "injected-secret")
	}
	mock.AssertCallCount(t, 1)
}

func TestOpRead_OpNotInstalledViaPackageVar(t *testing.T) {
	origCmdExists := commandExistsFunc
	origNewExec := newExecutorFunc
	t.Cleanup(func() {
		commandExistsFunc = origCmdExists
		newExecutorFunc = origNewExec
	})

	mock := exec.NewMockExecutor()
	commandExistsFunc = func(name string) bool { return false }
	newExecutorFunc = func() exec.Executor { return mock }

	_, err := OpRead("op://vault/item/field")
	if err == nil {
		t.Fatal("expected error when op not installed")
	}
	if err.Error() != "1Password CLI (op) not installed" {
		t.Errorf("unexpected error: %v", err)
	}
	mock.AssertCallCount(t, 0)
}

func TestOpRead_RunFailureViaPackageVar(t *testing.T) {
	origCmdExists := commandExistsFunc
	origNewExec := newExecutorFunc
	t.Cleanup(func() {
		commandExistsFunc = origCmdExists
		newExecutorFunc = origNewExec
	})

	mock := exec.NewMockExecutor()
	mock.ExpectRun("op read op://vault/item/field", "", fmt.Errorf("auth required"))
	commandExistsFunc = func(name string) bool { return true }
	newExecutorFunc = func() exec.Executor { return mock }

	_, err := OpRead("op://vault/item/field")
	if err == nil {
		t.Fatal("expected error on run failure")
	}
	expected := "op read op://vault/item/field failed: auth required"
	if err.Error() != expected {
		t.Errorf("got %q, want %q", err.Error(), expected)
	}
}

func TestOpRead_EmptyResultViaPackageVar(t *testing.T) {
	origCmdExists := commandExistsFunc
	origNewExec := newExecutorFunc
	t.Cleanup(func() {
		commandExistsFunc = origCmdExists
		newExecutorFunc = origNewExec
	})

	mock := exec.NewMockExecutor()
	mock.ExpectRun("op read op://vault/item/field", "", nil)
	commandExistsFunc = func(name string) bool { return true }
	newExecutorFunc = func() exec.Executor { return mock }

	_, err := OpRead("op://vault/item/field")
	if err == nil {
		t.Fatal("expected error for empty result")
	}
	expected := "op read op://vault/item/field returned empty value"
	if err.Error() != expected {
		t.Errorf("got %q, want %q", err.Error(), expected)
	}
}

// --- Legacy test: real PATH manipulation ---

func TestOpReadNoOpCLI(t *testing.T) {
	t.Setenv("PATH", "/nonexistent")

	_, err := OpRead("op://vault/item/field")
	if err == nil {
		t.Error("expected error when op CLI not installed")
	}
	if err.Error() != "1Password CLI (op) not installed" {
		t.Errorf("unexpected error message: %v", err)
	}
}
