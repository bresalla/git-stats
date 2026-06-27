# Concepts

Shared domain vocabulary for this project — entities, named processes, and status concepts with project-specific meaning. Seeded with core domain vocabulary, then accretes as ce-compound and ce-compound-refresh process learnings; direct edits are fine. Glossary only, not a spec or catch-all.

## Pull Request Lifecycle

### Pull Request
A change proposal tracked through a state taxonomy: Open, Merged, Declined, or Superseded. Only Merged pull requests have a merge timestamp, and only Merged pull requests participate in Delivery Flow timing metrics (Cycle Time, Time to First Review, Time in Review) — Open, Declined, and Superseded ones are excluded from those calculations entirely.

### Review
A single reviewer action (approve or comment) recorded against a Pull Request. The earliest Review on a Pull Request marks the boundary between Time to First Review and Time in Review.

### Author
A person identified by their source-control account ID.
*Avoid:* contributor, committer

The **Allowlisted** flag on an Author marks which authors count toward commit-activity metrics (excluding bots, imported history, or other accounts the team has chosen not to track). This filter currently applies only to commit-count/activity metrics — Pull Request authorship-based metrics (breakdowns, summaries) are not filtered by Allowlisted status.

## Delivery Flow

### Delivery Flow
The dashboard and underlying metric set — Cycle Time, Time to First Review, Time in Review, and their aggregates, percentile distributions, and breakdowns — that characterizes how Pull Requests move from creation to merge.

### Cycle Time
Elapsed time from a Pull Request's creation to its merge. Only defined for Merged pull requests.

### Time to First Review
Elapsed time from a Pull Request's creation to its earliest Review.

### Time in Review
Elapsed time from a Pull Request's earliest Review to its merge. Distinct from Cycle Time, which spans the full creation-to-merge window including any time before the first Review.
