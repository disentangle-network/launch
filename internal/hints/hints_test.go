package hints

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestPrintEmpty(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	Print(nil)

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	if buf.String() != "" {
		t.Errorf("Print(nil) should produce no output, got %q", buf.String())
	}
}

func TestPrintSteps(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	Print([]NextStep{
		{Command: "infra plan", Description: "Preview changes"},
		{Command: "cluster add dev", Description: "Register cluster"},
	})

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "Next steps") {
		t.Error("output should contain 'Next steps' header")
	}
	if !strings.Contains(output, "launch infra plan") {
		t.Error("output should contain 'launch infra plan'")
	}
	if !strings.Contains(output, "Preview changes") {
		t.Error("output should contain description")
	}
	if !strings.Contains(output, "launch cluster add dev") {
		t.Error("output should contain second command")
	}
}

func TestNextStepFields(t *testing.T) {
	s := NextStep{Command: "test", Description: "desc"}
	if s.Command != "test" {
		t.Errorf("Command = %q, want %q", s.Command, "test")
	}
	if s.Description != "desc" {
		t.Errorf("Description = %q, want %q", s.Description, "desc")
	}
}
