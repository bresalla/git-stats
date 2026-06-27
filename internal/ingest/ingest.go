package ingest

import (
	"context"
	"fmt"

	"git-statistics/internal/bitbucket"
	"git-statistics/internal/domain"
	"git-statistics/internal/normalize"
	"git-statistics/internal/storage"
)

type Syncer struct {
	Client    *bitbucket.Client
	Store     *storage.Store
	Workspace string
	Authors   []string
}

func (s *Syncer) SyncRepo(ctx context.Context, repoSlug string) error {
	since, err := s.Store.GetRepositorySyncedAt(repoSlug)
	if err != nil {
		return fmt.Errorf("reading watermark for %s: %w", repoSlug, err)
	}

	rawCommits, err := s.Client.ListCommits(ctx, s.Workspace, repoSlug, since)
	if err != nil {
		return fmt.Errorf("syncing %s: %w", repoSlug, err)
	}

	// Bitbucket only includes an email-bearing "raw" committer string on commits;
	// PR and review actors are reported as bare accounts with no email. Remember
	// emails seen on commits so those later authors can still be allowlist-matched
	// by ID instead of always failing the email check.
	authorEmails := map[string]string{}

	latest := since
	for _, raw := range rawCommits {
		commit, author, err := normalize.Commit(repoSlug, raw)
		if err != nil {
			return fmt.Errorf("syncing %s: %w", repoSlug, err)
		}
		if !since.IsZero() && !commit.AuthoredAt.After(since) {
			continue
		}

		authorEmails[author.ID] = author.Email
		author.Allowlisted = normalize.IsAllowlisted(author, s.Authors)
		if err := s.Store.UpsertAuthor(author); err != nil {
			return fmt.Errorf("syncing %s: storing author: %w", repoSlug, err)
		}
		if err := s.Store.UpsertCommit(commit); err != nil {
			return fmt.Errorf("syncing %s: storing commit: %w", repoSlug, err)
		}

		rawDiffstat, err := s.Client.GetDiffstat(ctx, s.Workspace, repoSlug, commit.Hash)
		if err != nil {
			return fmt.Errorf("syncing %s: %w", repoSlug, err)
		}
		for _, fc := range normalize.FileChanges(repoSlug, commit.Hash, rawDiffstat) {
			if err := s.Store.UpsertFileChange(fc); err != nil {
				return fmt.Errorf("syncing %s: storing file change: %w", repoSlug, err)
			}
		}

		if commit.AuthoredAt.After(latest) {
			latest = commit.AuthoredAt
		}
	}

	rawPRs, err := s.Client.ListPullRequests(ctx, s.Workspace, repoSlug)
	if err != nil {
		return fmt.Errorf("syncing %s: %w", repoSlug, err)
	}
	for _, rawPR := range rawPRs {
		pr, author, err := normalize.PullRequest(repoSlug, rawPR)
		if err != nil {
			return fmt.Errorf("syncing %s: %w", repoSlug, err)
		}
		if email, ok := authorEmails[author.ID]; ok {
			author.Email = email
		}
		author.Allowlisted = normalize.IsAllowlisted(author, s.Authors)
		if err := s.Store.UpsertAuthor(author); err != nil {
			return fmt.Errorf("syncing %s: storing author: %w", repoSlug, err)
		}
		if err := s.Store.UpsertPullRequest(pr); err != nil {
			return fmt.Errorf("syncing %s: storing pull request: %w", repoSlug, err)
		}

		rawActivity, err := s.Client.ListActivity(ctx, s.Workspace, repoSlug, pr.ID)
		if err != nil {
			return fmt.Errorf("syncing %s: %w", repoSlug, err)
		}
		for _, ra := range rawActivity {
			review, reviewer, ok := normalize.Review(repoSlug, pr.ID, ra)
			if !ok {
				continue
			}
			if email, ok := authorEmails[reviewer.ID]; ok {
				reviewer.Email = email
			}
			reviewer.Allowlisted = normalize.IsAllowlisted(reviewer, s.Authors)
			if err := s.Store.UpsertAuthor(reviewer); err != nil {
				return fmt.Errorf("syncing %s: storing reviewer: %w", repoSlug, err)
			}
			if err := s.Store.UpsertReview(review); err != nil {
				return fmt.Errorf("syncing %s: storing review: %w", repoSlug, err)
			}
		}
	}

	if err := s.Store.UpsertRepository(domain.Repository{Slug: repoSlug, Workspace: s.Workspace, SyncedAt: latest}); err != nil {
		return fmt.Errorf("syncing %s: advancing watermark: %w", repoSlug, err)
	}
	return nil
}

func (s *Syncer) SyncAll(ctx context.Context, repoSlugs []string) []error {
	var errs []error
	for _, slug := range repoSlugs {
		if err := s.SyncRepo(ctx, slug); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}
