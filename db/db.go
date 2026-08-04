// Package db provides SQLite persistence for frao-advisor.
// Uses modernc.org/sqlite — pure Go, no CGO required.
package db

import (
	"context"
	"database/sql"
	_ "modernc.org/sqlite"
)

// DB wraps the SQLite connection and provides all CRUD operations.
type DB struct {
	*sql.DB
}

// Open opens (or creates) the SQLite database at the given path and
// runs schema migrations. Uses WAL mode for concurrent read performance.
func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_txlock=immediate")
	if err != nil {
		return nil, err
	}
	conn.SetMaxOpenConns(1) // SQLite: single writer

	db := &DB{conn}
	if err := db.InitSchema(context.Background()); err != nil {
		conn.Close()
		return nil, err
	}
	return db, nil
}

// InitSchema creates all tables and indexes if they don't exist.
func (d *DB) InitSchema(ctx context.Context) error {
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() // no-op if committed

	for _, stmt := range schemaStatements {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// schemaStatements is the full DDL for all tables, indexes, and triggers.
var schemaStatements = []string{
	// Schema version tracking
	`CREATE TABLE IF NOT EXISTS schema_version (
		version    INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`,
	// Sessions
	`CREATE TABLE IF NOT EXISTS sessions (
		id            TEXT PRIMARY KEY,
		session_id   TEXT NOT NULL,
		client_name  TEXT NOT NULL DEFAULT '',
		client_version TEXT NOT NULL DEFAULT '',
		project      TEXT DEFAULT '',
		started_at   TEXT NOT NULL DEFAULT (datetime('now')),
		last_active  TEXT NOT NULL DEFAULT (datetime('now')),
		call_count   INTEGER NOT NULL DEFAULT 0
	)`,
	`CREATE INDEX IF NOT EXISTS idx_sessions_session_id ON sessions(session_id)`,
	// Consultations
	`CREATE TABLE IF NOT EXISTS consultations (
		id                TEXT PRIMARY KEY,
		session_id        TEXT NOT NULL REFERENCES sessions(id),
		question          TEXT NOT NULL,
		response          TEXT NOT NULL,
		model             TEXT NOT NULL DEFAULT 'deepseek-v4-pro',
		reasoning_effort  TEXT DEFAULT 'high',
		prompt_tokens     INTEGER DEFAULT 0,
		completion_tokens INTEGER DEFAULT 0,
		input_cost        REAL DEFAULT 0,
		output_cost       REAL DEFAULT 0,
		input_price_used  REAL DEFAULT 0,
		output_price_used REAL DEFAULT 0,
		cache_hit         INTEGER DEFAULT 0,
		duration_ms       INTEGER DEFAULT 0,
		created_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
	)`,
	`CREATE INDEX IF NOT EXISTS idx_consultations_session ON consultations(session_id)`,
	`CREATE INDEX IF NOT EXISTS idx_consultations_created ON consultations(created_at)`,
	`CREATE INDEX IF NOT EXISTS idx_consultations_model ON consultations(model)`,
	// Expert reviews
	`CREATE TABLE IF NOT EXISTS expert_reviews (
		id                TEXT PRIMARY KEY,
		session_id        TEXT NOT NULL REFERENCES sessions(id),
		expert_key        TEXT NOT NULL,
		context           TEXT NOT NULL,
		analysis          TEXT NOT NULL,
		model             TEXT NOT NULL DEFAULT 'deepseek-v4-pro',
		reasoning_effort  TEXT DEFAULT 'high',
		prompt_tokens     INTEGER DEFAULT 0,
		completion_tokens INTEGER DEFAULT 0,
		input_cost        REAL DEFAULT 0,
		output_cost       REAL DEFAULT 0,
		input_price_used  REAL DEFAULT 0,
		output_price_used REAL DEFAULT 0,
		cache_hit         INTEGER DEFAULT 0,
		duration_ms       INTEGER DEFAULT 0,
		created_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
	)`,
	`CREATE INDEX IF NOT EXISTS idx_expert_reviews_session ON expert_reviews(session_id)`,
	`CREATE INDEX IF NOT EXISTS idx_expert_reviews_expert ON expert_reviews(expert_key)`,
	`CREATE INDEX IF NOT EXISTS idx_expert_reviews_created ON expert_reviews(created_at)`,
	// Deliberations
	`CREATE TABLE IF NOT EXISTS deliberations (
		id                TEXT PRIMARY KEY,
		session_id        TEXT NOT NULL REFERENCES sessions(id),
		context           TEXT NOT NULL,
		synthesis         TEXT NOT NULL,
		expert_count      INTEGER NOT NULL,
		expert_keys       TEXT NOT NULL,
		model             TEXT NOT NULL DEFAULT 'deepseek-v4-pro',
		reasoning_effort  TEXT DEFAULT 'high',
		total_prompt_tokens    INTEGER DEFAULT 0,
		total_completion_tokens INTEGER DEFAULT 0,
		total_input_cost       REAL DEFAULT 0,
		total_output_cost      REAL DEFAULT 0,
		duration_ms       INTEGER DEFAULT 0,
		created_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
	)`,
	`CREATE INDEX IF NOT EXISTS idx_deliberations_session ON deliberations(session_id)`,
	`CREATE INDEX IF NOT EXISTS idx_deliberations_created ON deliberations(created_at)`,
	// Deliberation contributions
	`CREATE TABLE IF NOT EXISTS deliberation_contributions (
		id              TEXT PRIMARY KEY,
		deliberation_id TEXT NOT NULL REFERENCES deliberations(id),
		expert_key      TEXT NOT NULL,
		analysis        TEXT NOT NULL,
		prompt_tokens   INTEGER DEFAULT 0,
		completion_tokens INTEGER DEFAULT 0,
		input_cost      REAL DEFAULT 0,
		output_cost     REAL DEFAULT 0,
		duration_ms     INTEGER DEFAULT 0,
		sort_order      INTEGER DEFAULT 0,
		created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
	)`,
	`CREATE INDEX IF NOT EXISTS idx_delib_contrib_delib ON deliberation_contributions(deliberation_id)`,
	// Pricing snapshots
	`CREATE TABLE IF NOT EXISTS pricing_snapshots (
		id                INTEGER PRIMARY KEY AUTOINCREMENT,
		model             TEXT NOT NULL,
		input_price_per_m REAL NOT NULL,
		output_price_per_m REAL NOT NULL,
		cache_input_price_per_m REAL DEFAULT 0,
		effective_date    TEXT NOT NULL,
		source            TEXT DEFAULT 'api',
		notes             TEXT DEFAULT ''
	)`,
	// Daily aggregated metrics
	`CREATE TABLE IF NOT EXISTS daily_metrics (
		date                TEXT PRIMARY KEY,
		consultations_count INTEGER DEFAULT 0,
		expert_reviews_count INTEGER DEFAULT 0,
		deliberations_count INTEGER DEFAULT 0,
		total_prompt_tokens INTEGER DEFAULT 0,
		total_completion_tokens INTEGER DEFAULT 0,
		total_cost          REAL DEFAULT 0,
		consultations_cost  REAL DEFAULT 0,
		expert_reviews_cost REAL DEFAULT 0,
		deliberations_cost  REAL DEFAULT 0
	)`,
}

// Close closes the database connection.
func (d *DB) Close() error {
	return d.DB.Close()
}
