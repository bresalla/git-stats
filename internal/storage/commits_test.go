package storage

import (
	"testing"
	"time"

	"git-statistics/internal/domain"
)

func TestUpsertCommit_IdempotentAndFilterable(t *testing.T) {
	store := openTestStore(t)

	c := domain.Commit{
		Hash: "abc123", RepoSlug: "repo-one", AuthorID: "acct-1",
		Message: "fix bug", AuthoredAt: time.Date(2026, 1, 5, 10, 0, 0, 0, time.UTC),
	}
	if err := store.UpsertCommit(c); err != nil {
		t.Fatalf("upsert failed: %v", err)
	}
	if err := store.UpsertCommit(c); err != nil {
		t.Fatalf("second upsert failed: %v", err)
	}

	other := domain.Commit{
		Hash: "def456", RepoSlug: "repo-two", AuthorID: "acct-2",
		Message: "add feature", AuthoredAt: time.Date(2026, 2, 1, 10, 0, 0, 0, time.UTC),
	}
	if err := store.UpsertCommit(other); err != nil {
		t.Fatalf("upsert failed: %v", err)
	}

	all, err := store.ListCommits(Filter{})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 commits total, got %d", len(all))
	}

	filtered, err := store.ListCommits(Filter{RepoSlug: "repo-one"})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(filtered) != 1 || filtered[0].Hash != "abc123" {
		t.Fatalf("expected only abc123 for repo-one, got %+v", filtered)
	}

	byDate, err := store.ListCommits(Filter{From: time.Date(2026, 1, 20, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(byDate) != 1 || byDate[0].Hash != "def456" {
		t.Fatalf("expected only def456 after 2026-01-20, got %+v", byDate)
	}
}

func TestUpsertFileChange_IdempotentAndFilterable(t *testing.T) {
	store := openTestStore(t)

	fc := domain.FileChange{
		CommitHash: "abc123", RepoSlug: "repo-one", Path: "main.go",
		LinesAdded: 10, LinesRemoved: 2, Status: "modified",
	}
	if err := store.UpsertFileChange(fc); err != nil {
		t.Fatalf("upsert failed: %v", err)
	}
	fc.LinesAdded = 15
	if err := store.UpsertFileChange(fc); err != nil {
		t.Fatalf("second upsert failed: %v", err)
	}

	changes, err := store.ListFileChanges(Filter{RepoSlug: "repo-one"})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(changes) != 1 {
		t.Fatalf("expected 1 file change, got %d", len(changes))
	}
	if changes[0].LinesAdded != 15 {
		t.Errorf("expected updated LinesAdded 15, got %d", changes[0].LinesAdded)
	}
}
