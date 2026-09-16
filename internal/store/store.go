package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type Store struct{ db *sql.DB }

func Open(ctx context.Context, path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	store := &Store{db: db}
	if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = ON; PRAGMA journal_mode = WAL; PRAGMA busy_timeout = 5000;`); err != nil {
		db.Close()
		return nil, err
	}
	if err := store.migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) DB() *sql.DB { return s.db }

func (s *Store) migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY);
INSERT OR IGNORE INTO schema_migrations(version) VALUES (1);
CREATE TABLE IF NOT EXISTS tasks (
 id TEXT PRIMARY KEY, description TEXT NOT NULL, project TEXT, due_date TEXT,
 important INTEGER NOT NULL DEFAULT 0, status TEXT NOT NULL, created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL, completed_at TEXT, deleted_at TEXT
);
CREATE TABLE IF NOT EXISTS projects (id TEXT PRIMARY KEY, name TEXT NOT NULL, normalized_name TEXT NOT NULL UNIQUE, archived_at TEXT);
CREATE TABLE IF NOT EXISTS journal_entries (
 id TEXT PRIMARY KEY, description TEXT NOT NULL, project TEXT, entry_date TEXT,
 created_at TEXT NOT NULL, updated_at TEXT NOT NULL, deleted_at TEXT
);
CREATE TABLE IF NOT EXISTS tags (id TEXT PRIMARY KEY, name TEXT NOT NULL, normalized_name TEXT NOT NULL UNIQUE, archived_at TEXT);
CREATE TABLE IF NOT EXISTS task_tags (task_id TEXT NOT NULL REFERENCES tasks(id), tag_id TEXT NOT NULL REFERENCES tags(id), PRIMARY KEY(task_id, tag_id));
CREATE TABLE IF NOT EXISTS journal_entry_tags (journal_entry_id TEXT NOT NULL REFERENCES journal_entries(id), tag_id TEXT NOT NULL REFERENCES tags(id), PRIMARY KEY(journal_entry_id, tag_id));
CREATE TABLE IF NOT EXISTS journal_entry_tasks (journal_entry_id TEXT NOT NULL REFERENCES journal_entries(id), task_id TEXT NOT NULL REFERENCES tasks(id), PRIMARY KEY(journal_entry_id, task_id));
CREATE TABLE IF NOT EXISTS daily_reviews (review_date TEXT PRIMARY KEY, completed_at TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS tasks_status_due ON tasks(status, due_date);
CREATE INDEX IF NOT EXISTS journal_entries_date ON journal_entries(entry_date, created_at);
CREATE INDEX IF NOT EXISTS tasks_deleted ON tasks(deleted_at);
CREATE INDEX IF NOT EXISTS journal_entries_deleted ON journal_entries(deleted_at);
`)
	return err
}
