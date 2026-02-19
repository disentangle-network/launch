package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusPending    Status = "pending"
	StatusInProgress Status = "in_progress"
	StatusCompleted  Status = "completed"
	StatusFailed     Status = "failed"
)

type StageState struct {
	Status      Status                 `json:"status"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Output      map[string]interface{} `json:"output,omitempty"`
}

type PipelineState struct {
	PipelineID   string                `json:"pipeline_id"`
	StartedAt    time.Time             `json:"started_at"`
	CurrentStage string                `json:"current_stage"`
	Stages       map[string]StageState `json:"stages"`
}

var StageOrder = []string{"discover", "infra", "secrets", "deploy"}

func DefaultStatePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, ".config", "launch", "state.json"), nil
}

func New() *PipelineState {
	stages := make(map[string]StageState, len(StageOrder))
	for _, name := range StageOrder {
		stages[name] = StageState{Status: StatusPending}
	}
	return &PipelineState{
		PipelineID: uuid.New().String(),
		StartedAt:  time.Now().UTC(),
		Stages:     stages,
	}
}

func Load(path string) (*PipelineState, error) {
	if path == "" {
		var err error
		path, err = DefaultStatePath()
		if err != nil {
			return nil, err
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading state %s: %w", path, err)
	}
	var ps PipelineState
	if err := json.Unmarshal(data, &ps); err != nil {
		return nil, fmt.Errorf("parsing state %s: %w", path, err)
	}
	return &ps, nil
}

func Save(ps *PipelineState, path string) error {
	if path == "" {
		var err error
		path, err = DefaultStatePath()
		if err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating state directory: %w", err)
	}
	data, err := json.MarshalIndent(ps, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling state: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing state %s: %w", path, err)
	}
	return nil
}

func (ps *PipelineState) StartStage(name string) {
	now := time.Now().UTC()
	s := ps.Stages[name]
	s.Status = StatusInProgress
	s.StartedAt = &now
	ps.Stages[name] = s
	ps.CurrentStage = name
}

func (ps *PipelineState) CompleteStage(name string, output map[string]interface{}) {
	now := time.Now().UTC()
	s := ps.Stages[name]
	s.Status = StatusCompleted
	s.CompletedAt = &now
	s.Output = output
	ps.Stages[name] = s
}

func (ps *PipelineState) FailStage(name string, err error) {
	s := ps.Stages[name]
	s.Status = StatusFailed
	s.Error = err.Error()
	ps.Stages[name] = s
}

// NextPendingStage returns the first stage that is not completed.
func (ps *PipelineState) NextPendingStage() string {
	for _, name := range StageOrder {
		s := ps.Stages[name]
		if s.Status != StatusCompleted {
			return name
		}
	}
	return ""
}

// AllCompleted returns true if every stage is completed.
func (ps *PipelineState) AllCompleted() bool {
	for _, name := range StageOrder {
		if ps.Stages[name].Status != StatusCompleted {
			return false
		}
	}
	return true
}
