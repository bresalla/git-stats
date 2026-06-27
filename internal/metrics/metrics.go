package metrics

import (
	"sort"
	"time"

	"git-statistics/internal/domain"
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

// PRSummaryStats holds aggregate statistics for PR timings across merged pull requests.
type PRSummaryStats struct {
	AvgCycleTime      time.Duration
	AvgFirstReview    time.Duration
	AvgInReview       time.Duration
	MedianCycleTime   time.Duration
	MedianFirstReview time.Duration
	MedianInReview    time.Duration
	MinCycleTime      time.Duration
	MinFirstReview    time.Duration
	MinInReview       time.Duration
	MaxCycleTime      time.Duration
	MaxFirstReview    time.Duration
	MaxInReview       time.Duration
}

// mergedPRDurations collects per-PR cycle time, time-to-first-review, and time-in-review
// for all merged pull requests matching the filter. Review-derived durations are only
// included for PRs that actually have a review.
func mergedPRDurations(store *storage.Store, f storage.Filter) (cycleTimes, firstReviewTimes, inReviewTimes []time.Duration, err error) {
	prs, err := store.ListPullRequests(f)
	if err != nil {
		return nil, nil, nil, err
	}

	for _, pr := range prs {
		if pr.MergedAt == nil {
			continue
		}
		cycleTimes = append(cycleTimes, pr.MergedAt.Sub(pr.CreatedAt))

		reviews, err := store.ListReviews(pr.RepoSlug, pr.ID)
		if err != nil {
			return nil, nil, nil, err
		}
		if len(reviews) > 0 {
			firstReview := reviews[0].CreatedAt
			firstReviewTimes = append(firstReviewTimes, firstReview.Sub(pr.CreatedAt))
			inReviewTimes = append(inReviewTimes, pr.MergedAt.Sub(firstReview))
		}
	}
	return cycleTimes, firstReviewTimes, inReviewTimes, nil
}

// SummaryStats computes aggregate PR timing statistics (avg/median/min/max) for merged
// pull requests matching the filter.
func SummaryStats(store *storage.Store, f storage.Filter) (*PRSummaryStats, error) {
	cycleTimes, firstReviewTimes, inReviewTimes, err := mergedPRDurations(store, f)
	if err != nil {
		return nil, err
	}

	return &PRSummaryStats{
		AvgCycleTime:      avgDuration(cycleTimes),
		AvgFirstReview:    avgDuration(firstReviewTimes),
		AvgInReview:       avgDuration(inReviewTimes),
		MedianCycleTime:   PercentileDurations(cycleTimes, 50),
		MedianFirstReview: PercentileDurations(firstReviewTimes, 50),
		MedianInReview:    PercentileDurations(inReviewTimes, 50),
		MinCycleTime:      minDuration(cycleTimes),
		MinFirstReview:    minDuration(firstReviewTimes),
		MinInReview:       minDuration(inReviewTimes),
		MaxCycleTime:      maxDuration(cycleTimes),
		MaxFirstReview:    maxDuration(firstReviewTimes),
		MaxInReview:       maxDuration(inReviewTimes),
	}, nil
}

// DistributionMetrics holds percentile distributions for the three PR timing metrics.
type DistributionMetrics struct {
	CycleTime   map[string]time.Duration
	FirstReview map[string]time.Duration
	InReview    map[string]time.Duration
}

// Distributions computes percentile distributions (P50/P75/P90/P95/P99) for merged PR timings.
func Distributions(store *storage.Store, f storage.Filter) (*DistributionMetrics, error) {
	cycleTimes, firstReviewTimes, inReviewTimes, err := mergedPRDurations(store, f)
	if err != nil {
		return nil, err
	}

	return &DistributionMetrics{
		CycleTime:   DistributionStats(cycleTimes),
		FirstReview: DistributionStats(firstReviewTimes),
		InReview:    DistributionStats(inReviewTimes),
	}, nil
}

// BreakdownRow represents aggregated PR metrics for a single breakdown dimension (e.g. one repo or author).
type BreakdownRow struct {
	Key            string
	AvgCycleTime   time.Duration
	AvgFirstReview time.Duration
	AvgInReview    time.Duration
	Count          int
}

// BreakdownByRepository returns PR metrics grouped by repository, limited to the top 10
// repositories by PR count and sorted by AvgCycleTime descending.
func BreakdownByRepository(store *storage.Store, f storage.Filter) ([]BreakdownRow, error) {
	prs, err := store.ListPullRequests(f)
	if err != nil {
		return nil, err
	}

	groups := map[string][]domain.PullRequest{}
	for _, pr := range prs {
		if pr.MergedAt == nil {
			continue
		}
		groups[pr.RepoSlug] = append(groups[pr.RepoSlug], pr)
	}

	rows, err := breakdownFromGroups(store, groups)
	if err != nil {
		return nil, err
	}
	return topNByCount(rows, 10), nil
}

// BreakdownByAuthor returns PR metrics grouped by author display name, limited to the top
// 10 authors by PR count and sorted by AvgCycleTime descending.
func BreakdownByAuthor(store *storage.Store, f storage.Filter) ([]BreakdownRow, error) {
	prs, err := store.ListPullRequests(f)
	if err != nil {
		return nil, err
	}

	authors, err := store.ListAuthors()
	if err != nil {
		return nil, err
	}
	displayNames := make(map[string]string, len(authors))
	for _, a := range authors {
		displayNames[a.ID] = a.DisplayName
	}

	groups := map[string][]domain.PullRequest{}
	for _, pr := range prs {
		if pr.MergedAt == nil {
			continue
		}
		key := pr.AuthorID
		if name, ok := displayNames[pr.AuthorID]; ok && name != "" {
			key = name
		}
		groups[key] = append(groups[key], pr)
	}

	rows, err := breakdownFromGroups(store, groups)
	if err != nil {
		return nil, err
	}
	return topNByCount(rows, 10), nil
}

// breakdownFromGroups computes per-group PR timing aggregates, sorted by AvgCycleTime descending.
func breakdownFromGroups(store *storage.Store, groups map[string][]domain.PullRequest) ([]BreakdownRow, error) {
	rows := make([]BreakdownRow, 0, len(groups))
	for key, groupPRs := range groups {
		var cycleTimes, firstReviewTimes, inReviewTimes []time.Duration
		for _, pr := range groupPRs {
			cycleTimes = append(cycleTimes, pr.MergedAt.Sub(pr.CreatedAt))

			reviews, err := store.ListReviews(pr.RepoSlug, pr.ID)
			if err != nil {
				return nil, err
			}
			if len(reviews) > 0 {
				firstReview := reviews[0].CreatedAt
				firstReviewTimes = append(firstReviewTimes, firstReview.Sub(pr.CreatedAt))
				inReviewTimes = append(inReviewTimes, pr.MergedAt.Sub(firstReview))
			}
		}

		rows = append(rows, BreakdownRow{
			Key:            key,
			AvgCycleTime:   avgDuration(cycleTimes),
			AvgFirstReview: avgDuration(firstReviewTimes),
			AvgInReview:    avgDuration(inReviewTimes),
			Count:          len(groupPRs),
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].AvgCycleTime > rows[j].AvgCycleTime })
	return rows, nil
}

// topNByCount returns the n rows with the highest Count, preserving the AvgCycleTime-descending order.
func topNByCount(rows []BreakdownRow, n int) []BreakdownRow {
	if len(rows) <= n {
		return rows
	}
	byCount := make([]BreakdownRow, len(rows))
	copy(byCount, rows)
	sort.Slice(byCount, func(i, j int) bool { return byCount[i].Count > byCount[j].Count })
	keep := map[string]bool{}
	for _, r := range byCount[:n] {
		keep[r.Key] = true
	}

	top := make([]BreakdownRow, 0, n)
	for _, r := range rows {
		if keep[r.Key] {
			top = append(top, r)
		}
	}
	return top
}

func avgDuration(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	var total time.Duration
	for _, d := range durations {
		total += d
	}
	return total / time.Duration(len(durations))
}

func minDuration(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	m := durations[0]
	for _, d := range durations {
		if d < m {
			m = d
		}
	}
	return m
}

func maxDuration(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	m := durations[0]
	for _, d := range durations {
		if d > m {
			m = d
		}
	}
	return m
}

// PRStatusCountRow represents PR state counts for a single breakdown dimension (author or repo).
type PRStatusCountRow struct {
	Key     string
	Open    int
	Merged  int
	Declined int
	Superseded int
}

// PRStatusByAuthor returns PR state counts grouped by author display name.
func PRStatusByAuthor(store *storage.Store, f storage.Filter) ([]PRStatusCountRow, error) {
	prs, err := store.ListPullRequests(f)
	if err != nil {
		return nil, err
	}

	authors, err := store.ListAuthors()
	if err != nil {
		return nil, err
	}
	displayNames := make(map[string]string, len(authors))
	for _, a := range authors {
		displayNames[a.ID] = a.DisplayName
	}

	// Group PRs by author display name
	groups := map[string][]domain.PullRequest{}
	for _, pr := range prs {
		key := pr.AuthorID
		if name, ok := displayNames[pr.AuthorID]; ok && name != "" {
			key = name
		}
		groups[key] = append(groups[key], pr)
	}

	rows := prStatusFromGroups(groups)
	sort.Slice(rows, func(i, j int) bool { return rows[i].Key < rows[j].Key })
	return rows, nil
}

// PRStatusByRepository returns PR state counts grouped by repository slug.
func PRStatusByRepository(store *storage.Store, f storage.Filter) ([]PRStatusCountRow, error) {
	prs, err := store.ListPullRequests(f)
	if err != nil {
		return nil, err
	}

	// Group PRs by repo slug
	groups := map[string][]domain.PullRequest{}
	for _, pr := range prs {
		groups[pr.RepoSlug] = append(groups[pr.RepoSlug], pr)
	}

	rows := prStatusFromGroups(groups)
	sort.Slice(rows, func(i, j int) bool { return rows[i].Key < rows[j].Key })
	return rows, nil
}

// prStatusFromGroups counts PRs by state for each group, ensuring all states are reported.
func prStatusFromGroups(groups map[string][]domain.PullRequest) []PRStatusCountRow {
	rows := make([]PRStatusCountRow, 0, len(groups))
	for key, groupPRs := range groups {
		row := PRStatusCountRow{Key: key, Open: 0, Merged: 0, Declined: 0, Superseded: 0}
		for _, pr := range groupPRs {
			switch pr.State {
			case "OPEN":
				row.Open++
			case "MERGED":
				row.Merged++
			case "DECLINED":
				row.Declined++
			case "SUPERSEDED":
				row.Superseded++
			}
		}
		rows = append(rows, row)
	}
	return rows
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
