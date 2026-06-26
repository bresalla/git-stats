package storage

import (
	"strings"

	"git-statistics/internal/domain"
)

func (s *Store) UpsertCommit(c domain.Commit) error {
	_, err := s.db.Exec(`
		INSERT INTO commits (hash, repo_slug, author_id, message, authored_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(hash, repo_slug) DO UPDATE SET
			author_id = excluded.author_id, message = excluded.message, authored_at = excluded.authored_at
	`, c.Hash, c.RepoSlug, c.AuthorID, c.Message, c.AuthoredAt)
	return err
}

func (s *Store) UpsertFileChange(fc domain.FileChange) error {
	_, err := s.db.Exec(`
		INSERT INTO file_changes (commit_hash, repo_slug, path, lines_added, lines_removed, status)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(commit_hash, repo_slug, path) DO UPDATE SET
			lines_added = excluded.lines_added, lines_removed = excluded.lines_removed, status = excluded.status
	`, fc.CommitHash, fc.RepoSlug, fc.Path, fc.LinesAdded, fc.LinesRemoved, fc.Status)
	return err
}

func buildCommitFilter(f Filter) (string, []any) {
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
		clauses = append(clauses, "authored_at >= ?")
		args = append(args, f.From)
	}
	if !f.To.IsZero() {
		clauses = append(clauses, "authored_at <= ?")
		args = append(args, f.To)
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func (s *Store) ListCommits(f Filter) ([]domain.Commit, error) {
	where, args := buildCommitFilter(f)
	rows, err := s.db.Query("SELECT hash, repo_slug, author_id, message, authored_at FROM commits"+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var commits []domain.Commit
	for rows.Next() {
		var c domain.Commit
		if err := rows.Scan(&c.Hash, &c.RepoSlug, &c.AuthorID, &c.Message, &c.AuthoredAt); err != nil {
			return nil, err
		}
		commits = append(commits, c)
	}
	return commits, rows.Err()
}

func (s *Store) ListFileChanges(f Filter) ([]domain.FileChange, error) {
	where, args := "", []any(nil)
	if f.RepoSlug != "" {
		where, args = " WHERE repo_slug = ?", []any{f.RepoSlug}
	}
	rows, err := s.db.Query("SELECT commit_hash, repo_slug, path, lines_added, lines_removed, status FROM file_changes"+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var changes []domain.FileChange
	for rows.Next() {
		var fc domain.FileChange
		if err := rows.Scan(&fc.CommitHash, &fc.RepoSlug, &fc.Path, &fc.LinesAdded, &fc.LinesRemoved, &fc.Status); err != nil {
			return nil, err
		}
		changes = append(changes, fc)
	}
	return changes, rows.Err()
}
