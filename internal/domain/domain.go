package domain

import "time"

type Repository struct {
	Slug      string
	Workspace string
	SyncedAt  time.Time
}

type Author struct {
	ID          string // Bitbucket account_id
	DisplayName string
	Email       string
	Allowlisted bool
}

type Commit struct {
	Hash       string
	RepoSlug   string
	AuthorID   string
	Message    string
	AuthoredAt time.Time
}

type FileChange struct {
	CommitHash   string
	RepoSlug     string
	Path         string
	LinesAdded   int
	LinesRemoved int
	Status       string // "added", "modified", "removed", "renamed"
}

type PullRequest struct {
	ID        int
	RepoSlug  string
	Title     string
	AuthorID  string
	State     string // "OPEN", "MERGED", "DECLINED", "SUPERSEDED"
	CreatedAt time.Time
	UpdatedAt time.Time
	MergedAt  *time.Time
}

type Review struct {
	ID            string
	PullRequestID int
	RepoSlug      string
	ReviewerID    string
	Action        string // "approved", "commented"
	CreatedAt     time.Time
}
