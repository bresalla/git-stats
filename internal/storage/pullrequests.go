package storage

import (
	"database/sql"
	"strings"

	"git-statistics/internal/domain"
)

func (s *Store) UpsertPullRequest(pr domain.PullRequest) error {
	var mergedAt sql.NullTime
	if pr.MergedAt != nil {
		mergedAt = sql.NullTime{Time: *pr.MergedAt, Valid: true}
	}
	_, err := s.db.Exec(`
		INSERT INTO pull_requests (id, repo_slug, title, author_id, state, created_at, updated_at, merged_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id, repo_slug) DO UPDATE SET
			title = excluded.title, author_id = excluded.author_id, state = excluded.state,
			updated_at = excluded.updated_at, merged_at = excluded.merged_at
	`, pr.ID, pr.RepoSlug, pr.Title, pr.AuthorID, pr.State, pr.CreatedAt, pr.UpdatedAt, mergedAt)
	return err
}

func (s *Store) UpsertReview(r domain.Review) error {
	_, err := s.db.Exec(`
		INSERT INTO reviews (id, pull_request_id, repo_slug, reviewer_id, action, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			reviewer_id = excluded.reviewer_id, action = excluded.action, created_at = excluded.created_at
	`, r.ID, r.PullRequestID, r.RepoSlug, r.ReviewerID, r.Action, r.CreatedAt)
	return err
}

func (s *Store) ListPullRequests(f Filter) ([]domain.PullRequest, error) {
	var clauses []string
	var args []any
	if f.RepoSlug != "" {
		clauses = append(clauses, "repo_slug = ?")
		args = append(args, f.RepoSlug)
	}
	if f.AuthorID != "" {
		clauses = append(clauses, "author_id = ?")
		args = append(args, f.AuthorID)
	}
	if !f.From.IsZero() {
		clauses = append(clauses, "created_at >= ?")
		args = append(args, f.From)
	}
	if !f.To.IsZero() {
		clauses = append(clauses, "created_at <= ?")
		args = append(args, f.To)
	}
	where := ""
	if len(clauses) > 0 {
		where = " WHERE " + strings.Join(clauses, " AND ")
	}

	rows, err := s.db.Query(`
		SELECT id, repo_slug, title, author_id, state, created_at, updated_at, merged_at
		FROM pull_requests`+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var prs []domain.PullRequest
	for rows.Next() {
		var pr domain.PullRequest
		var mergedAt sql.NullTime
		if err := rows.Scan(&pr.ID, &pr.RepoSlug, &pr.Title, &pr.AuthorID, &pr.State, &pr.CreatedAt, &pr.UpdatedAt, &mergedAt); err != nil {
			return nil, err
		}
		if mergedAt.Valid {
			pr.MergedAt = &mergedAt.Time
		}
		prs = append(prs, pr)
	}
	return prs, rows.Err()
}

func (s *Store) ListReviews(repoSlug string, pullRequestID int) ([]domain.Review, error) {
	rows, err := s.db.Query(`
		SELECT id, pull_request_id, repo_slug, reviewer_id, action, created_at
		FROM reviews WHERE repo_slug = ? AND pull_request_id = ?
		ORDER BY created_at ASC
	`, repoSlug, pullRequestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reviews []domain.Review
	for rows.Next() {
		var r domain.Review
		if err := rows.Scan(&r.ID, &r.PullRequestID, &r.RepoSlug, &r.ReviewerID, &r.Action, &r.CreatedAt); err != nil {
			return nil, err
		}
		reviews = append(reviews, r)
	}
	return reviews, rows.Err()
}
