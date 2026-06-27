---
title: Verify a plan's assumed APIs against the real codebase before implementing it
date: 2026-06-27
category: docs/solutions/workflow-issues/
module: "internal/metrics, internal/web"
problem_type: workflow_issue
component: development_workflow
severity: medium
applies_when:
  - "The plan was authored in a separate session/context from the one executing it, so it could not have referenced the current, exact state of the code"
  - "The plan assumes interfaces, generic filter/parameter types, or abstractions that feel idiomatic but may not exist in this codebase's actual (often more concrete, less abstracted) design"
  - "The plan references a domain concept (e.g. \"team\") that may or may not actually exist in the schema/domain model"
  - "The plan includes nullable-looking fields (timestamps only set conditionally, like a merge date) whose real type may be a pointer/nullable type"
related_components:
  - internal/storage
  - internal/domain
  - internal/metrics
  - internal/web
tags: [plan-drift, api-verification, go, tdd, data-model-mismatch, scope-reduction]
---

# Verify a plan's assumed APIs against the real codebase before implementing it

## Context

The `/ce-work` skill was used to execute a written implementation plan, `docs/superpowers/plans/2026-06-27-pr-statistics-dashboard.md`, which added PR summary stats, percentile distributions, and repo/author breakdowns to a "Delivery Flow" dashboard in the `internal/metrics` and `internal/web` packages of this Go project (module `git-statistics`).

The plan's code samples were written against an imagined, generic data-access layer rather than the actual codebase. It assumed:

- A `Store` *interface* (not a concrete type) with a method `QueryPullRequests(filter FilterParams) ([]*domain.PullRequest, error)`.
- A generic `FilterParams` type carrying, among other things, a PR-state filter (e.g. `State: "MERGED"`).
- `domain.PullRequest.UpdatedAt` as the field to use for merge timestamp / cycle-time math.
- A `domain.Author` type with team membership, which a planned `BreakdownByTeam` function would group merged PRs by.

None of this matched reality:

- `*storage.Store` is a concrete struct defined in `internal/storage/sqlite.go`, not an interface.
- The actual query method is `ListPullRequests(f storage.Filter) ([]domain.PullRequest, error)` — note the return is a slice of values, not pointers, and the method name differs from the plan's `QueryPullRequests`.
- Reviews are fetched with `ListReviews(repoSlug string, pullRequestID int) ([]domain.Review, error)` — two scalar arguments, not a filter struct.
- `storage.Filter` only has `RepoSlug`, `AuthorID string`, and `From`, `To time.Time`. There is no PR-state field at all.
- `domain.PullRequest.MergedAt` is `*time.Time` — nullable, with open PRs carrying `nil`. Cycle time has to be computed from `MergedAt`, not `UpdatedAt`, and every consumer must nil-check it.
- `domain.Author` is `{ID, DisplayName, Email string; Allowlisted bool}`. A repo-wide `grep -rn "team|Team" --include=*.go` returned zero matches — there is no team concept anywhere in the schema, domain model, or existing code. Had `BreakdownByTeam` been implemented literally per the plan, it would have required inventing a team mapping with no backing data, i.e. fabricating fictional output for a dashboard that's explicitly meant to report real engineering metrics.

This is a recurring failure mode for plan-driven implementation: a plan written before (or without deep reference to) the real codebase encodes assumptions about types and signatures that feel plausible but are wrong, and naively transcribing its code samples either fails to compile or, worse, compiles by coincidence while computing the wrong thing (e.g. silently treating `nil` `MergedAt` as a valid merge date, or quietly inventing team data).

## Guidance

Before writing any new function that the plan specifies, read the actual current source of every type and function the plan would call — don't trust the plan's snippets as ground truth, treat them as a sketch of intent.

In this session that meant reading, in full, before writing code:
- `internal/storage/pullrequests.go`, `internal/storage/repository.go`, `internal/storage/sqlite.go` — for the real `Store` struct, `Filter` struct, and the SQL/table shape behind them.
- `internal/domain/domain.go` — for the real `PullRequest`, `Author`, `Review` struct shapes, including which fields are pointers/nullable.
- The existing `internal/metrics/metrics.go` + `internal/metrics/metrics_test.go` and `internal/web/handlers.go` + `internal/web/web_test.go` — to match established naming and testing conventions (how `DeliveryFlow`, `ChurnHotspots`, `CommitsPerAuthor` are structured; how `openTestStore`/`must` test helpers are used) rather than inventing a parallel style.

Plan-assumed signatures vs. what was actually implemented:

```go
// Plan assumed (does not exist in this codebase):
type Store interface {
    QueryPullRequests(filter FilterParams) ([]*domain.PullRequest, error)
}

type FilterParams struct {
    RepoSlug string
    AuthorID string
    State    string // e.g. "MERGED"
    From, To time.Time
}

func BreakdownByTeam(store Store, f FilterParams) (map[string]TeamStats, error)
```

```go
// Actual, derived from reading internal/storage and internal/domain:
func SummaryStats(store *storage.Store, f storage.Filter) (*PRSummaryStats, error)
func Distributions(store *storage.Store, f storage.Filter) (*DistributionMetrics, error)
func BreakdownByRepository(store *storage.Store, f storage.Filter) ([]BreakdownRow, error)
func BreakdownByAuthor(store *storage.Store, f storage.Filter) ([]BreakdownRow, error) // same BreakdownRow type, grouped by repo or author

// storage.Filter has no state field; "merged" is determined by nullability:
for _, pr := range prs {
    if pr.MergedAt == nil {
        continue // open PR, exclude from merged-PR stats
    }
    cycleTime := pr.MergedAt.Sub(pr.CreatedAt)
    // ...
}
```

`BreakdownByTeam` was dropped from scope entirely rather than implemented against fabricated team data, since there is no team concept anywhere in the system.

Checklist for verifying any implementation plan against a real codebase before writing code from it:

1. **Read the real types first.** For every struct, interface, and function signature the plan references, open the actual source file and confirm the plan's version matches field-for-field and arg-for-arg. Assume mismatches exist; don't assume the plan author had the file open.
2. **Build incrementally.** After drafting the first new exported type or function, run the build (`go build ./...`) immediately — before writing the next function on top of it — so a signature mismatch is caught in seconds, not after an entire feature's worth of code has been written against a wrong assumption.
3. **Drop fabricated scope rather than inventing it.** If a plan calls for a capability the codebase has no data or concept for (here: team membership), do not synthesize fake data or a placeholder mapping to make the planned function "work." Cut that piece of scope.
4. **Flag scope reductions to the user in the same turn.** State plainly what was dropped and why, e.g.: "the plan assumes a Team breakdown and a generic Store/FilterParams interface, but the actual codebase has no team concept and uses concrete `*storage.Store`/`storage.Filter` types. I'm adapting the plan to real signatures and dropping the Team breakdown." Silent omission is worse than an explicit, justified cut.

A related, smaller pattern from the same session: when a plan's UI step proposes script-driven interactivity (here, an `onclick` JS toggle for collapsible dashboard sections) but the plan's own stated constraint is "client-side uses only basic HTML/CSS toggle," prefer the native HTML mechanism — `<details>`/`<summary>` — over the scripted one. It satisfies the plan's actual constraint with less code and no JS to maintain.

## Why This Matters

Transcribing a plan's pseudocode literally, without first checking it against the real types, produces one of two bad outcomes:

- **A compile error**, which is the cheap failure — annoying but caught immediately.
- **Silently wrong behavior**, which is the expensive failure. In this case, the plan's implicit model (`UpdatedAt` as merge time, a non-nullable timestamp, a string `State` field) would have caused cycle-time calculations to include open PRs as if they were merged, or to use the wrong timestamp for ones that were merged — numbers that look plausible on a dashboard but are quietly incorrect. For a metrics dashboard whose entire purpose is to be a trustworthy source of engineering-flow numbers, a metric that's wrong but doesn't error is worse than a metric that's simply missing.

Inventing data to satisfy a planned feature (the `BreakdownByTeam` case) is a more severe version of the same failure: it produces a dashboard panel that looks authoritative but reports numbers with no real backing — actively misleading rather than merely incomplete.

Catching mismatches early via incremental builds also bounds the blast radius of a wrong assumption to one function instead of an entire layer of dependent code.

## When to Apply

- The plan was authored in a separate session/context from the one executing it, so it could not have referenced the current, exact state of the code.
- The plan assumes interfaces, generic filter/parameter types, or abstractions that "feel" idiomatic but may not exist in this codebase's actual (often more concrete, less abstracted) design.
- The plan references a domain concept (here, "team") that may or may not actually exist in the schema/domain model — verify with a repo-wide search before building on it.
- The plan includes nullable-looking fields (timestamps that are only set conditionally, like a merge date) — check whether the real field is a pointer/nullable type before writing arithmetic or comparisons against it.

It does not apply to plans that are pure prose/sequencing instructions with no code references, since there's nothing to verify against real types in that case.

## Examples

**Before (plan's imagined contract):**

```go
prs, err := store.QueryPullRequests(FilterParams{
    RepoSlug: repo,
    State:    "MERGED",
    From:     from,
    To:       to,
})
if err != nil {
    return nil, err
}
for _, pr := range prs {
    cycleTime := pr.UpdatedAt.Sub(pr.CreatedAt) // wrong field, and assumes always set
    // ...
}
```

**After (verified against `internal/storage/sqlite.go` and `internal/domain/domain.go`):**

```go
prs, err := store.ListPullRequests(storage.Filter{
    RepoSlug: repo,
    From:     from,
    To:       to,
})
if err != nil {
    return nil, err
}
for _, pr := range prs {
    if pr.MergedAt == nil {
        continue // open PR; not part of merged-PR cycle-time stats
    }
    cycleTime := pr.MergedAt.Sub(pr.CreatedAt)
    // ...
}
```

**Before (plan's test scaffolding, against the imagined interface):**

```go
must(t, store.UpsertPullRequest(&domain.PullRequest{
    State: "MERGED",
    // ...
}))
```

**After (matching the real, nullable `MergedAt` field and existing test helper conventions in `internal/metrics/metrics_test.go`):**

```go
merged := time.Now()
must(t, store.UpsertPullRequest(domain.PullRequest{
    MergedAt: &merged,
    // ...
}))
```

**Verification step that paid off independent of the plan-mismatch issue:** after implementation, the full suite (`go test ./...`) passed, and the server was additionally started against real Bitbucket-backed data with the live `/delivery-flow` HTTP response fetched and inspected directly (grepping the response body for expected class names and computed values) when the Claude-in-Chrome browser-automation tool failed to connect after repeated retries. That raw-HTTP-fetch fallback is a valid substitute for browser automation when the latter is flaky, and in this instance it additionally surfaced an unrelated pre-existing data bug — `pull_requests.author_id` was empty for every row despite a fully populated `authors` table — purely because the rendered "By Author" breakdown visibly showed a blank key where an author name was expected. Verifying against the actual rendered output, not just unit tests, caught a bug that no amount of plan-conformance checking would have surfaced.

## Related
- None yet — this is the first documented solution in this project.
