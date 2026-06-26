package normalize

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"git-statistics/internal/bitbucket"
	"git-statistics/internal/domain"
)

var emailPattern = regexp.MustCompile(`<([^>]+)>`)

func Commit(repoSlug string, raw bitbucket.RawCommit) (domain.Commit, domain.Author, error) {
	authoredAt, err := time.Parse(time.RFC3339, raw.Date)
	if err != nil {
		return domain.Commit{}, domain.Author{}, fmt.Errorf("parsing commit date %q: %w", raw.Date, err)
	}

	author := domain.Author{
		ID:          raw.Author.User.AccountID,
		DisplayName: raw.Author.User.DisplayName,
		Email:       extractEmail(raw.Author.Raw),
	}

	commit := domain.Commit{
		Hash:       raw.Hash,
		RepoSlug:   repoSlug,
		AuthorID:   author.ID,
		Message:    raw.Message,
		AuthoredAt: authoredAt,
	}
	return commit, author, nil
}

func extractEmail(raw string) string {
	match := emailPattern.FindStringSubmatch(raw)
	if len(match) < 2 {
		return ""
	}
	return match[1]
}

func FileChanges(repoSlug, commitHash string, raw []bitbucket.RawDiffstatEntry) []domain.FileChange {
	changes := make([]domain.FileChange, 0, len(raw))
	for _, entry := range raw {
		path := ""
		if entry.New != nil {
			path = entry.New.Path
		} else if entry.Old != nil {
			path = entry.Old.Path
		}
		changes = append(changes, domain.FileChange{
			CommitHash:   commitHash,
			RepoSlug:     repoSlug,
			Path:         path,
			LinesAdded:   entry.LinesAdded,
			LinesRemoved: entry.LinesRemoved,
			Status:       entry.Status,
		})
	}
	return changes
}

func IsAllowlisted(author domain.Author, allowlist []string) bool {
	for _, email := range allowlist {
		if strings.EqualFold(email, author.Email) {
			return true
		}
	}
	return false
}

func PullRequest(repoSlug string, raw bitbucket.RawPullRequest) (domain.PullRequest, domain.Author, error) {
	createdAt, err := time.Parse(time.RFC3339, raw.CreatedOn)
	if err != nil {
		return domain.PullRequest{}, domain.Author{}, fmt.Errorf("parsing PR created_on %q: %w", raw.CreatedOn, err)
	}
	updatedAt, err := time.Parse(time.RFC3339, raw.UpdatedOn)
	if err != nil {
		return domain.PullRequest{}, domain.Author{}, fmt.Errorf("parsing PR updated_on %q: %w", raw.UpdatedOn, err)
	}

	author := domain.Author{
		ID:          raw.Author.User.AccountID,
		DisplayName: raw.Author.User.DisplayName,
		Email:       extractEmail(raw.Author.Raw),
	}

	pr := domain.PullRequest{
		ID:        raw.ID,
		RepoSlug:  repoSlug,
		Title:     raw.Title,
		AuthorID:  author.ID,
		State:     raw.State,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}
	if raw.State == "MERGED" {
		mergedAt := updatedAt
		pr.MergedAt = &mergedAt
	}
	return pr, author, nil
}

func Review(repoSlug string, pullRequestID int, raw bitbucket.RawActivity) (domain.Review, domain.Author, bool) {
	switch {
	case raw.Approval != nil:
		createdAt, err := time.Parse(time.RFC3339, raw.Approval.Date)
		if err != nil {
			return domain.Review{}, domain.Author{}, false
		}
		author := domain.Author{
			ID:          raw.Approval.User.User.AccountID,
			DisplayName: raw.Approval.User.User.DisplayName,
			Email:       extractEmail(raw.Approval.User.Raw),
		}
		return domain.Review{
			ID:            fmt.Sprintf("%s-%d-approval-%s", repoSlug, pullRequestID, author.ID),
			PullRequestID: pullRequestID,
			RepoSlug:      repoSlug,
			ReviewerID:    author.ID,
			Action:        "approved",
			CreatedAt:     createdAt,
		}, author, true
	case raw.Comment != nil:
		createdAt, err := time.Parse(time.RFC3339, raw.Comment.CreatedOn)
		if err != nil {
			return domain.Review{}, domain.Author{}, false
		}
		author := domain.Author{
			ID:          raw.Comment.User.User.AccountID,
			DisplayName: raw.Comment.User.User.DisplayName,
			Email:       extractEmail(raw.Comment.User.Raw),
		}
		return domain.Review{
			ID:            fmt.Sprintf("%s-%d-comment-%s-%s", repoSlug, pullRequestID, author.ID, createdAt.Format(time.RFC3339)),
			PullRequestID: pullRequestID,
			RepoSlug:      repoSlug,
			ReviewerID:    author.ID,
			Action:        "commented",
			CreatedAt:     createdAt,
		}, author, true
	default:
		return domain.Review{}, domain.Author{}, false
	}
}
