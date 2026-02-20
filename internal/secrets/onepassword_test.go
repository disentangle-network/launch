package secrets

import (
	"os"
	"testing"
)

func TestOpReadNoOpCLI(t *testing.T) {
	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", "/nonexistent")
	defer os.Setenv("PATH", oldPath)

	_, err := OpRead("op://vault/item/field")
	if err == nil {
		t.Error("expected error when op CLI not installed")
	}
	if err.Error() != "1Password CLI (op) not installed" {
		t.Errorf("unexpected error message: %v", err)
	}
}
