package state

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestNewCreatesAllStagesPending(t *testing.T) {
	ps := New()

	if ps.PipelineID == "" {
		t.Error("PipelineID should not be empty")
	}
	if ps.StartedAt.IsZero() {
		t.Error("StartedAt should not be zero")
	}

	for _, name := range StageOrder {
		stage, ok := ps.Stages[name]
		if !ok {
			t.Errorf("stage %q not found in Stages map", name)
			continue
		}
		if stage.Status != StatusPending {
			t.Errorf("stage %q status = %q, want %q", name, stage.Status, StatusPending)
		}
	}

	if len(ps.Stages) != len(StageOrder) {
		t.Errorf("Stages has %d entries, want %d", len(ps.Stages), len(StageOrder))
	}
}

func TestStartStage(t *testing.T) {
	ps := New()
	ps.StartStage("discover")

	s := ps.Stages["discover"]
	if s.Status != StatusInProgress {
		t.Errorf("status = %q, want %q", s.Status, StatusInProgress)
	}
	if s.StartedAt == nil {
		t.Error("StartedAt should not be nil after StartStage")
	}
	if ps.CurrentStage != "discover" {
		t.Errorf("CurrentStage = %q, want %q", ps.CurrentStage, "discover")
	}
}

func TestCompleteStage(t *testing.T) {
	ps := New()
	ps.StartStage("discover")

	output := map[string]interface{}{
		"vcn_id": "ocid1.vcn.test",
	}
	ps.CompleteStage("discover", output)

	s := ps.Stages["discover"]
	if s.Status != StatusCompleted {
		t.Errorf("status = %q, want %q", s.Status, StatusCompleted)
	}
	if s.CompletedAt == nil {
		t.Error("CompletedAt should not be nil after CompleteStage")
	}
	if s.Output == nil {
		t.Error("Output should not be nil after CompleteStage")
	}
	if s.Output["vcn_id"] != "ocid1.vcn.test" {
		t.Errorf("Output[vcn_id] = %v, want %q", s.Output["vcn_id"], "ocid1.vcn.test")
	}
}

func TestCompleteStageWithNilOutput(t *testing.T) {
	ps := New()
	ps.StartStage("infra")
	ps.CompleteStage("infra", nil)

	s := ps.Stages["infra"]
	if s.Status != StatusCompleted {
		t.Errorf("status = %q, want %q", s.Status, StatusCompleted)
	}
	if s.CompletedAt == nil {
		t.Error("CompletedAt should not be nil")
	}
}

func TestFailStage(t *testing.T) {
	ps := New()
	ps.StartStage("infra")
	ps.FailStage("infra", errors.New("terraform apply failed"))

	s := ps.Stages["infra"]
	if s.Status != StatusFailed {
		t.Errorf("status = %q, want %q", s.Status, StatusFailed)
	}
	if s.Error != "terraform apply failed" {
		t.Errorf("Error = %q, want %q", s.Error, "terraform apply failed")
	}
}

func TestNextPendingStage(t *testing.T) {
	ps := New()

	// All pending, should return first stage.
	next := ps.NextPendingStage()
	if next != "discover" {
		t.Errorf("NextPendingStage() = %q, want %q", next, "discover")
	}

	// Complete first two stages.
	ps.CompleteStage("discover", nil)
	ps.CompleteStage("infra", nil)

	next = ps.NextPendingStage()
	if next != "secrets" {
		t.Errorf("NextPendingStage() = %q, want %q", next, "secrets")
	}

	// Complete remaining stages.
	ps.CompleteStage("secrets", nil)
	ps.CompleteStage("deploy", nil)

	next = ps.NextPendingStage()
	if next != "" {
		t.Errorf("NextPendingStage() = %q, want empty string", next)
	}
}

func TestNextPendingStageReturnsFailed(t *testing.T) {
	ps := New()
	ps.CompleteStage("discover", nil)
	ps.StartStage("infra")
	ps.FailStage("infra", errors.New("failed"))

	// Failed stages are not completed, so NextPendingStage should return them.
	next := ps.NextPendingStage()
	if next != "infra" {
		t.Errorf("NextPendingStage() = %q, want %q", next, "infra")
	}
}

func TestAllCompleted(t *testing.T) {
	ps := New()

	if ps.AllCompleted() {
		t.Error("AllCompleted() should be false when stages are pending")
	}

	for _, name := range StageOrder {
		ps.CompleteStage(name, nil)
	}

	if !ps.AllCompleted() {
		t.Error("AllCompleted() should be true when all stages are completed")
	}
}

func TestAllCompletedFalseWhenOneFailed(t *testing.T) {
	ps := New()
	ps.CompleteStage("discover", nil)
	ps.CompleteStage("infra", nil)
	ps.CompleteStage("secrets", nil)
	ps.FailStage("deploy", errors.New("helm failed"))

	if ps.AllCompleted() {
		t.Error("AllCompleted() should be false when a stage is failed")
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")

	original := New()
	original.StartStage("discover")
	original.CompleteStage("discover", map[string]interface{}{
		"vcn_id": "ocid1.vcn.test",
		"count":  float64(42),
	})
	original.StartStage("infra")
	original.FailStage("infra", errors.New("terraform failed"))

	if err := Save(original, statePath); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	loaded, err := Load(statePath)
	if err != nil {
		t.Fatalf("Load(%q) returned error: %v", statePath, err)
	}
	if loaded == nil {
		t.Fatal("Load() returned nil")
	}

	if loaded.PipelineID != original.PipelineID {
		t.Errorf("PipelineID = %q, want %q", loaded.PipelineID, original.PipelineID)
	}
	if !loaded.StartedAt.Equal(original.StartedAt) {
		t.Errorf("StartedAt = %v, want %v", loaded.StartedAt, original.StartedAt)
	}
	if loaded.CurrentStage != original.CurrentStage {
		t.Errorf("CurrentStage = %q, want %q", loaded.CurrentStage, original.CurrentStage)
	}

	// Verify discover stage.
	disc := loaded.Stages["discover"]
	if disc.Status != StatusCompleted {
		t.Errorf("discover status = %q, want %q", disc.Status, StatusCompleted)
	}
	if disc.StartedAt == nil {
		t.Error("discover StartedAt should not be nil")
	}
	if disc.CompletedAt == nil {
		t.Error("discover CompletedAt should not be nil")
	}
	if disc.Output["vcn_id"] != "ocid1.vcn.test" {
		t.Errorf("discover Output[vcn_id] = %v, want %q", disc.Output["vcn_id"], "ocid1.vcn.test")
	}
	// JSON unmarshals numbers as float64.
	if disc.Output["count"] != float64(42) {
		t.Errorf("discover Output[count] = %v, want %v", disc.Output["count"], float64(42))
	}

	// Verify infra stage.
	infra := loaded.Stages["infra"]
	if infra.Status != StatusFailed {
		t.Errorf("infra status = %q, want %q", infra.Status, StatusFailed)
	}
	if infra.Error != "terraform failed" {
		t.Errorf("infra Error = %q, want %q", infra.Error, "terraform failed")
	}

	// Verify pending stages survived round-trip.
	for _, name := range []string{"secrets", "deploy"} {
		s := loaded.Stages[name]
		if s.Status != StatusPending {
			t.Errorf("stage %q status = %q, want %q", name, s.Status, StatusPending)
		}
	}
}

func TestLoadNonExistentReturnsNil(t *testing.T) {
	ps, err := Load("/tmp/does-not-exist-launch-test-state.json")
	if err != nil {
		t.Fatalf("Load() returned error for non-existent file: %v", err)
	}
	if ps != nil {
		t.Error("Load() should return nil for non-existent file")
	}
}

func TestSaveCreatesParentDirectories(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "a", "b", "state.json")

	ps := New()
	if err := Save(ps, statePath); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	loaded, err := Load(statePath)
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if loaded == nil {
		t.Fatal("Load() returned nil after Save()")
	}
}

func TestStageOrder(t *testing.T) {
	expected := []string{"discover", "infra", "secrets", "deploy"}
	if len(StageOrder) != len(expected) {
		t.Fatalf("StageOrder has %d entries, want %d", len(StageOrder), len(expected))
	}
	for i, name := range expected {
		if StageOrder[i] != name {
			t.Errorf("StageOrder[%d] = %q, want %q", i, StageOrder[i], name)
		}
	}
}
