package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestCompletionBash(t *testing.T) {
	resetRootCmd()
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"completion", "bash"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("completion bash returned error: %v", err)
	}
}

func TestCompletionBashOutput(t *testing.T) {
	var buf bytes.Buffer
	err := rootCmd.GenBashCompletion(&buf)
	if err != nil {
		t.Fatalf("GenBashCompletion returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "bash") {
		t.Error("expected bash completion output to contain 'bash'")
	}
}

func TestCompletionZsh(t *testing.T) {
	resetRootCmd()
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"completion", "zsh"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("completion zsh returned error: %v", err)
	}
}

func TestCompletionZshOutput(t *testing.T) {
	var buf bytes.Buffer
	err := rootCmd.GenZshCompletion(&buf)
	if err != nil {
		t.Fatalf("GenZshCompletion returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "zsh") {
		t.Error("expected zsh completion output to contain 'zsh'")
	}
}

func TestCompletionFish(t *testing.T) {
	resetRootCmd()
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"completion", "fish"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("completion fish returned error: %v", err)
	}
}

func TestCompletionFishOutput(t *testing.T) {
	var buf bytes.Buffer
	err := rootCmd.GenFishCompletion(&buf, true)
	if err != nil {
		t.Fatalf("GenFishCompletion returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "fish") {
		t.Error("expected fish completion output to contain 'fish'")
	}
}

func TestCompletionPowershell(t *testing.T) {
	resetRootCmd()
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs([]string{"completion", "powershell"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("completion powershell returned error: %v", err)
	}
}

func TestCompletionPowershellOutput(t *testing.T) {
	var buf bytes.Buffer
	err := rootCmd.GenPowerShellCompletionWithDesc(&buf)
	if err != nil {
		t.Fatalf("GenPowerShellCompletionWithDesc returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "powershell") {
		t.Error("expected powershell completion output to contain 'powershell'")
	}
}

func TestCompletionInvalidShell(t *testing.T) {
	err := execRootCmd([]string{"completion", "invalid"})
	if err == nil {
		t.Fatal("expected error for invalid shell argument")
	}
}

func TestCompletionNoArgs(t *testing.T) {
	err := execRootCmd([]string{"completion"})
	if err == nil {
		t.Fatal("expected error when no shell argument is provided")
	}
}
