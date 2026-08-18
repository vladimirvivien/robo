package history

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

// Session represents a persistent conversation thread.
type Session struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Mode      string    `json:"mode"` // "daily", "thread"
	StartedAt time.Time `json:"started_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Message represents a single conversational turn in a session.
type Message struct {
	ID         string    `json:"id"`
	SessionID  string    `json:"session_id"`
	Role       string    `json:"role"` // "user", "assistant", "system"
	Content    string    `json:"content"`
	Provider   string    `json:"provider,omitempty"`
	Model      string    `json:"model,omitempty"`
	TokensUsed int       `json:"tokens_used,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// Store provides access to SQLite-backed REPL conversation history.
type Store struct {
	db *sql.DB
}

// DefaultDBPath returns the standard location for history.db (~/.config/robo/history.db).
func DefaultDBPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "history.db"
	}
	return filepath.Join(home, ".config", "robo", "history.db")
}

// NewStore creates a new Store and initializes the SQLite schema.
func NewStore(dbPath string) (*Store, error) {
	if dbPath == "" {
		dbPath = DefaultDBPath()
	}

	if dbPath != ":memory:" && !filepath.IsAbs(dbPath) && dbPath[0] != '.' {
		if err := os.MkdirAll(filepath.Dir(dbPath), 0750); err != nil {
			return nil, fmt.Errorf("history: create config dir: %w", err)
		}
	} else if dbPath != ":memory:" && filepath.IsAbs(dbPath) {
		if err := os.MkdirAll(filepath.Dir(dbPath), 0750); err != nil {
			return nil, fmt.Errorf("history: create dir: %w", err)
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("history: open sqlite: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("history: migrate schema: %w", err)
	}

	return s, nil
}

func (s *Store) migrate(ctx context.Context) error {
	schema := `
	PRAGMA foreign_keys = ON;

	CREATE TABLE IF NOT EXISTS sessions (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		mode TEXT NOT NULL,
		started_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_sessions_name ON sessions(name);
	CREATE INDEX IF NOT EXISTS idx_sessions_updated ON sessions(updated_at);

	CREATE TABLE IF NOT EXISTS messages (
		id TEXT PRIMARY KEY,
		session_id TEXT NOT NULL,
		role TEXT NOT NULL,
		content TEXT NOT NULL,
		provider TEXT NOT NULL DEFAULT '',
		model TEXT NOT NULL DEFAULT '',
		tokens_used INTEGER NOT NULL DEFAULT 0,
		created_at TIMESTAMP NOT NULL,
		FOREIGN KEY(session_id) REFERENCES sessions(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id, created_at);
	`
	_, err := s.db.ExecContext(ctx, schema)
	return err
}

// CreateSession creates and stores a new conversation session.
func (s *Store) CreateSession(ctx context.Context, name, mode string) (*Session, error) {
	id := uuid.New().String()
	if name == "" {
		name = id
	}
	if mode == "" {
		mode = "thread"
	}

	now := time.Now().UTC()
	sess := &Session{
		ID:        id,
		Name:      name,
		Mode:      mode,
		StartedAt: now,
		UpdatedAt: now,
	}

	query := `INSERT INTO sessions (id, name, mode, started_at, updated_at) VALUES (?, ?, ?, ?, ?)`
	_, err := s.db.ExecContext(ctx, query, sess.ID, sess.Name, sess.Mode, sess.StartedAt, sess.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("history: insert session: %w", err)
	}

	return sess, nil
}

// GetOrCreateDailySession retrieves or creates the standard daily session for today (e.g. "daily-2026-08-18").
func (s *Store) GetOrCreateDailySession(ctx context.Context) (*Session, error) {
	todayName := fmt.Sprintf("daily-%s", time.Now().UTC().Format("2006-01-02"))

	sess, err := s.GetSessionByName(ctx, todayName)
	if err == nil && sess != nil {
		return sess, nil
	}

	return s.CreateSession(ctx, todayName, "daily")
}

// GetSession retrieves a session by its unique ID.
func (s *Store) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	query := `SELECT id, name, mode, started_at, updated_at FROM sessions WHERE id = ?`
	row := s.db.QueryRowContext(ctx, query, sessionID)

	var sess Session
	err := row.Scan(&sess.ID, &sess.Name, &sess.Mode, &sess.StartedAt, &sess.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("history: session not found: %s", sessionID)
	}
	if err != nil {
		return nil, fmt.Errorf("history: get session: %w", err)
	}

	return &sess, nil
}

// GetSessionByName retrieves the most recent session with the specified name.
func (s *Store) GetSessionByName(ctx context.Context, name string) (*Session, error) {
	query := `SELECT id, name, mode, started_at, updated_at FROM sessions WHERE name = ? ORDER BY updated_at DESC LIMIT 1`
	row := s.db.QueryRowContext(ctx, query, name)

	var sess Session
	err := row.Scan(&sess.ID, &sess.Name, &sess.Mode, &sess.StartedAt, &sess.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("history: session name not found: %s", name)
	}
	if err != nil {
		return nil, fmt.Errorf("history: get session by name: %w", err)
	}

	return &sess, nil
}

// AppendMessage records a new turn in the given session and updates the session's timestamp.
func (s *Store) AppendMessage(ctx context.Context, sessionID string, msg Message) (*Message, error) {
	if sessionID == "" {
		return nil, errors.New("history: sessionID cannot be empty")
	}

	if msg.ID == "" {
		msg.ID = uuid.New().String()
	}
	msg.SessionID = sessionID
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now().UTC()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("history: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	insertQuery := `
	INSERT INTO messages (id, session_id, role, content, provider, model, tokens_used, created_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err = tx.ExecContext(ctx, insertQuery,
		msg.ID, msg.SessionID, msg.Role, msg.Content, msg.Provider, msg.Model, msg.TokensUsed, msg.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("history: insert message: %w", err)
	}

	updateQuery := `UPDATE sessions SET updated_at = ? WHERE id = ?`
	_, err = tx.ExecContext(ctx, updateQuery, msg.CreatedAt, sessionID)
	if err != nil {
		return nil, fmt.Errorf("history: update session timestamp: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("history: commit tx: %w", err)
	}

	return &msg, nil
}

// GetMessages retrieves messages for a session ordered chronologically.
// If limit > 0, returns the most recent 'limit' messages.
func (s *Store) GetMessages(ctx context.Context, sessionID string, limit int) ([]Message, error) {
	var query string
	var rows *sql.Rows
	var err error

	if limit > 0 {
		query = `
		SELECT id, session_id, role, content, provider, model, tokens_used, created_at
		FROM (
			SELECT id, session_id, role, content, provider, model, tokens_used, created_at
			FROM messages
			WHERE session_id = ?
			ORDER BY created_at DESC
			LIMIT ?
		) sub
		ORDER BY created_at ASC
		`
		rows, err = s.db.QueryContext(ctx, query, sessionID, limit)
	} else {
		query = `
		SELECT id, session_id, role, content, provider, model, tokens_used, created_at
		FROM messages
		WHERE session_id = ?
		ORDER BY created_at ASC
		`
		rows, err = s.db.QueryContext(ctx, query, sessionID)
	}

	if err != nil {
		return nil, fmt.Errorf("history: query messages: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var messages []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &m.Provider, &m.Model, &m.TokensUsed, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("history: scan message: %w", err)
		}
		messages = append(messages, m)
	}

	return messages, rows.Err()
}

// ListSessions lists stored sessions ordered by most recently updated.
func (s *Store) ListSessions(ctx context.Context, limit int) ([]Session, error) {
	query := `SELECT id, name, mode, started_at, updated_at FROM sessions ORDER BY updated_at DESC`
	var rows *sql.Rows
	var err error

	if limit > 0 {
		query += " LIMIT ?"
		rows, err = s.db.QueryContext(ctx, query, limit)
	} else {
		rows, err = s.db.QueryContext(ctx, query)
	}

	if err != nil {
		return nil, fmt.Errorf("history: query sessions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var sessions []Session
	for rows.Next() {
		var sess Session
		if err := rows.Scan(&sess.ID, &sess.Name, &sess.Mode, &sess.StartedAt, &sess.UpdatedAt); err != nil {
			return nil, fmt.Errorf("history: scan session: %w", err)
		}
		sessions = append(sessions, sess)
	}

	return sessions, rows.Err()
}

// DeleteSession removes a session and cascades deletion of all its messages.
func (s *Store) DeleteSession(ctx context.Context, sessionID string) error {
	query := `DELETE FROM sessions WHERE id = ?`
	res, err := s.db.ExecContext(ctx, query, sessionID)
	if err != nil {
		return fmt.Errorf("history: delete session: %w", err)
	}

	affected, _ := res.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("history: session not found: %s", sessionID)
	}
	return nil
}

// Close closes the underlying SQLite database connection.
func (s *Store) Close() error {
	return s.db.Close()
}
