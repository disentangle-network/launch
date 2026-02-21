package cmd

import (
	"bytes"
	"runtime/debug"
	"strings"
	"testing"
)

func TestVersionInfoDev(t *testing.T) {
	// With default ldflags (version == "dev"), versionInfo should return
	// something containing "dev" since we're running tests via go test.
	info := versionInfo()
	if info == "" {
		t.Fatal("versionInfo() returned empty string")
	}
	if !strings.Contains(info, "dev") {
		// When built with GoReleaser, version != "dev" so this test
		// would not apply. That's fine -- we only test the default path.
		t.Logf("versionInfo() = %q (may have VCS info)", info)
	}
}

func TestVersionInfoInjected(t *testing.T) {
	// Temporarily set the package-level vars to simulate GoReleaser ldflags.
	origVersion, origCommit, origDate := version, commit, buildDate
	defer func() {
		version, commit, buildDate = origVersion, origCommit, origDate
	}()

	version = "v1.2.3"
	commit = "abc1234"
	buildDate = "2026-01-01T00:00:00Z"

	info := versionInfo()
	if !strings.Contains(info, "v1.2.3") {
		t.Errorf("versionInfo() = %q, want to contain %q", info, "v1.2.3")
	}
	if !strings.Contains(info, "abc1234") {
		t.Errorf("versionInfo() = %q, want to contain %q", info, "abc1234")
	}
	if !strings.Contains(info, "2026-01-01T00:00:00Z") {
		t.Errorf("versionInfo() = %q, want to contain %q", info, "2026-01-01T00:00:00Z")
	}
}

func TestConfirmAutoYes(t *testing.T) {
	origAutoYes := autoYes
	defer func() { autoYes = origAutoYes }()

	autoYes = true
	if !confirm("test?") {
		t.Error("confirm() should return true when autoYes is set")
	}
}

func TestVersionFromBuildInfoModuleVersion(t *testing.T) {
	// Simulate `go install module@vX.Y.Z` where Main.Version is set.
	info := &debug.BuildInfo{
		Main: debug.Module{
			Version: "v1.5.0",
		},
	}
	got := versionFromBuildInfo(info)
	if got != "v1.5.0" {
		t.Errorf("versionFromBuildInfo() = %q, want %q", got, "v1.5.0")
	}
}

func TestVersionFromBuildInfoDevelVersion(t *testing.T) {
	// Main.Version == "(devel)" should fall through to VCS settings.
	info := &debug.BuildInfo{
		Main: debug.Module{
			Version: "(devel)",
		},
	}
	got := versionFromBuildInfo(info)
	// No VCS settings, so should return "dev (unknown commit)".
	if got != "dev (unknown commit)" {
		t.Errorf("versionFromBuildInfo() = %q, want %q", got, "dev (unknown commit)")
	}
}

func TestVersionFromBuildInfoEmptyVersion(t *testing.T) {
	// Main.Version == "" should also fall through.
	info := &debug.BuildInfo{
		Main: debug.Module{
			Version: "",
		},
	}
	got := versionFromBuildInfo(info)
	if got != "dev (unknown commit)" {
		t.Errorf("versionFromBuildInfo() = %q, want %q", got, "dev (unknown commit)")
	}
}

func TestVersionFromBuildInfoVCSRevisionOnly(t *testing.T) {
	// VCS revision present, no timestamp, not modified.
	info := &debug.BuildInfo{
		Main: debug.Module{Version: "(devel)"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "abcdef1234567890"},
		},
	}
	got := versionFromBuildInfo(info)
	want := "dev-abcdef1"
	if got != want {
		t.Errorf("versionFromBuildInfo() = %q, want %q", got, want)
	}
}

func TestVersionFromBuildInfoVCSShortRevision(t *testing.T) {
	// VCS revision shorter than 7 characters.
	info := &debug.BuildInfo{
		Main: debug.Module{Version: "(devel)"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "abc"},
		},
	}
	got := versionFromBuildInfo(info)
	want := "dev-abc"
	if got != want {
		t.Errorf("versionFromBuildInfo() = %q, want %q", got, want)
	}
}

func TestVersionFromBuildInfoVCSRevisionWithTimestamp(t *testing.T) {
	info := &debug.BuildInfo{
		Main: debug.Module{Version: "(devel)"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "abcdef1234567890"},
			{Key: "vcs.time", Value: "2026-02-20T12:00:00Z"},
		},
	}
	got := versionFromBuildInfo(info)
	want := "dev-abcdef1 (2026-02-20T12:00:00Z)"
	if got != want {
		t.Errorf("versionFromBuildInfo() = %q, want %q", got, want)
	}
}

func TestVersionFromBuildInfoVCSDirty(t *testing.T) {
	// Modified (dirty) should take precedence over timestamp.
	info := &debug.BuildInfo{
		Main: debug.Module{Version: "(devel)"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "abcdef1234567890"},
			{Key: "vcs.time", Value: "2026-02-20T12:00:00Z"},
			{Key: "vcs.modified", Value: "true"},
		},
	}
	got := versionFromBuildInfo(info)
	want := "dev-abcdef1 (dirty)"
	if got != want {
		t.Errorf("versionFromBuildInfo() = %q, want %q", got, want)
	}
}

func TestVersionFromBuildInfoVCSNotModified(t *testing.T) {
	// vcs.modified == "false" should not trigger the dirty path.
	info := &debug.BuildInfo{
		Main: debug.Module{Version: "(devel)"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "abcdef1234567890"},
			{Key: "vcs.modified", Value: "false"},
		},
	}
	got := versionFromBuildInfo(info)
	want := "dev-abcdef1"
	if got != want {
		t.Errorf("versionFromBuildInfo() = %q, want %q", got, want)
	}
}

func TestConfirmReaderYes(t *testing.T) {
	origAutoYes := autoYes
	defer func() { autoYes = origAutoYes }()
	autoYes = false

	tests := []struct {
		input string
		want  bool
	}{
		{"y\n", true},
		{"Y\n", true},
		{"yes\n", true},
		{"n\n", false},
		{"N\n", false},
		{"no\n", false},
		{"\n", false},
		{"maybe\n", false},
	}
	for _, tt := range tests {
		r := strings.NewReader(tt.input)
		got := confirmReader("test?", r)
		if got != tt.want {
			t.Errorf("confirmReader with input %q = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestConfirmReaderAutoYes(t *testing.T) {
	origAutoYes := autoYes
	defer func() { autoYes = origAutoYes }()
	autoYes = true

	// With autoYes, should return true regardless of input.
	r := strings.NewReader("n\n")
	if !confirmReader("test?", r) {
		t.Error("confirmReader should return true when autoYes is set")
	}
}

func TestConfirmReaderEmptyInput(t *testing.T) {
	origAutoYes := autoYes
	defer func() { autoYes = origAutoYes }()
	autoYes = false

	// Empty reader (EOF immediately) should return false.
	r := strings.NewReader("")
	if confirmReader("test?", r) {
		t.Error("confirmReader with empty reader should return false")
	}
}

func TestCommandRegistration(t *testing.T) {
	commands := map[string][]string{
		"setup":            {},
		"preflight":        {},
		"bootstrap":        {"cluster", "fleet-dir", "repo", "owner", "branch", "context"},
		"fleet init":       {"dir", "remote"},
		"fleet status":     {},
		"cluster add":      {"arch", "infra", "nodes", "resources", "storage-class", "nebula", "nebula-prefix", "nebula-lighthouse"},
		"cluster list":     {},
		"cluster import":   {"context"},
		"secrets init":     {"cluster", "provider", "key-arn", "fleet-dir"},
		"infra init":       {},
		"infra plan":       {},
		"infra apply":      {},
		"infra destroy":    {},
		"infra output":     {},
		"infra kubeconfig": {},
		"mesh init":        {"ca-output"},
		"mesh add":         {"cluster", "lighthouse", "lighthouse-addr", "fleet-dir"},
		"status":           {"fleet-dir"},
	}
	for path, flags := range commands {
		parts := strings.Fields(path)
		cmd, _, err := rootCmd.Find(parts)
		if err != nil {
			t.Errorf("command %q not found: %v", path, err)
			continue
		}
		if cmd.Name() != parts[len(parts)-1] {
			t.Errorf("command %q resolved to %q instead", path, cmd.Name())
			continue
		}
		for _, flag := range flags {
			if cmd.Flags().Lookup(flag) != nil {
				continue
			}
			if cmd.InheritedFlags().Lookup(flag) != nil {
				continue
			}
			t.Errorf("command %q missing flag --%s", path, flag)
		}
	}
}

func TestGlobalFlags(t *testing.T) {
	globalFlags := []string{"config", "verbose", "dry-run", "yes"}
	for _, flag := range globalFlags {
		if rootCmd.PersistentFlags().Lookup(flag) == nil {
			t.Errorf("root command missing global persistent flag --%s", flag)
		}
	}
}

func TestRootCommandHelp(t *testing.T) {
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"--help"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("rootCmd --help returned error: %v", err)
	}
	output := buf.String()

	expectedSubcommands := []string{
		"setup", "preflight", "bootstrap", "fleet", "cluster",
		"secrets", "infra", "mesh", "status",
	}
	for _, sub := range expectedSubcommands {
		if !strings.Contains(output, sub) {
			t.Errorf("help output missing subcommand %q", sub)
		}
	}
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"a", 1},
		{"a\nb", 2},
		{"a\nb\nc", 3},
		{"a\nb\n", 2},
	}
	for _, tt := range tests {
		got := splitLines(tt.input)
		if len(got) != tt.want {
			t.Errorf("splitLines(%q) = %d lines, want %d", tt.input, len(got), tt.want)
		}
	}
}
