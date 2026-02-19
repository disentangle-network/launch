package cloudflare

import (
	"fmt"
	"os"
	"strings"

	"github.com/disentangle-network/launch/internal/exec"
)

// ResolveToken returns a Cloudflare API token using the following precedence:
//  1. CLOUDFLARE_API_TOKEN environment variable
//  2. wrangler auth token (OAuth token from wrangler login)
//
// Returns the token and a description of the source. If no token can be
// resolved, returns empty strings and an error.
func ResolveToken() (token string, source string, err error) {
	if t := os.Getenv("CLOUDFLARE_API_TOKEN"); t != "" {
		return t, "CLOUDFLARE_API_TOKEN env var", nil
	}

	if !exec.CommandExists("wrangler") {
		return "", "", fmt.Errorf("CLOUDFLARE_API_TOKEN not set and wrangler not installed")
	}

	runner := exec.NewRunner()
	out, err := runner.RunSilent("wrangler", "auth", "token")
	if err != nil {
		return "", "", fmt.Errorf("wrangler auth token failed: %w", err)
	}

	// wrangler prints a banner to stdout before the token; extract the last non-empty line.
	var t string
	for _, line := range strings.Split(out.Stdout, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			t = line
		}
	}
	if t == "" || strings.Contains(t, "wrangler") {
		return "", "", fmt.Errorf("wrangler auth token returned no usable token (not logged in?)")
	}

	return t, "wrangler auth token", nil
}

// ResolveAccountID returns the Cloudflare account ID using the following precedence:
//  1. Existing value in cfg (already discovered by setup command)
//  2. wrangler whoami (parse 32-char hex from table output)
//
// Returns the account ID or empty string if not found.
func ResolveAccountID() string {
	if !exec.CommandExists("wrangler") {
		return ""
	}

	runner := exec.NewRunner()
	out, err := runner.RunSilent("wrangler", "whoami")
	if err != nil || strings.Contains(out.Stdout, "not authenticated") {
		return ""
	}

	for _, line := range strings.Split(out.Stdout, "\n") {
		trimmed := strings.TrimSpace(line)
		parts := strings.Split(trimmed, "│")
		for _, part := range parts {
			candidate := strings.TrimSpace(part)
			if len(candidate) == 32 && isHex(candidate) {
				return candidate
			}
		}
	}
	return ""
}

func isHex(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}
