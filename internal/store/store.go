package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Execution represents a recorded shell action and its outcome.
type Execution struct {
	ID          int64     `json:"id"`
	SessionID   string    `json:"session_id,omitempty"`
	Prompt      string    `json:"prompt"`
	Command     string    `json:"command"`
	Description string    `json:"description,omitempty"`
	Stdout      string    `json:"stdout,omitempty"`
	Stderr      string    `json:"stderr,omitempty"`
	ExitCode    int       `json:"exit_code"`
	RiskTier    string    `json:"risk_tier,omitempty"`
	RiskScore   float64   `json:"risk_score,omitempty"`
	Cwd         string    `json:"cwd"`
	Shell       string    `json:"shell"`
	Provider    string    `json:"provider,omitempty"`
	Model       string    `json:"model,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// Store handles SQLite persistence for robo execution history.
type Store struct {
	db *sql.DB
}

// DefaultDBPath returns the standard path to ~/.robo/robo.db
func DefaultDBPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".robo", "robo.db")
	}
	return filepath.Join(home, ".robo", "robo.db")
}

// Open opens or initializes a SQLite database at dbPath (or default ~/.robo/robo.db).
func Open(dbPath string) (*Store, error) {
	if dbPath == "" {
		dbPath = DefaultDBPath()
	}

	if err := os.MkdirAll(filepath.Dir(dbPath), 0750); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	// Connect with WAL mode and busy timeout for concurrent safety across terminals
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)", filepath.ToSlash(dbPath))
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	s := &Store{db: db}
	if err := s.initSchema(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to init schema: %w", err)
	}

	return s, nil
}

// ResetDB removes any existing SQLite database files (including WAL/SHM) at dbPath and initializes a fresh schema.
func ResetDB(dbPath string) error {
	if dbPath == "" {
		dbPath = DefaultDBPath()
	}

	_ = os.Remove(dbPath)
	_ = os.Remove(dbPath + "-wal")
	_ = os.Remove(dbPath + "-shm")

	s, err := Open(dbPath)
	if err != nil {
		return fmt.Errorf("reset sqlite db: %w", err)
	}
	return s.Close()
}

func (s *Store) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS executions (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id  TEXT,
		prompt      TEXT NOT NULL,
		command     TEXT NOT NULL,
		description TEXT,
		stdout      TEXT,
		stderr      TEXT,
		exit_code   INTEGER NOT NULL,
		risk_tier   TEXT,
		risk_score  REAL,
		cwd         TEXT NOT NULL,
		shell       TEXT NOT NULL,
		provider    TEXT,
		model       TEXT,
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_executions_cwd ON executions(cwd);
	CREATE INDEX IF NOT EXISTS idx_executions_created ON executions(created_at);
	`
	_, err := s.db.Exec(schema)
	return err
}

// Close closes the database connection.
func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// RecordExecution inserts a new execution into the database.
func (s *Store) RecordExecution(ctx context.Context, e Execution) error {
	query := `
	INSERT INTO executions (
		session_id, prompt, command, description, stdout, stderr, exit_code,
		risk_tier, risk_score, cwd, shell, provider, model, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
	`
	createdAt := e.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	_, err := s.db.ExecContext(
		ctx, query,
		e.SessionID, e.Prompt, e.Command, e.Description, e.Stdout, e.Stderr, e.ExitCode,
		e.RiskTier, e.RiskScore, e.Cwd, e.Shell, e.Provider, e.Model, createdAt.Format(time.RFC3339),
	)
	return err
}

// GetLastExecution returns the most recent execution record, prioritizing current CWD if within maxAge.
func (s *Store) GetLastExecution(ctx context.Context, cwd string, maxAge time.Duration) (*Execution, error) {
	if maxAge <= 0 {
		maxAge = 24 * time.Hour
	}
	since := time.Now().Add(-maxAge).Format(time.RFC3339)

	// First attempt: look for recent execution in the same directory
	if cwd != "" {
		query := `
		SELECT id, session_id, prompt, command, description, stdout, stderr, exit_code,
		       risk_tier, risk_score, cwd, shell, provider, model, created_at
		FROM executions
		WHERE cwd = ? AND created_at >= ?
		ORDER BY created_at DESC, id DESC
		LIMIT 1;
		`
		if exec, err := s.querySingle(ctx, query, cwd, since); err == nil && exec != nil {
			return exec, nil
		}
	}

	// Fallback: look for global recent execution
	query := `
	SELECT id, session_id, prompt, command, description, stdout, stderr, exit_code,
	       risk_tier, risk_score, cwd, shell, provider, model, created_at
	FROM executions
	WHERE created_at >= ?
	ORDER BY created_at DESC, id DESC
	LIMIT 1;
	`
	return s.querySingle(ctx, query, since)
}

// GetRecentExecutions returns the last N executions.
func (s *Store) GetRecentExecutions(ctx context.Context, limit int) ([]Execution, error) {
	if limit <= 0 {
		limit = 10
	}
	query := `
	SELECT id, session_id, prompt, command, description, stdout, stderr, exit_code,
	       risk_tier, risk_score, cwd, shell, provider, model, created_at
	FROM executions
	ORDER BY created_at DESC, id DESC
	LIMIT ?;
	`
	rows, err := s.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var results []Execution
	for rows.Next() {
		var e Execution
		var (
			sessionID, desc, stdout, stderr, riskTier, provider, model, createdAtStr sql.NullString
			riskScore                                                                sql.NullFloat64
		)
		err := rows.Scan(
			&e.ID, &sessionID, &e.Prompt, &e.Command, &desc, &stdout, &stderr, &e.ExitCode,
			&riskTier, &riskScore, &e.Cwd, &e.Shell, &provider, &model, &createdAtStr,
		)
		if err != nil {
			return nil, err
		}
		if sessionID.Valid {
			e.SessionID = sessionID.String
		}
		if desc.Valid {
			e.Description = desc.String
		}
		if stdout.Valid {
			e.Stdout = stdout.String
		}
		if stderr.Valid {
			e.Stderr = stderr.String
		}
		if riskTier.Valid {
			e.RiskTier = riskTier.String
		}
		if riskScore.Valid {
			e.RiskScore = riskScore.Float64
		}
		if provider.Valid {
			e.Provider = provider.String
		}
		if model.Valid {
			e.Model = model.String
		}
		if createdAtStr.Valid {
			t, _ := time.Parse(time.RFC3339, createdAtStr.String)
			e.CreatedAt = t
		}
		results = append(results, e)
	}
	return results, rows.Err()
}

func (s *Store) querySingle(ctx context.Context, query string, args ...any) (*Execution, error) {
	row := s.db.QueryRowContext(ctx, query, args...)
	var e Execution
	var (
		sessionID, desc, stdout, stderr, riskTier, provider, model, createdAtStr sql.NullString
		riskScore                                                                sql.NullFloat64
	)
	err := row.Scan(
		&e.ID, &sessionID, &e.Prompt, &e.Command, &desc, &stdout, &stderr, &e.ExitCode,
		&riskTier, &riskScore, &e.Cwd, &e.Shell, &provider, &model, &createdAtStr,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if sessionID.Valid {
		e.SessionID = sessionID.String
	}
	if desc.Valid {
		e.Description = desc.String
	}
	if stdout.Valid {
		e.Stdout = stdout.String
	}
	if stderr.Valid {
		e.Stderr = stderr.String
	}
	if riskTier.Valid {
		e.RiskTier = riskTier.String
	}
	if riskScore.Valid {
		e.RiskScore = riskScore.Float64
	}
	if provider.Valid {
		e.Provider = provider.String
	}
	if model.Valid {
		e.Model = model.String
	}
	if createdAtStr.Valid {
		t, _ := time.Parse(time.RFC3339, createdAtStr.String)
		e.CreatedAt = t
	}
	return &e, nil
}
