---
date: 2026-06-27
topic: mcp-data-interface
---

# MCP Data Interface for Agents

## Summary

Add an MCP server that exposes this app's existing dashboard data — delivery flow, churn, author activity, repo/author breakdowns — and a new PR-status-by-author/repo view as tools an external agent (e.g. a VS Code Copilot session investigating feature progress) can query directly, plus a few curated MCP prompts that bundle those tools into common investigation workflows.

## Problem Frame

The app already collects and computes useful engineering-flow data, but that data is only reachable by clicking through dashboard pages. An agent investigating something concrete — "how is this feature's PR going for each team member" — has no way to pull repo/author/PR-status context without a human relaying it through the UI. The app also has no current view of PR status counts per author or repo; existing breakdowns ([internal/metrics/metrics.go](internal/metrics/metrics.go)) only aggregate timing for *merged* PRs.

## Key Decisions

- **Mirror existing dashboard functions as MCP tools, not a generalized query engine.** Each existing metric function (`DeliveryFlow`, `ChurnHotspots`, `CommitsPerAuthor`, `SummaryStats`, `Distributions`, `BreakdownByRepository`, `BreakdownByAuthor`) becomes one MCP tool with the same repo/author/date filters the web UI already uses. A more flexible/generic query tool was considered and deferred — the fixed function set is close to zero new logic today, and a generalized engine is easier to justify once real usage patterns are known.
- **Add a new PR-status-by-author/repo capability.** Nothing in the app currently surfaces open/merged/declined counts per author or repo; this is new logic, not a wrapper, but it directly serves the target investigation scenario.
- **Discoverability uses both self-describing tools and curated MCP prompts.** Tool names/descriptions/schemas cover raw discovery; a small set of MCP prompts (e.g. "investigate feature/repo progress," "team activity summary," "PR health check") bundle multiple tool calls into a guided workflow for the case where the agent (or the human directing it) doesn't know what to ask yet.
- **v1 is read-only against already-synced SQLite data.** No new ingestion logic, no live Bitbucket calls.

## Requirements

**Tool surface**
- R1. The MCP server exposes one tool per existing metric capability (delivery flow, summary stats, percentile distributions, churn hotspots, author commit activity, repo breakdown, author breakdown), each accepting the same repo/author/date-range filter the web UI uses (`storage.Filter`).
- R2. The MCP server exposes a new PR-status tool returning open/merged/declined counts grouped by author and by repo.
- R3. Every tool's name, description, and parameter schema is self-explanatory enough that an agent can pick the right tool from `tools/list` alone, without out-of-band documentation.

**Discoverability**
- R4. The MCP server defines a small set of curated MCP prompts that each bundle multiple tool calls into a named investigation workflow (e.g. feature/repo progress, team activity summary, PR health check).

**Scope and data freshness**
- R5. All tools read only from data already present in the local SQLite store; no tool triggers a live Bitbucket API call or new ingestion.

## Scope Boundaries

**Deferred for later**
- A generalized/flexible ad-hoc query tool beyond the fixed per-metric tool set.
- Live Bitbucket API fallback for questions the locally-synced data doesn't cover.

## Dependencies / Assumptions

- Assumes "agents" means external MCP clients (VS Code Copilot, Claude Code, etc.) connecting to a server process for this app — not a change to the app's own internal architecture beyond adding that server.
- The PR-status tool (R2) depends on `domain.PullRequest.State` already being populated for all synced PRs ([internal/domain/domain.go](internal/domain/domain.go)) — confirmed present in the existing schema.

## Outstanding Questions

**Deferred to Planning**
- Where the MCP server process lives (new `cmd/` binary vs. mode of the existing server) and which Go MCP library to use.
- Exact tool and prompt names/schemas.
- Whether the PR-status tool needs its own filter dimensions beyond repo/author/date (e.g. minimum age of an open PR).
