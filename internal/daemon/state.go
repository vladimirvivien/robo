package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const StateFilename = "robod.json"

// State represents the active daemon metadata on disk.
type State struct {
	URL       string    `json:"url"`
	Port      int       `json:"port"`
	PID       int       `json:"pid"`
	Model     string    `json:"model"`
	StartedAt time.Time `json:"started_at"`
	LastTouch time.Time `json:"last_touch"`
}

// StatePath returns the path to robod.json in the user's ~/.robo directory.
func StatePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return StateFilename
	}
	return filepath.Join(home, ".robo", StateFilename)
}

// SaveState writes the daemon state to disk at path (or default if path is empty).
func SaveState(path string, s State) error {
	if path == "" {
		path = StatePath()
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("daemon state: create dir %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("daemon state: marshal: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("daemon state: write %s: %w", path, err)
	}

	return nil
}

// LoadState reads the daemon state from disk at path (or default if path is empty).
func LoadState(path string) (*State, error) {
	if path == "" {
		path = StatePath()
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("daemon state: parse %s: %w", path, err)
	}

	return &s, nil
}

// RemoveState removes the daemon state file from disk.
func RemoveState(path string) error {
	if path == "" {
		path = StatePath()
	}

	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
