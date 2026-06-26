package storage

import (
	"database/sql"
	"time"

	"git-statistics/internal/domain"
)

func (s *Store) UpsertRepository(r domain.Repository) error {
	_, err := s.db.Exec(`
		INSERT INTO repositories (slug, workspace, synced_at)
		VALUES (?, ?, ?)
		ON CONFLICT(slug) DO UPDATE SET workspace = excluded.workspace, synced_at = excluded.synced_at
	`, r.Slug, r.Workspace, r.SyncedAt)
	return err
}

func (s *Store) GetRepositorySyncedAt(slug string) (time.Time, error) {
	var syncedAt sql.NullTime
	row := s.db.QueryRow("SELECT synced_at FROM repositories WHERE slug = ?", slug)
	err := row.Scan(&syncedAt)
	if err == sql.ErrNoRows {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, err
	}
	if !syncedAt.Valid {
		return time.Time{}, nil
	}
	return syncedAt.Time, nil
}

func (s *Store) UpsertAuthor(a domain.Author) error {
	_, err := s.db.Exec(`
		INSERT INTO authors (id, display_name, email, allowlisted)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET display_name = excluded.display_name, email = excluded.email, allowlisted = excluded.allowlisted
	`, a.ID, a.DisplayName, a.Email, a.Allowlisted)
	return err
}

func (s *Store) ListAuthors() ([]domain.Author, error) {
	rows, err := s.db.Query("SELECT id, display_name, email, allowlisted FROM authors")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var authors []domain.Author
	for rows.Next() {
		var a domain.Author
		if err := rows.Scan(&a.ID, &a.DisplayName, &a.Email, &a.Allowlisted); err != nil {
			return nil, err
		}
		authors = append(authors, a)
	}
	return authors, rows.Err()
}
