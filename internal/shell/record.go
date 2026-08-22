package shell

import (
	"context"
	"time"

	"github.com/vladimirvivien/robo/internal/store"
)

// ExecutionRecord stores metadata of the last executed command for contextual multi-turn diagnosis.
type ExecutionRecord struct {
	Prompt      string    `json:"prompt,omitempty"`
	Command     string    `json:"command"`
	Description string    `json:"description,omitempty"`
	Output      string    `json:"output,omitempty"`
	Error       string    `json:"error,omitempty"`
	ExitCode    int       `json:"exit_code"`
	Timestamp   time.Time `json:"timestamp"`
	Cwd         string    `json:"cwd,omitempty"`
}

// SaveLastExecution writes the execution record to the SQLite store (~/.robo/robo.db).
func SaveLastExecution(rec ExecutionRecord) error {
	s, err := store.Open("")
	if err != nil {
		return err
	}
	defer func() { _ = s.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	return s.RecordExecution(ctx, store.Execution{
		Prompt:      rec.Prompt,
		Command:     rec.Command,
		Description: rec.Description,
		Stdout:      rec.Output,
		Stderr:      rec.Error,
		ExitCode:    rec.ExitCode,
		Cwd:         rec.Cwd,
		Shell:       string(DetectShell()),
		CreatedAt:   rec.Timestamp,
	})
}

// LoadLastExecution reads the most recent execution record for the given cwd (or global) from ~/.robo/robo.db.
func LoadLastExecution(cwd string) (*ExecutionRecord, error) {
	s, err := store.Open("")
	if err != nil {
		return nil, err
	}
	defer func() { _ = s.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	exec, err := s.GetLastExecution(ctx, cwd, 24*time.Hour)
	if err != nil || exec == nil {
		return nil, err
	}

	out := exec.Stdout
	if out == "" {
		out = exec.Stderr
	}

	return &ExecutionRecord{
		Prompt:      exec.Prompt,
		Command:     exec.Command,
		Description: exec.Description,
		Output:      out,
		Error:       exec.Stderr,
		ExitCode:    exec.ExitCode,
		Timestamp:   exec.CreatedAt,
		Cwd:         exec.Cwd,
	}, nil
}
