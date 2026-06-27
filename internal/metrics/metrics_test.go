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

func TestSummaryStats_ComputesAvgMedianMinMaxForMergedPRs(t *testing.T) {
	store := openTestStore(t)

	created := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	firstReview := created.Add(6 * time.Hour)
	merged := created.Add(24 * time.Hour)

	must(t, store.UpsertPullRequest(domain.PullRequest{
		ID: 1, RepoSlug: "repo-one", Title: "PR 1", AuthorID: "acct-1",
		State: "MERGED", CreatedAt: created, UpdatedAt: merged, MergedAt: &merged,
	}))
	must(t, store.UpsertReview(domain.Review{
		ID: "r1", PullRequestID: 1, RepoSlug: "repo-one", ReviewerID: "acct-2",
		Action: "approved", CreatedAt: firstReview,
	}))
	must(t, store.UpsertPullRequest(domain.PullRequest{
		ID: 2, RepoSlug: "repo-one", Title: "PR 2 (still open)", AuthorID: "acct-1",
		State: "OPEN", CreatedAt: created, UpdatedAt: created,
	}))

	stats, err := SummaryStats(store, storage.Filter{})
	if err != nil {
		t.Fatalf("SummaryStats failed: %v", err)
	}
	if stats.AvgCycleTime != 24*time.Hour || stats.MedianCycleTime != 24*time.Hour ||
		stats.MinCycleTime != 24*time.Hour || stats.MaxCycleTime != 24*time.Hour {
		t.Errorf("unexpected cycle time stats: %+v", stats)
	}
	if stats.AvgFirstReview != 6*time.Hour {
		t.Errorf("expected AvgFirstReview 6h, got %v", stats.AvgFirstReview)
	}
	if stats.AvgInReview != 18*time.Hour {
		t.Errorf("expected AvgInReview 18h, got %v", stats.AvgInReview)
	}
}

func TestSummaryStats_NoMergedPRsReturnsZeroValues(t *testing.T) {
	store := openTestStore(t)

	stats, err := SummaryStats(store, storage.Filter{})
	if err != nil {
		t.Fatalf("SummaryStats failed: %v", err)
	}
	if stats.AvgCycleTime != 0 || stats.MaxCycleTime != 0 {
		t.Errorf("expected zero-value stats for no merged PRs, got %+v", stats)
	}
}

func TestDistributions_ComputesPercentilesAcrossMergedPRs(t *testing.T) {
	store := openTestStore(t)

	created := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	for i := 1; i <= 10; i++ {
		merged := created.Add(time.Duration(i*2) * time.Hour)
		must(t, store.UpsertPullRequest(domain.PullRequest{
			ID: i, RepoSlug: "repo-one", Title: "PR", AuthorID: "acct-1",
			State: "MERGED", CreatedAt: created, UpdatedAt: merged, MergedAt: &merged,
		}))
	}

	dist, err := Distributions(store, storage.Filter{})
	if err != nil {
		t.Fatalf("Distributions failed: %v", err)
	}
	for _, key := range []string{"p50", "p75", "p90", "p95", "p99"} {
		if _, ok := dist.CycleTime[key]; !ok {
			t.Errorf("expected %s in CycleTime distribution", key)
		}
	}
}

func TestBreakdownByRepository_GroupsAndSortsByAvgCycleTime(t *testing.T) {
	store := openTestStore(t)

	created := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	mergedSlow := created.Add(24 * time.Hour)
	mergedFast := created.Add(12 * time.Hour)

	must(t, store.UpsertPullRequest(domain.PullRequest{
		ID: 1, RepoSlug: "repo-one", Title: "PR 1", AuthorID: "acct-1",
		State: "MERGED", CreatedAt: created, UpdatedAt: mergedSlow, MergedAt: &mergedSlow,
	}))
	must(t, store.UpsertPullRequest(domain.PullRequest{
		ID: 2, RepoSlug: "repo-two", Title: "PR 2", AuthorID: "acct-1",
		State: "MERGED", CreatedAt: created, UpdatedAt: mergedFast, MergedAt: &mergedFast,
	}))

	rows, err := BreakdownByRepository(store, storage.Filter{})
	if err != nil {
		t.Fatalf("BreakdownByRepository failed: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].Key != "repo-one" || rows[0].AvgCycleTime != 24*time.Hour || rows[0].Count != 1 {
		t.Errorf("expected repo-one first with 24h avg cycle time, got %+v", rows[0])
	}
	if rows[1].Key != "repo-two" {
		t.Errorf("expected repo-two second, got %+v", rows[1])
	}
}

func TestBreakdownByAuthor_GroupsByDisplayName(t *testing.T) {
	store := openTestStore(t)

	must(t, store.UpsertAuthor(domain.Author{ID: "acct-1", DisplayName: "Alice"}))
	must(t, store.UpsertAuthor(domain.Author{ID: "acct-2", DisplayName: "Bob"}))

	created := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	merged := created.Add(12 * time.Hour)

	must(t, store.UpsertPullRequest(domain.PullRequest{
		ID: 1, RepoSlug: "repo-one", Title: "PR 1", AuthorID: "acct-1",
		State: "MERGED", CreatedAt: created, UpdatedAt: merged, MergedAt: &merged,
	}))
	must(t, store.UpsertPullRequest(domain.PullRequest{
		ID: 2, RepoSlug: "repo-one", Title: "PR 2", AuthorID: "acct-2",
		State: "MERGED", CreatedAt: created, UpdatedAt: merged, MergedAt: &merged,
	}))

	rows, err := BreakdownByAuthor(store, storage.Filter{})
	if err != nil {
		t.Fatalf("BreakdownByAuthor failed: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 authors, got %d", len(rows))
	}
	keys := map[string]bool{rows[0].Key: true, rows[1].Key: true}
	if !keys["Alice"] || !keys["Bob"] {
		t.Errorf("expected breakdown keyed by display name, got %+v", rows)
	}
}

func TestBreakdownByRepository_LimitsToTop10ByCount(t *testing.T) {
	store := openTestStore(t)

	created := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	merged := created.Add(time.Hour)

	prID := 1
	for repo := 0; repo < 12; repo++ {
		count := 1
		if repo < 11 {
			count = 2 // give the first 11 repos more PRs than the 12th, so only one is dropped
		}
		for i := 0; i < count; i++ {
			must(t, store.UpsertPullRequest(domain.PullRequest{
				ID: prID, RepoSlug: "repo-" + string(rune('a'+repo)), Title: "PR", AuthorID: "acct-1",
				State: "MERGED", CreatedAt: created, UpdatedAt: merged, MergedAt: &merged,
			}))
			prID++
		}
	}

	rows, err := BreakdownByRepository(store, storage.Filter{})
	if err != nil {
		t.Fatalf("BreakdownByRepository failed: %v", err)
	}
	if len(rows) != 10 {
		t.Fatalf("expected breakdown limited to 10 rows, got %d", len(rows))
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
