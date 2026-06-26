package storage

import (
	"testing"
	"time"

	"git-statistics/internal/domain"
)

func TestUpsertPullRequest_IdempotentAndFilterable(t *testing.T) {
	store := openTestStore(t)

	created := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	merged := created.Add(48 * time.Hour)
	pr := domain.PullRequest{
		ID: 1, RepoSlug: "repo-one", Title: "Add feature", AuthorID: "acct-1",
		State: "MERGED", CreatedAt: created, UpdatedAt: merged, MergedAt: &merged,
	}
	if err := store.UpsertPullRequest(pr); err != nil {
		t.Fatalf("upsert failed: %v", err)
	}
	pr.State = "MERGED"
	if err := store.UpsertPullRequest(pr); err != nil {
		t.Fatalf("second upsert failed: %v", err)
	}

	prs, err := store.ListPullRequests(Filter{RepoSlug: "repo-one"})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(prs) != 1 {
		t.Fatalf("expected 1 PR, got %d", len(prs))
	}
	if prs[0].MergedAt == nil || !prs[0].MergedAt.Equal(merged) {
		t.Errorf("expected MergedAt %v, got %v", merged, prs[0].MergedAt)
	}
}

func TestUpsertReview_AndListByPullRequest(t *testing.T) {
	store := openTestStore(t)

	r := domain.Review{
		ID: "rev-1", PullRequestID: 1, RepoSlug: "repo-one",
		ReviewerID: "acct-2", Action: "approved",
		CreatedAt: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
	}
	if err := store.UpsertReview(r); err != nil {
		t.Fatalf("upsert failed: %v", err)
	}
	if err := store.UpsertReview(r); err != nil {
		t.Fatalf("second upsert failed: %v", err)
	}

	reviews, err := store.ListReviews("repo-one", 1)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(reviews) != 1 {
		t.Fatalf("expected 1 review, got %d", len(reviews))
	}

	none, err := store.ListReviews("repo-one", 2)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("expected 0 reviews for PR 2, got %d", len(none))
	}
}
