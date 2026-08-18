package history_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/vladimirvivien/robo/internal/history"
)

func TestStore_CreateAndGetSession(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	store, err := history.NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	sess, err := store.CreateSession(ctx, "test-thread", "thread")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	if sess.ID == "" || sess.Name != "test-thread" || sess.Mode != "thread" {
		t.Errorf("unexpected session created: %+v", sess)
	}

	retrieved, err := store.GetSession(ctx, sess.ID)
	if err != nil {
		t.Fatalf("GetSession failed: %v", err)
	}

	if retrieved.ID != sess.ID || retrieved.Name != sess.Name {
		t.Errorf("retrieved mismatch: got %+v, want %+v", retrieved, sess)
	}

	byName, err := store.GetSessionByName(ctx, "test-thread")
	if err != nil {
		t.Fatalf("GetSessionByName failed: %v", err)
	}
	if byName.ID != sess.ID {
		t.Errorf("GetSessionByName mismatch: got %+v, want %+v", byName, sess)
	}
}

func TestStore_GetOrCreateDailySession(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	store, err := history.NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	s1, err := store.GetOrCreateDailySession(ctx)
	if err != nil {
		t.Fatalf("GetOrCreateDailySession first call failed: %v", err)
	}

	s2, err := store.GetOrCreateDailySession(ctx)
	if err != nil {
		t.Fatalf("GetOrCreateDailySession second call failed: %v", err)
	}

	if s1.ID != s2.ID {
		t.Errorf("expected same daily session ID, got %s and %s", s1.ID, s2.ID)
	}
	if s1.Mode != "daily" {
		t.Errorf("expected mode daily, got %s", s1.Mode)
	}
}

func TestStore_AppendAndGetMessages(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	store, err := history.NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	sess, err := store.CreateSession(ctx, "chat-thread", "thread")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	t0 := time.Now().Add(-2 * time.Minute)
	t1 := time.Now().Add(-1 * time.Minute)
	t2 := time.Now()

	_, err = store.AppendMessage(ctx, sess.ID, history.Message{
		Role:      "user",
		Content:   "how do I use go channels?",
		CreatedAt: t0,
	})
	if err != nil {
		t.Fatalf("AppendMessage 1 failed: %v", err)
	}

	_, err = store.AppendMessage(ctx, sess.ID, history.Message{
		Role:       "assistant",
		Content:    "Channels are typed conduits for synchronization.",
		Provider:   "litertlm",
		Model:      "gemma3-1b-it-int4",
		TokensUsed: 12,
		CreatedAt:  t1,
	})
	if err != nil {
		t.Fatalf("AppendMessage 2 failed: %v", err)
	}

	_, err = store.AppendMessage(ctx, sess.ID, history.Message{
		Role:      "user",
		Content:   "show me buffered channel example",
		CreatedAt: t2,
	})
	if err != nil {
		t.Fatalf("AppendMessage 3 failed: %v", err)
	}

	// 1. Get all messages
	all, err := store.GetMessages(ctx, sess.ID, 0)
	if err != nil {
		t.Fatalf("GetMessages all failed: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(all))
	}
	if all[0].Content != "how do I use go channels?" || all[2].Content != "show me buffered channel example" {
		t.Errorf("unexpected message order: %+v", all)
	}

	// 2. Get last 2 messages (sliding window)
	last2, err := store.GetMessages(ctx, sess.ID, 2)
	if err != nil {
		t.Fatalf("GetMessages limit 2 failed: %v", err)
	}
	if len(last2) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(last2))
	}
	if last2[0].Role != "assistant" || last2[1].Role != "user" {
		t.Errorf("unexpected sliding window messages: %+v", last2)
	}
}

func TestStore_ListSessions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	store, err := history.NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	_, _ = store.CreateSession(ctx, "session-A", "thread")
	time.Sleep(10 * time.Millisecond)
	_, _ = store.CreateSession(ctx, "session-B", "thread")

	list, err := store.ListSessions(ctx, 10)
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}

	if len(list) < 2 {
		t.Fatalf("expected at least 2 sessions, got %d", len(list))
	}
	if list[0].Name != "session-B" || list[1].Name != "session-A" {
		t.Errorf("expected most recent first, got: %s, %s", list[0].Name, list[1].Name)
	}
}

func TestStore_DeleteSession_Cascade(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	store, err := history.NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	sess, _ := store.CreateSession(ctx, "to-delete", "thread")
	_, _ = store.AppendMessage(ctx, sess.ID, history.Message{Role: "user", Content: "hello"})

	if err := store.DeleteSession(ctx, sess.ID); err != nil {
		t.Fatalf("DeleteSession failed: %v", err)
	}

	// Verify session is gone
	_, err = store.GetSession(ctx, sess.ID)
	if err == nil {
		t.Error("expected error getting deleted session, got nil")
	}

	// Verify messages are cascaded
	msgs, err := store.GetMessages(ctx, sess.ID, 0)
	if err != nil {
		t.Fatalf("GetMessages error: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 cascaded messages, got %d", len(msgs))
	}
}

func TestStore_FileBasedPersistence(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "history.db")
	ctx := context.Background()

	// 1. Write session in first instance
	store1, err := history.NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore 1 failed: %v", err)
	}
	sess, err := store1.CreateSession(ctx, "persisted-thread", "thread")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	_, err = store1.AppendMessage(ctx, sess.ID, history.Message{Role: "user", Content: "persisted query"})
	if err != nil {
		t.Fatalf("AppendMessage failed: %v", err)
	}
	_ = store1.Close()

	// 2. Reopen database in second instance
	store2, err := history.NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore 2 failed: %v", err)
	}
	defer func() { _ = store2.Close() }()

	reopenedSess, err := store2.GetSession(ctx, sess.ID)
	if err != nil {
		t.Fatalf("GetSession reopened failed: %v", err)
	}
	if reopenedSess.Name != "persisted-thread" {
		t.Errorf("expected name 'persisted-thread', got %s", reopenedSess.Name)
	}

	msgs, err := store2.GetMessages(ctx, sess.ID, 0)
	if err != nil {
		t.Fatalf("GetMessages reopened failed: %v", err)
	}
	if len(msgs) != 1 || msgs[0].Content != "persisted query" {
		t.Errorf("unexpected reopened messages: %+v", msgs)
	}
}
