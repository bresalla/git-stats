# Git Analytics App — MVP Design

## Purpose

Deliver the smallest end-to-end slice of the Git Analytics App (see [README.md](../../../README.md)) that proves the ingestion → normalization → metrics → dashboard pipeline against a real data source, scoped to one team's repos.

## Scope

**In scope for MVP:**
- Data source: Bitbucket Cloud (bitbucket.org), workspace `rdwrcloud`, project `WB`.
- A specific, explicitly configured list of repo slugs within that project (not "all repos in the project").
- Entities: Repository, Commit, PullRequest, Review, Author, FileChange.
- Author scope: a configured allowlist of team authors (usernames/emails), loaded from a static YAML config file. Dashboards filter to this list by default.
- Dashboards: **Delivery flow** (PR cycle time, time-to-first-review, time-in-review), **Code churn** (top changed files/folders, repeated-edit hotspots), and an **Activity** author breakdown (commits per author).
- Filters: date range, repo, author (within the allowlist).
- Sync: scheduled background sync on a configurable interval, plus a manual "Sync now" trigger.
- UI: server-rendered Go templates + HTMX, served from the same binary as the backend.
- Storage: SQLite.

**Explicitly out of scope for MVP** (per [README.md](../../../README.md) non-goals, deferred to future iterations):
- GitHub/GitLab support.
- The Overview KPI/trend dashboard.
- CSV export.
- Anomaly detection, alerts, Slack/email digests, AI-generated summaries.
- Role-based access control, audit logs.
- Deployment metadata / production lead time.
- A UI for managing the author allowlist or repo list (config-file only for MVP).

## Architecture

A single Go binary, modular internally:

- **`ingest`** — polls the Bitbucket Cloud REST API v2.0 for commits and pull requests (including reviews/activity) for each configured repo, using a per-repo `synced_at` watermark to fetch incrementally.
- **`normalize`** — maps raw Bitbucket API responses into common entities (Repository, Commit, PullRequest, Review, Author, FileChange), tagging each Author as in/out of the configured allowlist.
- **`metrics`** — computes aggregates on demand from normalized data: PR cycle time, time-to-first-review, time-in-review, churn hotspots, commits-per-author.
- **`web`** — Go templates + HTMX rendering the Delivery Flow, Code Churn, and Activity dashboards, with date range / repo / author filters. Calls into `metrics` directly (in-process), no separate HTTP API contract needed for MVP.
- **Scheduler** — a background ticker (configurable interval, e.g. every N minutes) invokes `ingest` for each configured repo; a "Sync now" button/endpoint triggers the same path on demand.

Configuration lives in a single YAML file:
```yaml
bitbucket:
  workspace: rdwrcloud
  project: WB
  repos:
    - repo-slug-1
    - repo-slug-2
sync_interval_minutes: 30
authors:
  - alice@example.com
  - bob@example.com
```
Bitbucket credentials (app password / API token) are supplied via environment variable, never committed to the config file or repo.

## Data flow

1. Scheduler (or manual trigger) calls `ingest.SyncRepo(repoSlug)` for each configured repo.
2. `ingest` fetches commits and PRs (with reviews/activity) from Bitbucket since that repo's `synced_at` watermark.
3. `normalize` converts raw payloads into common entities and tags each Author against the allowlist. Non-allowlisted authors' data is stored (not dropped) but excluded from default dashboard views — this lets the allowlist be expanded later without re-syncing.
4. Normalized entities are upserted into SQLite; the repo's `synced_at` watermark advances to the latest fetched timestamp.
5. On dashboard page load, `metrics` queries SQLite for the selected date range, repo, and author filters, and computes aggregates on the fly (MVP data volumes don't need pre-aggregation/caching).
6. `web` renders:
   - **Delivery flow**: PR cycle time, time-to-first-review, time-in-review as line charts/boxplots over the date range.
   - **Code churn**: top changed files/folders and repeated-edit hotspots.
   - **Activity**: commits per allowlisted author over the date range.

## Error handling

- A failed sync for a repo (Bitbucket API error, rate limit, network failure) is logged and leaves that repo's watermark unchanged, so the next scheduled or manual sync retries from the same point. No partial or duplicate data is written.
- Each repo syncs independently — one repo's failure doesn't block others.
- Invalid or incomplete config (unknown repo slug, empty author list, missing credentials) fails fast at startup with a clear error message.

## Testing

- `normalize`: unit tests using fixture Bitbucket API JSON payloads, asserting the resulting common entities.
- `metrics`: unit tests using known sets of normalized entities, asserting expected aggregate values (PR cycle time, churn, etc.).
- `ingest`: tested against a mocked HTTP client — no live Bitbucket calls in tests.
- One integration test exercising the full path (fixture data → sync → normalize → metrics → SQLite → query) end-to-end.

## Future enhancements (not MVP)

GitHub/GitLab support, Overview dashboard, CSV export, anomaly detection/alerts, RBAC, audit logs, deployment-aware lead time, allowlist/repo-list admin UI — see [README.md](../../../README.md) for the full long-term feature set.
