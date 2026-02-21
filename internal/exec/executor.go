package exec

// Executor abstracts command execution for testability.
type Executor interface {
	// Run executes a command and streams output in real time.
	Run(name string, args ...string) (*Result, error)
	// RunSilent executes a command without streaming output (captures only).
	RunSilent(name string, args ...string) (*Result, error)
	// SetDir sets the working directory for subsequent commands.
	SetDir(dir string)
	// SetEnv sets environment variables for subsequent commands.
	SetEnv(env []string)
	// GetDir returns the current working directory.
	GetDir() string
}

// Ensure Runner satisfies the Executor interface.
var _ Executor = (*Runner)(nil)
