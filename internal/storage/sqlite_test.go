package storage

import (
	"path/filepath"
	"testing"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestOpen_CreatesSchema(t *testing.T) {
	store := openTestStore(t)

	tables := []string{"repositories", "authors", "commits", "file_changes", "pull_requests", "reviews"}
	for _, table := range tables {
		var name string
		row := store.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table)
		if err := row.Scan(&name); err != nil {
			t.Errorf("expected table %q to exist: %v", table, err)
		}
	}
}
