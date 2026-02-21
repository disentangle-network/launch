package cloudflare

import (
	"fmt"
	"strings"
	"testing"

	"github.com/disentangle-network/launch/internal/exec"
)

// ---------------------------------------------------------------------------
// Test helpers: swap package-level seams and restore on cleanup
// ---------------------------------------------------------------------------

// withCommandExists overrides commandExists for the duration of a test.
func withCommandExists(t *testing.T, fn func(string) bool) {
	t.Helper()
	orig := commandExists
	commandExists = fn
	t.Cleanup(func() { commandExists = orig })
}

// withOpRead overrides opRead for the duration of a test.
func withOpRead(t *testing.T, fn func(string) (string, error)) {
	t.Helper()
	orig := opRead
	opRead = fn
	t.Cleanup(func() { opRead = orig })
}

// withNewRunner overrides newRunner for the duration of a test.
func withNewRunner(t *testing.T, fn func() exec.Executor) {
	t.Helper()
	orig := newRunner
	newRunner = fn
	t.Cleanup(func() { newRunner = orig })
}

// stubCommandExists returns a function that reports true only for names in the set.
func stubCommandExists(cmds ...string) func(string) bool {
	set := make(map[string]bool, len(cmds))
	for _, c := range cmds {
		set[c] = true
	}
	return func(name string) bool { return set[name] }
}

// ---------------------------------------------------------------------------
// isHex
// ---------------------------------------------------------------------------

func TestIsHexValid(t *testing.T) {
	valid := []struct {
		name  string
		input string
	}{
		{"lowercase hex", "abcdef0123456789"},
		{"uppercase hex", "ABCDEF0123456789"},
		{"all zeros", "0000000000000000"},
		{"empty string", ""},
		{"single digit", "0"},
		{"single hex letter", "f"},
		{"mixed case hex", "aAbBcCdDeEfF"},
		{"32-char account id", "1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d"},
		{"all f", "ffffffffffffffffffffffffffffffff"},
		{"all 9", "9999"},
		{"short hex", "ab"},
	}
	for _, tt := range valid {
		t.Run(tt.name, func(t *testing.T) {
			if !isHex(tt.input) {
				t.Errorf("isHex(%q) = false, want true", tt.input)
			}
		})
	}
}

func TestIsHexInvalid(t *testing.T) {
	invalid := []struct {
		name  string
		input string
	}{
		{"lowercase beyond f", "abcdefg"},
		{"uppercase beyond F", "ABCDEFG"},
		{"space in middle", "123 456"},
		{"trailing space", "12345 "},
		{"leading space", " 12345"},
		{"special chars", "abc!def"},
		{"hyphen", "abc-def"},
		{"underscore", "abc_def"},
		{"newline", "abc\ndef"},
		{"tab", "abc\tdef"},
		{"unicode", "abc\u00e9def"},
		{"dot", "abc.def"},
		{"at sign", "abc@def"},
		{"colon", "ab:cd"},
		{"slash", "ab/cd"},
	}
	for _, tt := range invalid {
		t.Run(tt.name, func(t *testing.T) {
			if isHex(tt.input) {
				t.Errorf("isHex(%q) = true, want false", tt.input)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ResolveTokenFromConfig
// ---------------------------------------------------------------------------

func TestResolveTokenFromConfigWithValue(t *testing.T) {
	token, source, err := ResolveTokenFromConfig("my-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "my-token" {
		t.Errorf("token = %q, want %q", token, "my-token")
	}
	if source != "launch-disentangle config" {
		t.Errorf("source = %q, want %q", source, "launch-disentangle config")
	}
}

func TestResolveTokenFromConfigPrefersConfigOverEnv(t *testing.T) {
	t.Setenv("CLOUDFLARE_API_TOKEN", "env-token-should-not-win")
	token, source, err := ResolveTokenFromConfig("config-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "config-token" {
		t.Errorf("token = %q, want %q", token, "config-token")
	}
	if source != "launch-disentangle config" {
		t.Errorf("source = %q, want %q", source, "launch-disentangle config")
	}
}

func TestResolveTokenFromConfigEmptyFallsThrough(t *testing.T) {
	t.Setenv("CLOUDFLARE_API_TOKEN", "env-fallback-token")
	token, source, err := ResolveTokenFromConfig("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "env-fallback-token" {
		t.Errorf("token = %q, want %q", token, "env-fallback-token")
	}
	if source != "CLOUDFLARE_API_TOKEN env var" {
		t.Errorf("source = %q, want %q", source, "CLOUDFLARE_API_TOKEN env var")
	}
}

func TestResolveTokenFromConfigEmptyNoSources(t *testing.T) {
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	t.Setenv("OP_CLOUDFLARE_REF", "")
	withCommandExists(t, stubCommandExists()) // nothing exists
	_, _, err := ResolveTokenFromConfig("")
	if err == nil {
		t.Error("expected error when config is empty and no token sources available")
	}
}

// ---------------------------------------------------------------------------
// ResolveToken -- env var path
// ---------------------------------------------------------------------------

func TestResolveTokenFromEnvVar(t *testing.T) {
	t.Setenv("CLOUDFLARE_API_TOKEN", "env-token-123")
	token, source, err := ResolveToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "env-token-123" {
		t.Errorf("token = %q, want %q", token, "env-token-123")
	}
	if source != "CLOUDFLARE_API_TOKEN env var" {
		t.Errorf("source = %q, want %q", source, "CLOUDFLARE_API_TOKEN env var")
	}
}

func TestResolveTokenPrecedence(t *testing.T) {
	t.Setenv("CLOUDFLARE_API_TOKEN", "env-wins")
	t.Setenv("OP_CLOUDFLARE_REF", "op://should/not/be/used")
	token, source, _ := ResolveToken()
	if token != "env-wins" {
		t.Errorf("env var should take precedence, got token=%q source=%q", token, source)
	}
}

// ---------------------------------------------------------------------------
// ResolveToken -- 1Password custom ref path (OP_CLOUDFLARE_REF)
// ---------------------------------------------------------------------------

func TestResolveTokenOpRefSuccess(t *testing.T) {
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	t.Setenv("OP_CLOUDFLARE_REF", "op://vault/item/field")

	withOpRead(t, func(ref string) (string, error) {
		if ref == "op://vault/item/field" {
			return "op-custom-secret", nil
		}
		return "", fmt.Errorf("not found")
	})
	// commandExists doesn't matter here because we return before checking it.

	token, source, err := ResolveToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "op-custom-secret" {
		t.Errorf("token = %q, want %q", token, "op-custom-secret")
	}
	if source != "1Password (op://vault/item/field)" {
		t.Errorf("source = %q, want %q", source, "1Password (op://vault/item/field)")
	}
}

func TestResolveTokenOpRefFailsFallsToDefaultOp(t *testing.T) {
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	t.Setenv("OP_CLOUDFLARE_REF", "op://vault/item/field")

	calls := []string{}
	withOpRead(t, func(ref string) (string, error) {
		calls = append(calls, ref)
		if ref == "op://Infrastructure/disentangle/cloudflare" {
			return "op-default-secret", nil
		}
		return "", fmt.Errorf("op read failed")
	})
	withCommandExists(t, stubCommandExists("op", "wrangler"))

	token, source, err := ResolveToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "op-default-secret" {
		t.Errorf("token = %q, want %q", token, "op-default-secret")
	}
	if source != "1Password (default ref)" {
		t.Errorf("source = %q, want %q", source, "1Password (default ref)")
	}
	// Verify both refs were attempted.
	if len(calls) != 2 {
		t.Errorf("expected 2 opRead calls, got %d: %v", len(calls), calls)
	}
}

// ---------------------------------------------------------------------------
// ResolveToken -- 1Password default ref path (op exists, no OP_CLOUDFLARE_REF)
// ---------------------------------------------------------------------------

func TestResolveTokenOpDefaultRefSuccess(t *testing.T) {
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	t.Setenv("OP_CLOUDFLARE_REF", "")

	withOpRead(t, func(ref string) (string, error) {
		if ref == "op://Infrastructure/disentangle/cloudflare" {
			return "op-default-token", nil
		}
		return "", fmt.Errorf("not found")
	})
	withCommandExists(t, stubCommandExists("op"))

	token, source, err := ResolveToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "op-default-token" {
		t.Errorf("token = %q, want %q", token, "op-default-token")
	}
	if source != "1Password (default ref)" {
		t.Errorf("source = %q, want %q", source, "1Password (default ref)")
	}
}

func TestResolveTokenOpDefaultRefFailsFallsToWrangler(t *testing.T) {
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	t.Setenv("OP_CLOUDFLARE_REF", "")

	withOpRead(t, func(_ string) (string, error) {
		return "", fmt.Errorf("op read failed")
	})
	withCommandExists(t, stubCommandExists("op", "wrangler"))

	mock := exec.NewMockExecutor()
	mock.ExpectRun("wrangler auth token", "some-banner\ntoken-abc-from-cli\n", nil)
	withNewRunner(t, func() exec.Executor { return mock })

	token, source, err := ResolveToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "token-abc-from-cli" {
		t.Errorf("token = %q, want %q", token, "token-abc-from-cli")
	}
	if source != "wrangler auth token" {
		t.Errorf("source = %q, want %q", source, "wrangler auth token")
	}
}

// ---------------------------------------------------------------------------
// ResolveToken -- no op, no wrangler
// ---------------------------------------------------------------------------

func TestResolveTokenNoSources(t *testing.T) {
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	t.Setenv("OP_CLOUDFLARE_REF", "")
	withCommandExists(t, stubCommandExists()) // nothing available
	_, _, err := ResolveToken()
	if err == nil {
		t.Error("expected error when no token sources available")
	}
}

func TestResolveTokenNoSourcesErrorMessage(t *testing.T) {
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	t.Setenv("OP_CLOUDFLARE_REF", "")
	withCommandExists(t, stubCommandExists()) // nothing available
	_, _, err := ResolveToken()
	if err == nil {
		t.Fatal("expected error when no token sources available")
	}
	if !strings.Contains(err.Error(), "wrangler not installed") {
		t.Errorf("error message = %q, want it to mention wrangler not installed", err.Error())
	}
}

// ---------------------------------------------------------------------------
// ResolveToken -- OP_CLOUDFLARE_REF set but op fails, wrangler not available
// ---------------------------------------------------------------------------

func TestResolveTokenOpRefFallthrough(t *testing.T) {
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	t.Setenv("OP_CLOUDFLARE_REF", "op://vault/item/field")

	withOpRead(t, func(_ string) (string, error) {
		return "", fmt.Errorf("op not available")
	})
	withCommandExists(t, stubCommandExists()) // neither op nor wrangler

	_, _, err := ResolveToken()
	if err == nil {
		t.Error("expected error when op and wrangler are not available")
	}
}

func TestResolveTokenOpRefWithEnvFallback(t *testing.T) {
	t.Setenv("CLOUDFLARE_API_TOKEN", "env-takes-priority")
	t.Setenv("OP_CLOUDFLARE_REF", "op://vault/item/field")
	token, source, err := ResolveToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "env-takes-priority" {
		t.Errorf("token = %q, want %q", token, "env-takes-priority")
	}
	if source != "CLOUDFLARE_API_TOKEN env var" {
		t.Errorf("source = %q, want %q", source, "CLOUDFLARE_API_TOKEN env var")
	}
}

// ---------------------------------------------------------------------------
// ResolveToken -- wrangler path
// ---------------------------------------------------------------------------

func TestResolveTokenWranglerSuccess(t *testing.T) {
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	t.Setenv("OP_CLOUDFLARE_REF", "")

	withCommandExists(t, stubCommandExists("wrangler"))
	withOpRead(t, func(_ string) (string, error) {
		return "", fmt.Errorf("not available")
	})

	mock := exec.NewMockExecutor()
	mock.ExpectRun("wrangler auth token", "some banner text\nmy-cf-api-token\n", nil)
	withNewRunner(t, func() exec.Executor { return mock })

	token, source, err := ResolveToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "my-cf-api-token" {
		t.Errorf("token = %q, want %q", token, "my-cf-api-token")
	}
	if source != "wrangler auth token" {
		t.Errorf("source = %q, want %q", source, "wrangler auth token")
	}
}

func TestResolveTokenWranglerMultilineBanner(t *testing.T) {
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	t.Setenv("OP_CLOUDFLARE_REF", "")

	withCommandExists(t, stubCommandExists("wrangler"))
	withOpRead(t, func(_ string) (string, error) {
		return "", fmt.Errorf("not available")
	})

	// Simulate wrangler output with multiple banner lines.
	banner := "Cloudflare Workers\nVersion 3.x\n\nactual-token-value\n"
	mock := exec.NewMockExecutor()
	mock.ExpectRun("wrangler auth token", banner, nil)
	withNewRunner(t, func() exec.Executor { return mock })

	token, _, err := ResolveToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "actual-token-value" {
		t.Errorf("token = %q, want %q", token, "actual-token-value")
	}
}

func TestResolveTokenWranglerFails(t *testing.T) {
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	t.Setenv("OP_CLOUDFLARE_REF", "")

	withCommandExists(t, stubCommandExists("wrangler"))
	withOpRead(t, func(_ string) (string, error) {
		return "", fmt.Errorf("not available")
	})

	mock := exec.NewMockExecutor()
	mock.ExpectRun("wrangler auth token", "", fmt.Errorf("exit code 1"))
	withNewRunner(t, func() exec.Executor { return mock })

	_, _, err := ResolveToken()
	if err == nil {
		t.Fatal("expected error when wrangler auth token fails")
	}
	if !strings.Contains(err.Error(), "wrangler auth token failed") {
		t.Errorf("error = %q, want it to contain 'wrangler auth token failed'", err.Error())
	}
}

func TestResolveTokenWranglerEmptyOutput(t *testing.T) {
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	t.Setenv("OP_CLOUDFLARE_REF", "")

	withCommandExists(t, stubCommandExists("wrangler"))
	withOpRead(t, func(_ string) (string, error) {
		return "", fmt.Errorf("not available")
	})

	mock := exec.NewMockExecutor()
	mock.ExpectRun("wrangler auth token", "\n\n", nil)
	withNewRunner(t, func() exec.Executor { return mock })

	_, _, err := ResolveToken()
	if err == nil {
		t.Fatal("expected error when wrangler returns empty output")
	}
	if !strings.Contains(err.Error(), "no usable token") {
		t.Errorf("error = %q, want it to contain 'no usable token'", err.Error())
	}
}

func TestResolveTokenWranglerOutputContainsWrangler(t *testing.T) {
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	t.Setenv("OP_CLOUDFLARE_REF", "")

	withCommandExists(t, stubCommandExists("wrangler"))
	withOpRead(t, func(_ string) (string, error) {
		return "", fmt.Errorf("not available")
	})

	// If the last non-empty line contains "wrangler", it is not a usable token.
	mock := exec.NewMockExecutor()
	mock.ExpectRun("wrangler auth token", "Please run wrangler login first\n", nil)
	withNewRunner(t, func() exec.Executor { return mock })

	_, _, err := ResolveToken()
	if err == nil {
		t.Fatal("expected error when wrangler output contains 'wrangler'")
	}
	if !strings.Contains(err.Error(), "not logged in") {
		t.Errorf("error = %q, want it to contain 'not logged in'", err.Error())
	}
}

func TestResolveTokenWranglerTokenOnlyLine(t *testing.T) {
	// Wrangler outputs only the token with no banner.
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	t.Setenv("OP_CLOUDFLARE_REF", "")

	withCommandExists(t, stubCommandExists("wrangler"))
	withOpRead(t, func(_ string) (string, error) {
		return "", fmt.Errorf("not available")
	})

	mock := exec.NewMockExecutor()
	mock.ExpectRun("wrangler auth token", "single-line-token", nil)
	withNewRunner(t, func() exec.Executor { return mock })

	token, source, err := ResolveToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "single-line-token" {
		t.Errorf("token = %q, want %q", token, "single-line-token")
	}
	if source != "wrangler auth token" {
		t.Errorf("source = %q, want %q", source, "wrangler auth token")
	}
}

func TestResolveTokenWranglerTrailingNewlines(t *testing.T) {
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	t.Setenv("OP_CLOUDFLARE_REF", "")

	withCommandExists(t, stubCommandExists("wrangler"))
	withOpRead(t, func(_ string) (string, error) {
		return "", fmt.Errorf("not available")
	})

	// Token with trailing whitespace and newlines.
	mock := exec.NewMockExecutor()
	mock.ExpectRun("wrangler auth token", "  clean-token  \n\n\n", nil)
	withNewRunner(t, func() exec.Executor { return mock })

	token, _, err := ResolveToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "clean-token" {
		t.Errorf("token = %q, want %q", token, "clean-token")
	}
}

// ---------------------------------------------------------------------------
// ResolveToken -- OP_CLOUDFLARE_REF set, op read succeeds for custom ref
// but also both op and wrangler exist (should return custom ref, not default)
// ---------------------------------------------------------------------------

func TestResolveTokenOpRefSuccessSkipsDefaultAndWrangler(t *testing.T) {
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	t.Setenv("OP_CLOUDFLARE_REF", "op://custom/vault/token")

	withOpRead(t, func(ref string) (string, error) {
		if ref == "op://custom/vault/token" {
			return "custom-op-token", nil
		}
		return "", fmt.Errorf("should not be called for %s", ref)
	})
	// op and wrangler both "exist" but should not be reached.
	withCommandExists(t, stubCommandExists("op", "wrangler"))

	token, source, err := ResolveToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "custom-op-token" {
		t.Errorf("token = %q, want %q", token, "custom-op-token")
	}
	if source != "1Password (op://custom/vault/token)" {
		t.Errorf("source = %q, want %q", source, "1Password (op://custom/vault/token)")
	}
}

// ---------------------------------------------------------------------------
// ResolveToken -- both op reads fail, wrangler exists and succeeds
// ---------------------------------------------------------------------------

func TestResolveTokenBothOpFailsWranglerSucceeds(t *testing.T) {
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	t.Setenv("OP_CLOUDFLARE_REF", "op://vault/item/field")

	withOpRead(t, func(_ string) (string, error) {
		return "", fmt.Errorf("op failed")
	})
	withCommandExists(t, stubCommandExists("op", "wrangler"))

	mock := exec.NewMockExecutor()
	mock.ExpectRun("wrangler auth token", "final-fallback-token\n", nil)
	withNewRunner(t, func() exec.Executor { return mock })

	token, source, err := ResolveToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "final-fallback-token" {
		t.Errorf("token = %q, want %q", token, "final-fallback-token")
	}
	if source != "wrangler auth token" {
		t.Errorf("source = %q, want %q", source, "wrangler auth token")
	}
}

// ---------------------------------------------------------------------------
// ResolveAccountID
// ---------------------------------------------------------------------------

func TestResolveAccountIDNoWrangler(t *testing.T) {
	withCommandExists(t, stubCommandExists()) // nothing available
	id := ResolveAccountID()
	if id != "" {
		t.Errorf("expected empty account ID without wrangler, got %q", id)
	}
}

func TestResolveAccountIDWranglerError(t *testing.T) {
	withCommandExists(t, stubCommandExists("wrangler"))

	mock := exec.NewMockExecutor()
	mock.ExpectRun("wrangler whoami", "", fmt.Errorf("wrangler crashed"))
	withNewRunner(t, func() exec.Executor { return mock })

	id := ResolveAccountID()
	if id != "" {
		t.Errorf("expected empty account ID on wrangler error, got %q", id)
	}
}

func TestResolveAccountIDNotAuthenticated(t *testing.T) {
	withCommandExists(t, stubCommandExists("wrangler"))

	mock := exec.NewMockExecutor()
	mock.ExpectRun("wrangler whoami", "You are not authenticated. Please run wrangler login.", nil)
	withNewRunner(t, func() exec.Executor { return mock })

	id := ResolveAccountID()
	if id != "" {
		t.Errorf("expected empty account ID when not authenticated, got %q", id)
	}
}

func TestResolveAccountIDParsesHexFromTable(t *testing.T) {
	withCommandExists(t, stubCommandExists("wrangler"))

	// Simulate typical wrangler whoami table output.
	output := strings.Join([]string{
		"Getting User settings...",
		"",
		"Account Name │ Account ID",
		"MyAccount    │ 1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d",
		"",
	}, "\n")

	mock := exec.NewMockExecutor()
	mock.ExpectRun("wrangler whoami", output, nil)
	withNewRunner(t, func() exec.Executor { return mock })

	id := ResolveAccountID()
	if id != "1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d" {
		t.Errorf("account ID = %q, want %q", id, "1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d")
	}
}

func TestResolveAccountIDUppercaseHex(t *testing.T) {
	withCommandExists(t, stubCommandExists("wrangler"))

	output := "Account │ AABB00112233445566778899AABBCCDD\n"
	mock := exec.NewMockExecutor()
	mock.ExpectRun("wrangler whoami", output, nil)
	withNewRunner(t, func() exec.Executor { return mock })

	id := ResolveAccountID()
	if id != "AABB00112233445566778899AABBCCDD" {
		t.Errorf("account ID = %q, want %q", id, "AABB00112233445566778899AABBCCDD")
	}
}

func TestResolveAccountIDNoHexInOutput(t *testing.T) {
	withCommandExists(t, stubCommandExists("wrangler"))

	output := "Account │ some-non-hex-value\nAnother line\n"
	mock := exec.NewMockExecutor()
	mock.ExpectRun("wrangler whoami", output, nil)
	withNewRunner(t, func() exec.Executor { return mock })

	id := ResolveAccountID()
	if id != "" {
		t.Errorf("expected empty account ID when no 32-char hex found, got %q", id)
	}
}

func TestResolveAccountIDHexWrongLength(t *testing.T) {
	withCommandExists(t, stubCommandExists("wrangler"))

	// A hex string that is 31 chars (too short) and one that is 33 (too long).
	output := strings.Join([]string{
		"Account │ 1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6",   // 31 chars
		"Account │ 1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6de", // 33 chars
	}, "\n")
	mock := exec.NewMockExecutor()
	mock.ExpectRun("wrangler whoami", output, nil)
	withNewRunner(t, func() exec.Executor { return mock })

	id := ResolveAccountID()
	if id != "" {
		t.Errorf("expected empty account ID for wrong-length hex, got %q", id)
	}
}

func TestResolveAccountIDMultipleAccounts(t *testing.T) {
	withCommandExists(t, stubCommandExists("wrangler"))

	// Multiple accounts in table -- should return the first 32-char hex match.
	output := strings.Join([]string{
		"Account Name │ Account ID",
		"First Acct   │ aaaabbbbccccddddeeeeffffaaaabbbb",
		"Second Acct  │ 11112222333344445555666677778888",
	}, "\n")
	mock := exec.NewMockExecutor()
	mock.ExpectRun("wrangler whoami", output, nil)
	withNewRunner(t, func() exec.Executor { return mock })

	id := ResolveAccountID()
	if id != "aaaabbbbccccddddeeeeffffaaaabbbb" {
		t.Errorf("account ID = %q, want first match %q", id, "aaaabbbbccccddddeeeeffffaaaabbbb")
	}
}

func TestResolveAccountIDEmptyOutput(t *testing.T) {
	withCommandExists(t, stubCommandExists("wrangler"))

	mock := exec.NewMockExecutor()
	mock.ExpectRun("wrangler whoami", "", nil)
	withNewRunner(t, func() exec.Executor { return mock })

	id := ResolveAccountID()
	if id != "" {
		t.Errorf("expected empty account ID for empty output, got %q", id)
	}
}

func TestResolveAccountIDNoPipeDelimiter(t *testing.T) {
	withCommandExists(t, stubCommandExists("wrangler"))

	// Output without pipe delimiters -- a 32-char hex string on a line
	// without pipes would appear as a single "part" after splitting on │.
	output := "1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d\n"
	mock := exec.NewMockExecutor()
	mock.ExpectRun("wrangler whoami", output, nil)
	withNewRunner(t, func() exec.Executor { return mock })

	id := ResolveAccountID()
	if id != "1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d" {
		t.Errorf("account ID = %q, want %q", id, "1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d")
	}
}

func TestResolveAccountIDOnlyWhitespace(t *testing.T) {
	withCommandExists(t, stubCommandExists("wrangler"))

	mock := exec.NewMockExecutor()
	mock.ExpectRun("wrangler whoami", "   \n  \n  ", nil)
	withNewRunner(t, func() exec.Executor { return mock })

	id := ResolveAccountID()
	if id != "" {
		t.Errorf("expected empty account ID for whitespace-only output, got %q", id)
	}
}

func TestResolveAccountIDHexWithSurroundingSpaces(t *testing.T) {
	withCommandExists(t, stubCommandExists("wrangler"))

	// The hex value has surrounding spaces that should be trimmed.
	output := "Account │   1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d   \n"
	mock := exec.NewMockExecutor()
	mock.ExpectRun("wrangler whoami", output, nil)
	withNewRunner(t, func() exec.Executor { return mock })

	id := ResolveAccountID()
	if id != "1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d" {
		t.Errorf("account ID = %q, want %q", id, "1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d")
	}
}
