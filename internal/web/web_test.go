package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"git-statistics/internal/domain"
	"git-statistics/internal/scheduler"
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

func newTestHandler(t *testing.T, store *storage.Store) http.Handler {
	t.Helper()
	sched := scheduler.New(time.Hour, func(ctx context.Context) {})
	h := NewHandler(store, sched, []string{"repo-one"})
	return h.Routes()
}

func TestActivityDashboard_ListsAllowlistedAuthorCommitCounts(t *testing.T) {
	store := openTestStore(t)
	must(t, store.UpsertAuthor(domain.Author{ID: "acct-1", DisplayName: "Alice", Allowlisted: true}))
	must(t, store.UpsertCommit(domain.Commit{Hash: "c1", RepoSlug: "repo-one", AuthorID: "acct-1", AuthoredAt: time.Now()}))

	handler := newTestHandler(t, store)
	req := httptest.NewRequest(http.MethodGet, "/activity", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Alice") {
		t.Errorf("expected response to mention Alice, got: %s", body)
	}
	if !strings.Contains(body, "Activity") {
		t.Errorf("expected response to mention the Activity dashboard title, got: %s", body)
	}
}

func TestActivityDashboard_FiltersByRepo(t *testing.T) {
	store := openTestStore(t)
	must(t, store.UpsertAuthor(domain.Author{ID: "acct-1", DisplayName: "Alice", Allowlisted: true}))
	must(t, store.UpsertCommit(domain.Commit{Hash: "c1", RepoSlug: "repo-one", AuthorID: "acct-1", AuthoredAt: time.Now()}))
	must(t, store.UpsertCommit(domain.Commit{Hash: "c2", RepoSlug: "repo-two", AuthorID: "acct-1", AuthoredAt: time.Now()}))

	handler := newTestHandler(t, store)
	req := httptest.NewRequest(http.MethodGet, "/activity?repo=repo-one", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), ">1<") {
		t.Errorf("expected commit count of 1 after repo filter, got: %s", rec.Body.String())
	}
}

func TestDeliveryFlowDashboard_ShowsMergedPRsOnly(t *testing.T) {
	store := openTestStore(t)
	created := time.Now().Add(-72 * time.Hour)
	merged := created.Add(48 * time.Hour)
	must(t, store.UpsertPullRequest(domain.PullRequest{
		ID: 1, RepoSlug: "repo-one", Title: "Add feature", AuthorID: "acct-1",
		State: "MERGED", CreatedAt: created, UpdatedAt: merged, MergedAt: &merged,
	}))
	must(t, store.UpsertPullRequest(domain.PullRequest{
		ID: 2, RepoSlug: "repo-one", Title: "WIP", AuthorID: "acct-1",
		State: "OPEN", CreatedAt: created, UpdatedAt: created,
	}))

	handler := newTestHandler(t, store)
	req := httptest.NewRequest(http.MethodGet, "/delivery-flow", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Add feature") {
		t.Errorf("expected merged PR title in response, got: %s", body)
	}
	if strings.Contains(body, "WIP") {
		t.Errorf("expected open PR to be excluded from delivery flow, got: %s", body)
	}
}

func TestDeliveryFlowDashboard_ShowsSummaryStatsDistributionsAndBreakdowns(t *testing.T) {
	store := openTestStore(t)
	must(t, store.UpsertAuthor(domain.Author{ID: "acct-1", DisplayName: "Alice"}))
	created := time.Now().Add(-72 * time.Hour)
	firstReview := created.Add(6 * time.Hour)
	merged := created.Add(24 * time.Hour)
	must(t, store.UpsertPullRequest(domain.PullRequest{
		ID: 1, RepoSlug: "repo-one", Title: "Add feature", AuthorID: "acct-1",
		State: "MERGED", CreatedAt: created, UpdatedAt: merged, MergedAt: &merged,
	}))
	must(t, store.UpsertReview(domain.Review{
		ID: "r1", PullRequestID: 1, RepoSlug: "repo-one", ReviewerID: "acct-2",
		Action: "approved", CreatedAt: firstReview,
	}))

	handler := newTestHandler(t, store)
	req := httptest.NewRequest(http.MethodGet, "/delivery-flow", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Avg Cycle Time") {
		t.Errorf("expected summary cards in response, got: %s", body)
	}
	if !strings.Contains(body, "Distribution (Percentiles)") {
		t.Errorf("expected distribution tables in response, got: %s", body)
	}
	if !strings.Contains(body, "By Repository") || !strings.Contains(body, "repo-one") {
		t.Errorf("expected repository breakdown in response, got: %s", body)
	}
	if !strings.Contains(body, "By Author") || !strings.Contains(body, "Alice") {
		t.Errorf("expected author breakdown keyed by display name in response, got: %s", body)
	}
}

func TestChurnDashboard_ShowsTopFiles(t *testing.T) {
	store := openTestStore(t)
	must(t, store.UpsertCommit(domain.Commit{Hash: "c1", RepoSlug: "repo-one", AuthoredAt: time.Now()}))
	must(t, store.UpsertFileChange(domain.FileChange{CommitHash: "c1", RepoSlug: "repo-one", Path: "hot.go", LinesAdded: 50, LinesRemoved: 10}))

	handler := newTestHandler(t, store)
	req := httptest.NewRequest(http.MethodGet, "/churn", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "hot.go") {
		t.Errorf("expected hot.go in churn dashboard, got: %s", rec.Body.String())
	}
}

func TestSyncEndpoint_TriggersSchedulerAndRedirects(t *testing.T) {
	store := openTestStore(t)
	triggered := make(chan struct{}, 1)
	sched := scheduler.New(time.Hour, func(ctx context.Context) { triggered <- struct{}{} })
	h := NewHandler(store, sched, []string{"repo-one"})
	handler := h.Routes()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go sched.Start(ctx)

	req := httptest.NewRequest(http.MethodPost, "/sync", nil)
	req.Header.Set("Referer", "/activity")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect, got %d", rec.Code)
	}
	if rec.Header().Get("Location") != "/activity" {
		t.Errorf("expected redirect to /activity, got %q", rec.Header().Get("Location"))
	}

	select {
	case <-triggered:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected sync to be triggered")
	}
}
