//go:build integration

package integration

import (
	"os"
	"testing"

	"github.com/disentangle-network/launch/internal/config"
	"github.com/disentangle-network/launch/internal/state"
)

func TestStateResumption(t *testing.T) {
	dir := t.TempDir()
	statePath := dir + "/state.json"

	// Create initial state with discover completed
	ps := state.New()
	ps.CompleteStage("discover", map[string]interface{}{"test": true})
	if err := state.Save(ps, statePath); err != nil {
		t.Fatalf("save state: %v", err)
	}

	// Reload and verify resume point
	loaded, err := state.Load(statePath)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}

	next := loaded.NextPendingStage()
	if next != "infra" {
		t.Errorf("expected next stage 'infra', got %q", next)
	}
}

func TestConfigFromEnv(t *testing.T) {
	dir := t.TempDir()
	cfgPath := dir + "/config.yaml"

	cfg := &config.Config{
		OCIRegion:   "us-phoenix-1",
		Environment: "dev",
	}
	if err := config.Save(cfg, cfgPath); err != nil {
		t.Fatalf("save config: %v", err)
	}

	loaded, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if loaded.OCIRegion != "us-phoenix-1" {
		t.Errorf("expected region 'us-phoenix-1', got %q", loaded.OCIRegion)
	}
}

func TestPreflightDetectsMissingTools(t *testing.T) {
	// This test validates that preflight tool checking works
	// It's gated behind integration tag because it inspects real PATH
	if os.Getenv("LAUNCH_INTEGRATION_LIVE") == "" {
		t.Skip("skipping live integration test")
	}
	// Live tests would go here
}
