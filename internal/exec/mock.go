package exec

import (
	"strings"
	"testing"
)

// MockCall records a single invocation on MockExecutor.
type MockCall struct {
	Method string // "Run" or "RunSilent"
	Name   string
	Args   []string
}

// CommandString returns the full command as "name arg1 arg2".
func (c MockCall) CommandString() string {
	if len(c.Args) == 0 {
		return c.Name
	}
	return c.Name + " " + strings.Join(c.Args, " ")
}

type mockResponse struct {
	result *Result
	err    error
}

// MockExecutor records calls and returns preconfigured responses.
// It satisfies the Executor interface for use in tests.
type MockExecutor struct {
	Calls     []MockCall
	responses map[string]mockResponse
	Dir       string
	Env       []string
}

// NewMockExecutor creates a MockExecutor with an empty response map.
func NewMockExecutor() *MockExecutor {
	return &MockExecutor{
		responses: make(map[string]mockResponse),
	}
}

// ExpectRun registers a canned response for the given full command string.
// Both Run and RunSilent match against the same key.
func (m *MockExecutor) ExpectRun(cmd string, stdout string, err error) {
	m.responses[cmd] = mockResponse{
		result: &Result{Command: cmd, Stdout: stdout},
		err:    err,
	}
}

// ExpectRunWithResult registers a canned Result for the given command string.
func (m *MockExecutor) ExpectRunWithResult(cmd string, r *Result, err error) {
	m.responses[cmd] = mockResponse{result: r, err: err}
}

func (m *MockExecutor) Run(name string, args ...string) (*Result, error) {
	cmd := name
	if len(args) > 0 {
		cmd += " " + strings.Join(args, " ")
	}
	m.Calls = append(m.Calls, MockCall{Method: "Run", Name: name, Args: args})
	if resp, ok := m.responses[cmd]; ok {
		return resp.result, resp.err
	}
	return &Result{Command: cmd}, nil
}

func (m *MockExecutor) RunSilent(name string, args ...string) (*Result, error) {
	cmd := name
	if len(args) > 0 {
		cmd += " " + strings.Join(args, " ")
	}
	m.Calls = append(m.Calls, MockCall{Method: "RunSilent", Name: name, Args: args})
	if resp, ok := m.responses[cmd]; ok {
		return resp.result, resp.err
	}
	return &Result{Command: cmd}, nil
}

func (m *MockExecutor) SetDir(dir string)   { m.Dir = dir }
func (m *MockExecutor) SetEnv(env []string) { m.Env = env }
func (m *MockExecutor) GetDir() string      { return m.Dir }

// AssertCallCount fails the test if the number of calls doesn't match n.
func (m *MockExecutor) AssertCallCount(t *testing.T, n int) {
	t.Helper()
	if len(m.Calls) != n {
		t.Errorf("expected %d calls, got %d", n, len(m.Calls))
		for i, c := range m.Calls {
			t.Logf("  call %d: %s %s", i, c.Method, c.CommandString())
		}
	}
}

// AssertCalled fails if the given command string was never called.
func (m *MockExecutor) AssertCalled(t *testing.T, cmd string) {
	t.Helper()
	for _, c := range m.Calls {
		if c.CommandString() == cmd {
			return
		}
	}
	t.Errorf("expected call %q not found in %d calls", cmd, len(m.Calls))
	for i, c := range m.Calls {
		t.Logf("  call %d: %s", i, c.CommandString())
	}
}

// Reset clears recorded calls (responses are kept).
func (m *MockExecutor) Reset() {
	m.Calls = nil
}
