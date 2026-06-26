package metrics

import (
	"sort"
	"time"

	"git-statistics/internal/storage"
)

type PullRequestFlow struct {
	PullRequestID     int
	Title             string
	CycleTime         time.Duration
	TimeToFirstReview time.Duration
	TimeInReview      time.Duration
}

type FileChurn struct {
	Path         string
	CommitCount  int
	LinesChanged int
}

type AuthorActivity struct {
	AuthorID    string
	DisplayName string
	CommitCount int
}

func DeliveryFlow(store *storage.Store, f storage.Filter) ([]PullRequestFlow, error) {
	prs, err := store.ListPullRequests(f)
	if err != nil {
		return nil, err
	}

	var flows []PullRequestFlow
	for _, pr := range prs {
		if pr.MergedAt == nil {
			continue
		}
		flow := PullRequestFlow{
			PullRequestID: pr.ID,
			Title:         pr.Title,
			CycleTime:     pr.MergedAt.Sub(pr.CreatedAt),
		}

		reviews, err := store.ListReviews(pr.RepoSlug, pr.ID)
		if err != nil {
			return nil, err
		}
		if len(reviews) > 0 {
			firstReview := reviews[0].CreatedAt
			flow.TimeToFirstReview = firstReview.Sub(pr.CreatedAt)
			flow.TimeInReview = pr.MergedAt.Sub(firstReview)
		}
		flows = append(flows, flow)
	}
	return flows, nil
}

func ChurnHotspots(store *storage.Store, f storage.Filter) ([]FileChurn, error) {
	changes, err := store.ListFileChanges(f)
	if err != nil {
		return nil, err
	}

	type agg struct {
		commits map[string]bool
		lines   int
	}
	byPath := map[string]*agg{}
	for _, c := range changes {
		a, ok := byPath[c.Path]
		if !ok {
			a = &agg{commits: map[string]bool{}}
			byPath[c.Path] = a
		}
		a.commits[c.CommitHash] = true
		a.lines += c.LinesAdded + c.LinesRemoved
	}

	churn := make([]FileChurn, 0, len(byPath))
	for path, a := range byPath {
		churn = append(churn, FileChurn{Path: path, CommitCount: len(a.commits), LinesChanged: a.lines})
	}
	sort.Slice(churn, func(i, j int) bool { return churn[i].LinesChanged > churn[j].LinesChanged })
	return churn, nil
}

func CommitsPerAuthor(store *storage.Store, f storage.Filter) ([]AuthorActivity, error) {
	commits, err := store.ListCommits(f)
	if err != nil {
		return nil, err
	}
	authors, err := store.ListAuthors()
	if err != nil {
		return nil, err
	}

	allowlisted := map[string]string{}
	for _, a := range authors {
		if a.Allowlisted {
			allowlisted[a.ID] = a.DisplayName
		}
	}

	counts := map[string]int{}
	for _, c := range commits {
		if _, ok := allowlisted[c.AuthorID]; ok {
			counts[c.AuthorID]++
		}
	}

	activity := make([]AuthorActivity, 0, len(counts))
	for authorID, count := range counts {
		activity = append(activity, AuthorActivity{AuthorID: authorID, DisplayName: allowlisted[authorID], CommitCount: count})
	}
	sort.Slice(activity, func(i, j int) bool { return activity[i].CommitCount > activity[j].CommitCount })
	return activity, nil
}
