package store_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/vladimirvivien/robo/internal/store"
)

func TestStore_RecordAndGetLastExecution(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test_robo.db")

	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = s.Close() }()

	ctx := context.Background()

	// Initial query should return nil
	exec, err := s.GetLastExecution(ctx, dir, 24*time.Hour)
	if err != nil {
		t.Fatalf("GetLastExecution on empty db failed: %v", err)
	}
	if exec != nil {
		t.Fatalf("expected nil execution, got %+v", exec)
	}

	// Insert execution in dir A
	execA := store.Execution{
		Prompt:      "run unit tests",
		Command:     "go test ./...",
		Description: "Run all Go unit tests",
		Stdout:      "FAIL: internal/store_test",
		Stderr:      "",
		ExitCode:    1,
		Cwd:         filepath.Join(dir, "projectA"),
		Shell:       "powershell",
		Provider:    "litertlm",
		Model:       "gemma-4-2b",
		CreatedAt:   time.Now().Add(-10 * time.Minute),
	}

	if err := s.RecordExecution(ctx, execA); err != nil {
		t.Fatalf("RecordExecution failed: %v", err)
	}

	// Insert execution in dir B
	execB := store.Execution{
		Prompt:      "add ssh key",
		Command:     "ssh-add ~/.ssh/id_ed25519",
		Description: "Add key to keychain",
		Stdout:      "",
		Stderr:      "Could not open a connection to your authentication agent.",
		ExitCode:    2,
		Cwd:         filepath.Join(dir, "projectB"),
		Shell:       "bash",
		Provider:    "litertlm",
		Model:       "gemma-4-2b",
		CreatedAt:   time.Now(),
	}

	if err := s.RecordExecution(ctx, execB); err != nil {
		t.Fatalf("RecordExecution failed: %v", err)
	}

	// Query for dir A should return execA
	foundA, err := s.GetLastExecution(ctx, filepath.Join(dir, "projectA"), 24*time.Hour)
	if err != nil {
		t.Fatalf("GetLastExecution for projectA failed: %v", err)
	}
	if foundA == nil || foundA.Command != "go test ./..." {
		t.Fatalf("expected execA in projectA, got %+v", foundA)
	}
	if foundA.ExitCode != 1 {
		t.Errorf("expected exit code 1, got %d", foundA.ExitCode)
	}

	// Query for dir B should return execB
	foundB, err := s.GetLastExecution(ctx, filepath.Join(dir, "projectB"), 24*time.Hour)
	if err != nil {
		t.Fatalf("GetLastExecution for projectB failed: %v", err)
	}
	if foundB == nil || foundB.Command != "ssh-add ~/.ssh/id_ed25519" {
		t.Fatalf("expected execB in projectB, got %+v", foundB)
	}
	if foundB.Stderr != "Could not open a connection to your authentication agent." {
		t.Errorf("unexpected stderr: %s", foundB.Stderr)
	}

	// Query recent executions
	recent, err := s.GetRecentExecutions(ctx, 10)
	if err != nil {
		t.Fatalf("GetRecentExecutions failed: %v", err)
	}
	if len(recent) != 2 {
		t.Fatalf("expected 2 recent executions, got %d", len(recent))
	}
	if recent[0].Command != "ssh-add ~/.ssh/id_ed25519" {
		t.Errorf("expected most recent to be ssh-add, got %s", recent[0].Command)
	}
}

func TestStore_ResetDB(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "reset_robo.db")

	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	ctx := context.Background()
	exec := store.Execution{
		Prompt:   "hello",
		Command:  "echo hello",
		ExitCode: 0,
		Cwd:      dir,
		Shell:    "bash",
	}
	if err := s.RecordExecution(ctx, exec); err != nil {
		t.Fatalf("RecordExecution failed: %v", err)
	}
	_ = s.Close()

	if err := store.ResetDB(dbPath); err != nil {
		t.Fatalf("ResetDB failed: %v", err)
	}

	s2, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open after ResetDB failed: %v", err)
	}
	defer func() { _ = s2.Close() }()

	recent, err := s2.GetRecentExecutions(ctx, 10)
	if err != nil {
		t.Fatalf("GetRecentExecutions failed: %v", err)
	}
	if len(recent) != 0 {
		t.Fatalf("expected 0 executions after reset, got %d", len(recent))
	}
}
