package daemon_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/vladimirvivien/robo/internal/daemon"
)

func TestState_SaveLoadRemove(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daemon.json")

	initial := daemon.State{
		Port:      8765,
		PID:       12345,
		AuthToken: "token-abc-123",
		Model:     "test-model",
		StartedAt: time.Now().Truncate(time.Second),
		LastTouch: time.Now().Truncate(time.Second),
	}

	if err := daemon.SaveState(path, initial); err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	loaded, err := daemon.LoadState(path)
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	if loaded.Port != initial.Port {
		t.Errorf("expected port %d, got %d", initial.Port, loaded.Port)
	}
	if loaded.AuthToken != initial.AuthToken {
		t.Errorf("expected token %s, got %s", initial.AuthToken, loaded.AuthToken)
	}
	if loaded.PID != initial.PID {
		t.Errorf("expected PID %d, got %d", initial.PID, loaded.PID)
	}

	if err := daemon.RemoveState(path); err != nil {
		t.Fatalf("RemoveState failed: %v", err)
	}

	_, err = daemon.LoadState(path)
	if err == nil {
		t.Fatal("expected error loading removed state, got nil")
	}
}
