---
title: How to run and connect to the MCP server mode
date: 2026-06-27
category: docs/solutions/documentation-gaps/
module: "cmd/server, internal/mcpserver"
problem_type: documentation_gap
component: tooling
severity: low
applies_when:
  - "running the app in MCP server mode instead of the web dashboard"
  - "connecting an external MCP client (e.g. VS Code Copilot, Claude Code, Claude Desktop) to this app over stdio"
  - "extending the tool set or prompt set exposed by internal/mcpserver"
  - "debugging why an MCP client cannot see or call git-statistics tools/prompts"
tags: [mcp, stdio, agents, tooling, cli-flag, go-sdk]
---

# How to run and connect to the MCP server mode

## Context

The app already exposed all of its engineering-flow metrics (PR cycle time, review lag, churn hotspots, commit activity, PR state breakdowns) through a Go web dashboard built on `internal/web`. That dashboard is good for humans clicking through charts, but it's a dead end for an external agent (VS Code Copilot, Claude Code, or any other MCP-aware tool) that wants to ask a question like "how is PR #482 going for each team member" and get structured data back — the agent would have to scrape HTML or reimplement the SQL queries from scratch. There was no programmatic, agent-friendly interface onto the already-synced metrics data.

To close that gap, a second run mode was added to the existing `cmd/server` binary: an MCP server mode, gated behind a new `-mcp` boolean flag in `cmd/server/main.go`. Selecting this mode skips the HTTP listener and the background sync scheduler entirely and instead runs an MCP server over stdio, built on `github.com/modelcontextprotocol/go-sdk/mcp`, that exposes the same metrics engine (`internal/metrics`) the dashboard uses — plus two new PR-status tools — as MCP tools, and bundles common investigation workflows as MCP prompts. The implementation lives in `cmd/server/mcpserver.go` (the `runMCPServer` function that wires everything together) and the new `internal/mcpserver` package (`tools.go` and `prompts.go`). There was no doc yet explaining how to actually start this mode, what it exposes, or how a client would be configured to talk to it — that's the gap this doc fills.

## Guidance

**1. Prerequisites — same as running the HTTP server.**
The MCP server mode shares config and storage loading with the HTTP path in `cmd/server/main.go`: `main()` always calls `config.Load(*configPath)` and `storage.Open(*dbPath)` before branching on `*mcpMode`. That means you need:
- A valid `config.yaml` (or whatever path you pass via `-config`) satisfying `internal/config/config.go`'s `validate()`: `bitbucket.workspace` set, `bitbucket.repos` non-empty, `sync_interval_minutes > 0`, `authors` non-empty, plus the environment variables `BITBUCKET_EMAIL` and `BITBUCKET_API_TOKEN` set (these are read from the environment, not the YAML, and `Load` fails fast if either is empty).
- A SQLite database file at the path given by `-db` (default `git-statistics.db`) that has already been populated by a normal sync run of this same binary in HTTP mode. The MCP server does not sync anything itself — see point 6 below.

**2. The exact command to start MCP mode.**
From `cmd/server/main.go`, the three flags are `-config` (default `"config.yaml"`), `-db` (default `"git-statistics.db"`), and `-mcp` (a bool flag, default `false`). To run in stdio MCP mode:

```
go run ./cmd/server -mcp -config config.yaml -db git-statistics.db
```

or, against a built binary:

```
./server -mcp -config config.yaml -db git-statistics.db
```

When `-mcp` is true, `main()` calls `runMCPServer(store)` and returns immediately afterward — it never starts the HTTP listener, the background `scheduler`, or the Bitbucket client. There is no `:8080` to connect to in this mode; the only interface is stdio.

**3. The tools exposed (`internal/mcpserver/tools.go`).**
All tools take the same `ToolInput` struct — `repo`, `author`, `from`, `to` — all optional strings, where `from`/`to` must be `"2006-01-02"`-formatted dates (parsed by `parseFilterFromTool`; an invalid date format returns an error from the tool). They map onto `storage.Filter{RepoSlug, AuthorID, From, To}`.

Registered by `RegisterToolsDashboardMetrics` (mirrors the existing dashboard panels):
- `delivery_flow` — PR cycle time, time-to-first-review, time-in-review per merged PR (`metrics.DeliveryFlow`); returns `{flows: []PullRequestFlow}`.
- `summary_stats` — aggregate average/median/min/max for the same timing metrics (`metrics.SummaryStats`); returns `{stats: PRSummaryStats}`.
- `distributions` — P50/P75/P90/P95/P99 percentile distributions for cycle time and review times (`metrics.Distributions`); returns `{distributions: DistributionMetrics}`.
- `churn_hotspots` — files ranked by lines changed and commit frequency (`metrics.ChurnHotspots`); returns `{hotspots: []FileChurn}`.
- `commits_per_author` — commit counts per allowlisted author (`metrics.CommitsPerAuthor`); returns `{activity: []AuthorActivity}`.
- `breakdown_by_repository` — PR timing metrics grouped by repo (`metrics.BreakdownByRepository`); returns `{rows: []BreakdownRow}`.
- `breakdown_by_author` — PR timing metrics grouped by author (`metrics.BreakdownByAuthor`); returns `{rows: []BreakdownRow}`.

Registered by `RegisterToolsPRStatus` (new, not previously on the dashboard):
- `pr_status_by_author` — counts of open/merged/declined/superseded PRs grouped by author display name (`metrics.PRStatusByAuthor`); returns `{rows: []PRStatusCountRow}`.
- `pr_status_by_repo` — same state counts grouped by repository slug (`metrics.PRStatusByRepository`); returns `{rows: []PRStatusCountRow}`.

**4. The prompts exposed (`internal/mcpserver/prompts.go`).**
`RegisterPrompts` registers three curated prompts, each of which returns a single user-role text message instructing the calling agent which tools to call, in what order, and with which arguments — the prompt itself does not call any tools, it scripts the agent's next moves:

- `investigate_progress` — args: `repo` (required), `author` (optional). With an author given, walks the agent through `pr_status_by_author` → `delivery_flow` → `commits_per_author`, all scoped to that repo+author, to build a picture of one person's progress. Without an author, walks through `pr_status_by_repo` → `delivery_flow` → `commits_per_author` → (optionally) `breakdown_by_author`, scoped to the whole repo, to build a picture of repo-wide progress.
- `team_activity_summary` — args: `repo` (optional), `from` (optional), `to` (optional). If `repo` is given, walks through `commits_per_author` → `breakdown_by_author` → `pr_status_by_author`, all scoped to that repo and date range. If `repo` is omitted, walks through the same three plus `breakdown_by_repository`, across all repos.
- `pr_health_check` — args: `repo` (required). Walks through `summary_stats` → `distributions` → `pr_status_by_repo`, then asks the agent to interpret the results for: whether median cycle time is reasonable, whether review-time percentiles indicate bottlenecks, the merged-vs-open/declined ratio, and any outliers worth investigating.

Both `investigate_progress` and `pr_health_check` return an error from their handler if `repo` is missing, since it's a required argument.

**5. Configuring an MCP client to launch this server.**
This repo has no committed client config (no `.vscode/mcp.json`, no `claude_desktop_config.json`) to point to, but the shape is the standard stdio-MCP-server entry: a `command` plus `args` that exec the binary with `-mcp` and the config/db paths. For example, in a generic `mcp.json`-style config:

```json
{
  "mcpServers": {
    "git-statistics": {
      "command": "go",
      "args": ["run", "./cmd/server", "-mcp", "-config", "config.yaml", "-db", "git-statistics.db"],
      "cwd": "C:/projects/git_statistics"
    }
  }
}
```

or, pointing at a built binary instead of `go run`:

```json
{
  "mcpServers": {
    "git-statistics": {
      "command": "C:/projects/git_statistics/server.exe",
      "args": ["-mcp", "-config", "config.yaml", "-db", "git-statistics.db"]
    }
  }
}
```

The same shape applies to VS Code's `mcp.json`/Copilot MCP server settings or Claude Desktop's config — only the surrounding file/key names differ, the `command`/`args`/`cwd` triple is what matters. Because the server communicates over stdio (`&mcp.StdioTransport{}` in `runMCPServer`), the client must launch it as a subprocess rather than connecting to a network port — there's nothing listening on `:8080` in this mode.

**6. Read-only against already-synced data.**
`runMCPServer` only constructs the MCP server, registers tools/prompts, and serves stdio — it never touches `bitbucket.NewClient` or `ingest.Syncer`, both of which are only constructed in the non-MCP branch of `main()`. Every tool call queries the already-open `*storage.Store` (the same SQLite file passed via `-db`). Practically: if you want fresh data, you still have to run the binary in normal HTTP mode (or however sync is currently triggered) first to populate the database; the MCP server mode will never reach out to Bitbucket or mutate the database itself, no matter what an agent asks it to do.

## Why This Matters

The dashboard is built for a human scanning charts; an agent doing investigative work needs structured, machine-consumable answers to narrow questions, often as one step in a larger task ("summarize how this sprint went," "check this PR before commenting on it," "tell me which author has the most open PRs in the billing-service repo"). Exposing the metrics engine as MCP tools means an agent can call `pr_status_by_author` directly and get back typed JSON it can reason over, instead of either screen-scraping the dashboard's HTML or hand-rolling SQL against the SQLite file. The curated prompts go a step further: they encode the same multi-tool investigation patterns a human would perform by clicking through several dashboard panels in sequence, so an agent gets the right sequence and scoping "for free" rather than guessing which combination of tools answers a given question. This turns the app from a destination you visit into a data source other tools and agents can pull from directly.

## When to Apply

Reach for `-mcp` mode when the consumer is an agent or automated tool that needs structured answers to specific metric questions — not a person who wants to browse trends visually; for that, the HTTP dashboard (`go run ./cmd/server -config config.yaml -db git-statistics.db`, no `-mcp`) is still the right mode, since it's the only mode that also runs the sync scheduler. The two modes are mutually exclusive within a single process — pick one binary invocation for syncing/dashboard duty and a separate invocation (potentially against the same `-db` file) for MCP duty.

When extending the metrics surface — e.g. adding a new metric function to `internal/metrics` — follow the same registration pattern already established: add a new `XxxOutput` wrapper struct in `internal/mcpserver/tools.go`, call `mcp.AddTool` with a `mcp.Tool{Name, Description}` and a handler that parses the filter via `parseFilterFromTool`, calls the new `metrics.Xxx(store, filter)` function, and returns the wrapped output. If the new metric is commonly used together with existing ones in an investigation workflow, also consider adding or extending a prompt in `internal/mcpserver/prompts.go` so agents get the scripted multi-tool sequence rather than having to discover the right combination themselves.

## Examples

Starting the server for a Claude Code or VS Code Copilot session to attach to:

```
go run ./cmd/server -mcp -config config.yaml -db git-statistics.db
```

A representative agent interaction once connected: an agent investigating "how is the payments-service repo doing for Jane this sprint" would either invoke the `investigate_progress` prompt with `repo="payments-service", author="jane.doe"` — which hands back instructions to call `pr_status_by_author`, then `delivery_flow`, then `commits_per_author`, each scoped to that repo+author — or skip straight to calling `pr_status_by_author` itself with:

```json
{ "repo": "payments-service", "author": "jane.doe" }
```

which returns something shaped like `{"rows": [{"author": "Jane Doe", "open": 2, "merged": 14, "declined": 1, "superseded": 0}]}` (the exact `PRStatusCountRow` fields come from `internal/metrics`), giving the agent a precise, structured answer it can fold into a larger summary without ever touching the dashboard UI.

## Related
- [docs/solutions/workflow-issues/verify-plan-assumed-apis-against-real-source.md](../workflow-issues/verify-plan-assumed-apis-against-real-source.md) — the PR-status tool's underlying metric (`PRStatusByAuthor`/`PRStatusByRepository`) groups by `domain.PullRequest.State` and deliberately does *not* filter on `MergedAt` nullability, following the same real-API-verification discipline this doc established for the same `internal/metrics`/`internal/storage` area.
- [docs/SETUP.md](../../SETUP.md) — canonical run instructions; now includes a short "Run in MCP mode" section pointing back here for full detail.
