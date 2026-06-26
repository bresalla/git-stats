package storage

import (
	"testing"
	"time"

	"git-statistics/internal/domain"
)

func TestUpsertRepository_AndGetSyncedAt(t *testing.T) {
	store := openTestStore(t)

	zero, err := store.GetRepositorySyncedAt("repo-one")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !zero.IsZero() {
		t.Fatalf("expected zero time for unsynced repo, got %v", zero)
	}

	syncedAt := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	err = store.UpsertRepository(domain.Repository{Slug: "repo-one", Workspace: "rdwrcloud", SyncedAt: syncedAt})
	if err != nil {
		t.Fatalf("upsert failed: %v", err)
	}

	got, err := store.GetRepositorySyncedAt("repo-one")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Equal(syncedAt) {
		t.Errorf("expected synced_at %v, got %v", syncedAt, got)
	}

	later := syncedAt.Add(time.Hour)
	if err := store.UpsertRepository(domain.Repository{Slug: "repo-one", Workspace: "rdwrcloud", SyncedAt: later}); err != nil {
		t.Fatalf("second upsert failed: %v", err)
	}
	got, _ = store.GetRepositorySyncedAt("repo-one")
	if !got.Equal(later) {
		t.Errorf("expected synced_at %v after second upsert, got %v", later, got)
	}

	var count int
	row := store.db.QueryRow("SELECT COUNT(*) FROM repositories WHERE slug = ?", "repo-one")
	if err := row.Scan(&count); err != nil {
		t.Fatalf("count query failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected exactly 1 row for repo-one, got %d", count)
	}
}

func TestUpsertAuthor_AndList(t *testing.T) {
	store := openTestStore(t)

	a := domain.Author{ID: "acct-1", DisplayName: "Alice", Email: "alice@example.com", Allowlisted: true}
	if err := store.UpsertAuthor(a); err != nil {
		t.Fatalf("upsert failed: %v", err)
	}
	a.Allowlisted = false
	if err := store.UpsertAuthor(a); err != nil {
		t.Fatalf("second upsert failed: %v", err)
	}

	authors, err := store.ListAuthors()
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(authors) != 1 {
		t.Fatalf("expected 1 author, got %d", len(authors))
	}
	if authors[0].Allowlisted {
		t.Errorf("expected allowlisted=false after update, got true")
	}
}
