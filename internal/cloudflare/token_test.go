package cloudflare

import (
	"testing"
)

func TestIsHex(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"abcdef0123456789", true},
		{"ABCDEF0123456789", true},
		{"0000000000000000", true},
		{"", true},
		{"xyz", false},
		{"abcdefg", false},
		{"12345 ", false},
	}
	for _, tt := range tests {
		if got := isHex(tt.input); got != tt.want {
			t.Errorf("isHex(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

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

func TestResolveTokenNoSources(t *testing.T) {
	t.Setenv("CLOUDFLARE_API_TOKEN", "")
	t.Setenv("OP_CLOUDFLARE_REF", "")
	t.Setenv("PATH", "/nonexistent")
	_, _, err := ResolveToken()
	if err == nil {
		t.Error("expected error when no token sources available")
	}
}

func TestResolveAccountIDNoWrangler(t *testing.T) {
	t.Setenv("PATH", "/nonexistent")
	id := ResolveAccountID()
	if id != "" {
		t.Errorf("expected empty account ID without wrangler, got %q", id)
	}
}
