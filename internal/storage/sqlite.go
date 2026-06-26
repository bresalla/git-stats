package storage

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

type Filter struct {
	RepoSlug string
	AuthorID string
	From     time.Time
	To       time.Time
}

const schema = `
CREATE TABLE IF NOT EXISTS repositories (
	slug TEXT PRIMARY KEY,
	workspace TEXT NOT NULL,
	synced_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS authors (
	id TEXT PRIMARY KEY,
	display_name TEXT,
	email TEXT,
	allowlisted INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS commits (
	hash TEXT NOT NULL,
	repo_slug TEXT NOT NULL,
	author_id TEXT,
	message TEXT,
	authored_at TIMESTAMP NOT NULL,
	PRIMARY KEY (hash, repo_slug)
);

CREATE TABLE IF NOT EXISTS file_changes (
	commit_hash TEXT NOT NULL,
	repo_slug TEXT NOT NULL,
	path TEXT NOT NULL,
	lines_added INTEGER NOT NULL DEFAULT 0,
	lines_removed INTEGER NOT NULL DEFAULT 0,
	status TEXT,
	PRIMARY KEY (commit_hash, repo_slug, path)
);

CREATE TABLE IF NOT EXISTS pull_requests (
	id INTEGER NOT NULL,
	repo_slug TEXT NOT NULL,
	title TEXT,
	author_id TEXT,
	state TEXT,
	created_at TIMESTAMP NOT NULL,
	updated_at TIMESTAMP,
	merged_at TIMESTAMP,
	PRIMARY KEY (id, repo_slug)
);

CREATE TABLE IF NOT EXISTS reviews (
	id TEXT PRIMARY KEY,
	pull_request_id INTEGER NOT NULL,
	repo_slug TEXT NOT NULL,
	reviewer_id TEXT,
	action TEXT,
	created_at TIMESTAMP NOT NULL
);
`

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite db: %w", err)
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("running schema migration: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}
