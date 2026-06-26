package metrics

import (
	"path/filepath"
	"testing"
	"time"

	"git-statistics/internal/domain"
	"git-statistics/internal/storage"
)

func openTestStore(t *testing.T) *storage.Store {
	t.Helper()
	store, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeliveryFlow_ComputesCycleTimeAndReviewLag(t *testing.T) {
	store := openTestStore(t)

	created := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	firstReview := created.Add(4 * time.Hour)
	merged := created.Add(48 * time.Hour)

	must(t, store.UpsertPullRequest(domain.PullRequest{
		ID: 1, RepoSlug: "repo-one", Title: "Add feature", AuthorID: "acct-1",
		State: "MERGED", CreatedAt: created, UpdatedAt: merged, MergedAt: &merged,
	}))
	must(t, store.UpsertReview(domain.Review{
		ID: "r1", PullRequestID: 1, RepoSlug: "repo-one", ReviewerID: "acct-2",
		Action: "approved", CreatedAt: firstReview,
	}))
	must(t, store.UpsertPullRequest(domain.PullRequest{
		ID: 2, RepoSlug: "repo-one", Title: "WIP", AuthorID: "acct-1",
		State: "OPEN", CreatedAt: created, UpdatedAt: created,
	}))

	flows, err := DeliveryFlow(store, storage.Filter{RepoSlug: "repo-one"})
	if err != nil {
		t.Fatalf("DeliveryFlow failed: %v", err)
	}
	if len(flows) != 1 {
		t.Fatalf("expected 1 merged PR in results, got %d", len(flows))
	}
	if flows[0].CycleTime != 48*time.Hour {
		t.Errorf("expected cycle time 48h, got %v", flows[0].CycleTime)
	}
	if flows[0].TimeToFirstReview != 4*time.Hour {
		t.Errorf("expected time to first review 4h, got %v", flows[0].TimeToFirstReview)
	}
	if flows[0].TimeInReview != 44*time.Hour {
		t.Errorf("expected time in review 44h, got %v", flows[0].TimeInReview)
	}
}

func TestChurnHotspots_AggregatesByPathSortedDescending(t *testing.T) {
	store := openTestStore(t)

	must(t, store.UpsertCommit(domain.Commit{Hash: "c1", RepoSlug: "repo-one", AuthoredAt: time.Now()}))
	must(t, store.UpsertCommit(domain.Commit{Hash: "c2", RepoSlug: "repo-one", AuthoredAt: time.Now()}))
	must(t, store.UpsertFileChange(domain.FileChange{CommitHash: "c1", RepoSlug: "repo-one", Path: "hot.go", LinesAdded: 10, LinesRemoved: 5}))
	must(t, store.UpsertFileChange(domain.FileChange{CommitHash: "c2", RepoSlug: "repo-one", Path: "hot.go", LinesAdded: 20, LinesRemoved: 0}))
	must(t, store.UpsertFileChange(domain.FileChange{CommitHash: "c2", RepoSlug: "repo-one", Path: "cold.go", LinesAdded: 1, LinesRemoved: 1}))

	churn, err := ChurnHotspots(store, storage.Filter{RepoSlug: "repo-one"})
	if err != nil {
		t.Fatalf("ChurnHotspots failed: %v", err)
	}
	if len(churn) != 2 {
		t.Fatalf("expected 2 distinct files, got %d", len(churn))
	}
	if churn[0].Path != "hot.go" || churn[0].LinesChanged != 35 || churn[0].CommitCount != 2 {
		t.Errorf("expected hot.go first with 35 lines changed across 2 commits, got %+v", churn[0])
	}
	if churn[1].Path != "cold.go" {
		t.Errorf("expected cold.go second, got %+v", churn[1])
	}
}

func TestCommitsPerAuthor_OnlyAllowlistedSortedDescending(t *testing.T) {
	store := openTestStore(t)

	must(t, store.UpsertAuthor(domain.Author{ID: "acct-1", DisplayName: "Alice", Allowlisted: true}))
	must(t, store.UpsertAuthor(domain.Author{ID: "acct-2", DisplayName: "Carol", Allowlisted: false}))
	must(t, store.UpsertCommit(domain.Commit{Hash: "c1", RepoSlug: "repo-one", AuthorID: "acct-1", AuthoredAt: time.Now()}))
	must(t, store.UpsertCommit(domain.Commit{Hash: "c2", RepoSlug: "repo-one", AuthorID: "acct-1", AuthoredAt: time.Now()}))
	must(t, store.UpsertCommit(domain.Commit{Hash: "c3", RepoSlug: "repo-one", AuthorID: "acct-2", AuthoredAt: time.Now()}))

	activity, err := CommitsPerAuthor(store, storage.Filter{RepoSlug: "repo-one"})
	if err != nil {
		t.Fatalf("CommitsPerAuthor failed: %v", err)
	}
	if len(activity) != 1 {
		t.Fatalf("expected only the allowlisted author (acct-1), got %d entries: %+v", len(activity), activity)
	}
	if activity[0].AuthorID != "acct-1" || activity[0].CommitCount != 2 {
		t.Errorf("unexpected activity: %+v", activity[0])
	}
}
