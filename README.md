# Git Analytics App Spec

## Purpose
Build an internal application that analyzes Git activity across repositories and shows engineering flow metrics, code churn, and delivery bottlenecks in a team-friendly dashboard [web:25][web:30].

## Goals
- Show how delivery flow changes over time.
- Help spot bottlenecks in review, merge, and release processes.
- Reveal unstable modules and high-churn areas.
- Provide team-level insights rather than individual ranking [web:29][web:30].

## Non-goals
- Performance ranking of developers.
- Full project management replacement.
- Deep static code analysis.
- Opinionated process enforcement.

## Target users
- Engineering leads.
- Backend developers.
- DevOps and platform engineers.
- Project or delivery managers [web:25][web:30].

## Core metrics
- Commits per day and week.
- Commits by author.
- Commit size distribution.
- Changed files and modules frequency.
- PR cycle time.
- Time to first review.
- Time in review.
- Merge rate.
- Rework rate or churn rate.
- Activity heatmap by weekday and hour [web:28][web:30].

## Main dashboards

### Overview
Show KPI cards for the selected time range, plus trend charts for commit volume, PR cycle time, merge rate, and churn [web:26][web:30].

### Activity
Show commit frequency by day, author contribution, and a time-of-day heatmap [web:12][web:14].

### Delivery flow
Show PR cycle time, review lag, and merge delays using line charts and boxplots [web:26][web:30].

### Code churn
Show top changing files, folders, and modules, plus repeated edits to recently changed code [web:28][web:30].

## Filters
- Repository.
- Branch.
- Author.
- Team.
- Date range.
- PR state.
- Commit type, if available.

## Data sources
- GitHub.
- GitLab.
- Bitbucket.
- Optional deployment metadata for production lead time [web:25][web:26].

## Data model
Store normalized entities for:
- Repository.
- Commit.
- Pull request.
- Review.
- Merge.
- Deployment.
- Author.
- File change.
- Time bucket [web:25][web:26].

## Functional requirements
- Sync repositories on a schedule.
- Support incremental updates.
- Calculate metrics daily and on demand.
- Allow drill-down from chart to raw commits or PRs.
- Export filtered datasets as CSV.
- Support alerts for anomalies such as sudden activity drops or long review lag [web:25][web:30].

## Non-functional requirements
- Multi-repo support.
- Role-based access control.
- Fast query response for 7/30/90 day ranges.
- Privacy-friendly defaults with aggregated team views first.
- Audit logs for sync and configuration changes [web:21][web:30].

## Suggested architecture
- Ingestion service pulls data from Git provider APIs.
- Normalization layer maps events into a common schema.
- Metrics engine computes aggregates and derived metrics.
- API layer serves dashboards and exports.
- Web UI renders charts and tables [web:25][web:26].

## MVP
1. Connect GitHub and GitLab repositories.
2. Pull commits, PRs, reviews, and merges.
3. Show overview dashboard.
4. Show activity heatmap.
5. Show PR cycle time and review lag.
6. Show top files and churn.
7. Support CSV export [web:25][web:26][web:30].

## Future enhancements
- Slack or email digests.
- Anomaly detection.
- Team benchmarks.
- Deployment-aware lead time.
- AI-generated summaries [web:21][web:24].
