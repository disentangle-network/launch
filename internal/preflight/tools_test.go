package preflight

import "testing"

func TestHasAllRequiredTrue(t *testing.T) {
	results := []ToolResult{
		{Name: "kubectl", Required: true, Available: true},
		{Name: "helm", Required: true, Available: true},
		{Name: "sops", Required: false, Available: false},
	}
	if !HasAllRequired(results) {
		t.Error("HasAllRequired should be true when all required tools are available")
	}
}

func TestHasAllRequiredFalse(t *testing.T) {
	results := []ToolResult{
		{Name: "kubectl", Required: true, Available: true},
		{Name: "helm", Required: true, Available: false},
	}
	if HasAllRequired(results) {
		t.Error("HasAllRequired should be false when a required tool is missing")
	}
}

func TestMissingRequired(t *testing.T) {
	results := []ToolResult{
		{Name: "kubectl", Required: true, Available: true},
		{Name: "helm", Required: true, Available: false},
		{Name: "sops", Required: false, Available: false},
		{Name: "flux", Required: true, Available: false},
	}
	missing := MissingRequired(results)
	if len(missing) != 2 {
		t.Fatalf("expected 2 missing, got %d: %v", len(missing), missing)
	}
	if missing[0] != "helm" || missing[1] != "flux" {
		t.Errorf("expected [helm flux], got %v", missing)
	}
}

func TestMissingRequiredNone(t *testing.T) {
	results := []ToolResult{
		{Name: "kubectl", Required: true, Available: true},
	}
	missing := MissingRequired(results)
	if len(missing) != 0 {
		t.Errorf("expected no missing, got %v", missing)
	}
}

func TestToolCheckFields(t *testing.T) {
	tc := ToolCheck{Name: "test", Command: "test-cmd", Args: []string{"-v"}, Required: true}
	if tc.Name != "test" || tc.Command != "test-cmd" || !tc.Required {
		t.Error("ToolCheck fields not set correctly")
	}
}

func TestRequiredToolsNotEmpty(t *testing.T) {
	if len(RequiredTools) == 0 {
		t.Error("RequiredTools should not be empty")
	}
}
