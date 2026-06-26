package normalize

import (
	"testing"
	"time"

	"git-statistics/internal/bitbucket"
	"git-statistics/internal/domain"
)

func TestCommit_ParsesFieldsAndAuthor(t *testing.T) {
	raw := bitbucket.RawCommit{
		Hash:    "abc123",
		Message: "fix bug",
		Date:    "2026-01-05T10:30:00Z",
	}
	raw.Author.Raw = "Alice Smith <alice@example.com>"
	raw.Author.User.AccountID = "acct-1"
	raw.Author.User.DisplayName = "Alice Smith"

	commit, author, err := Commit("repo-one", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if commit.Hash != "abc123" || commit.RepoSlug != "repo-one" || commit.AuthorID != "acct-1" {
		t.Errorf("unexpected commit: %+v", commit)
	}
	want := time.Date(2026, 1, 5, 10, 30, 0, 0, time.UTC)
	if !commit.AuthoredAt.Equal(want) {
		t.Errorf("expected AuthoredAt %v, got %v", want, commit.AuthoredAt)
	}
	if author.Email != "alice@example.com" || author.DisplayName != "Alice Smith" {
		t.Errorf("unexpected author: %+v", author)
	}
}

func TestCommit_InvalidDateReturnsError(t *testing.T) {
	raw := bitbucket.RawCommit{Hash: "abc", Date: "not-a-date"}
	if _, _, err := Commit("repo-one", raw); err == nil {
		t.Fatal("expected error for invalid date, got nil")
	}
}

func TestFileChanges_MapsAddedModifiedRemoved(t *testing.T) {
	raw := []bitbucket.RawDiffstatEntry{
		{Status: "modified", LinesAdded: 5, LinesRemoved: 1, New: &struct{ Path string `json:"path"` }{Path: "main.go"}},
		{Status: "removed", LinesAdded: 0, LinesRemoved: 20, Old: &struct{ Path string `json:"path"` }{Path: "old.go"}},
	}

	changes := FileChanges("repo-one", "abc123", raw)

	if len(changes) != 2 {
		t.Fatalf("expected 2 file changes, got %d", len(changes))
	}
	if changes[0].Path != "main.go" || changes[0].LinesAdded != 5 {
		t.Errorf("unexpected first file change: %+v", changes[0])
	}
	if changes[1].Path != "old.go" || changes[1].LinesRemoved != 20 {
		t.Errorf("unexpected second file change: %+v", changes[1])
	}
}

func TestIsAllowlisted_CaseInsensitiveEmailMatch(t *testing.T) {
	author := domain.Author{ID: "acct-x", Email: "Alice@Example.com"}
	allowlist := []string{"alice@example.com", "bob@example.com"}

	if !IsAllowlisted(author, allowlist) {
		t.Error("expected case-insensitive match to succeed")
	}

	other := domain.Author{ID: "acct-y", Email: "carol@example.com"}
	if IsAllowlisted(other, allowlist) {
		t.Error("expected carol@example.com to not be allowlisted")
	}
}

func TestPullRequest_MergedSetsMergedAt(t *testing.T) {
	raw := bitbucket.RawPullRequest{
		ID: 1, Title: "Add feature", State: "MERGED",
		CreatedOn: "2026-01-01T09:00:00Z", UpdatedOn: "2026-01-03T09:00:00Z",
	}
	raw.Author.Raw = "Alice <alice@example.com>"
	raw.Author.User.AccountID = "acct-1"

	pr, author, err := PullRequest("repo-one", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pr.MergedAt == nil {
		t.Fatal("expected MergedAt to be set for a MERGED PR")
	}
	if !pr.MergedAt.Equal(pr.UpdatedAt) {
		t.Errorf("expected MergedAt to equal UpdatedAt, got %v vs %v", pr.MergedAt, pr.UpdatedAt)
	}
	if author.ID != "acct-1" {
		t.Errorf("unexpected author: %+v", author)
	}
}

func TestPullRequest_OpenLeavesMergedAtNil(t *testing.T) {
	raw := bitbucket.RawPullRequest{
		ID: 2, Title: "WIP", State: "OPEN",
		CreatedOn: "2026-01-01T09:00:00Z", UpdatedOn: "2026-01-01T09:00:00Z",
	}
	pr, _, err := PullRequest("repo-one", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pr.MergedAt != nil {
		t.Errorf("expected MergedAt nil for an OPEN PR, got %v", pr.MergedAt)
	}
}

func TestReview_ApprovalMapsToActionApproved(t *testing.T) {
	raw := bitbucket.RawActivity{}
	raw.Approval = &struct {
		Date string              `json:"date"`
		User bitbucket.RawAuthor `json:"user"`
	}{Date: "2026-01-02T09:00:00Z"}
	raw.Approval.User.Raw = "Bob <bob@example.com>"
	raw.Approval.User.User.AccountID = "acct-2"

	review, author, ok := Review("repo-one", 1, raw)
	if !ok {
		t.Fatal("expected ok=true for an approval activity entry")
	}
	if review.Action != "approved" || review.ReviewerID != "acct-2" {
		t.Errorf("unexpected review: %+v", review)
	}
	if author.ID != "acct-2" {
		t.Errorf("unexpected author: %+v", author)
	}
}

func TestReview_UnrecognizedActivitySkipped(t *testing.T) {
	raw := bitbucket.RawActivity{}
	_, _, ok := Review("repo-one", 1, raw)
	if ok {
		t.Error("expected ok=false for an activity entry with no approval or comment")
	}
}
