---
date: 2026-06-27
type: feat
origin: docs/brainstorms/2026-06-27-mcp-data-interface-requirements.md
---

# feat: MCP Data Interface for Agents

## Summary

Add an MCP stdio server mode to the existing `cmd/server` binary that exposes the app's metrics (delivery flow, churn, author activity, repo/author breakdowns) plus a new PR-status-by-author/repo metric as MCP tools, with curated MCP prompts bundling them into investigation workflows, so external agents can query this data without going through the web UI.

## Problem Frame

The app already computes engineering-flow metrics in `internal/metrics`, but the only access path is the HTML dashboard in `internal/web`. An agent investigating feature progress (e.g. "how is this PR going for each team member") has no programmatic way to pull repo/author/PR data. Separately, no existing metric reports PR status counts (open/merged/declined) per author or repo — `BreakdownByRepository`/`BreakdownByAuthor` in [internal/metrics/metrics.go](internal/metrics/metrics.go) only aggregate timing for merged PRs. (See origin: docs/brainstorms/2026-06-27-mcp-data-interface-requirements.md)

## Key Technical Decisions

- **Official `github.com/modelcontextprotocol/go-sdk`, not `mark3labs/mcp-go`.** Confirmed via web search ([go-sdk repo](https://github.com/modelcontextprotocol/go-sdk)) as the actively maintained, spec-complete (MCP 2025-11-25) Go SDK, maintained in collaboration with Google. It supports stdio transport (`mcp.NewServer`, `mcp.AddTool`, `StdioTransport`) and a prompts primitive, matching both required capabilities directly. `mark3labs/mcp-go` is a viable community alternative but unofficial.
- **MCP mode on the existing `cmd/server` binary, not a separate binary.** A `-mcp` flag switches `cmd/server` into MCP stdio mode instead of starting the HTTP listener, reusing the same `-config`/`-db` flags and `storage.Open`/`config.Load` wiring already in [cmd/server/main.go](cmd/server/main.go). One binary to build and deploy, at the cost of `main` branching on mode (see origin).
- **Tools take the same filter shape as the web UI**, not a new schema. Each tool's parameters mirror `storage.Filter` (repo, author, from, to) the way [internal/web/handlers.go](internal/web/handlers.go)'s `parseFilter` already does, so there is one mental model for "filter this data" across the dashboard and the MCP surface.
- **PR status breakdown groups by `domain.PullRequest.State` client-side**, the same way existing code already branches on `MergedAt` nullability (per docs/solutions/workflow-issues/verify-plan-assumed-apis-against-real-source.md) — `storage.Filter` has no state column, so this is computed over `ListPullRequests` results in Go, not SQL.
- **Discoverability via both self-describing tools and curated prompts** (carried from origin): tool names/descriptions/schemas cover ad-hoc discovery; a small set of MCP prompts bundle several tool calls into named investigation workflows.

## Output Structure

```
internal/mcpserver/
  tools.go        # MCP tool registrations wrapping internal/metrics functions
  tools_test.go
  prompts.go      # curated MCP prompts bundling tool calls
  prompts_test.go
```

## Requirements

**MCP server mode**
- R1. `cmd/server` supports a `-mcp` flag that, when set, opens the configured store and runs an MCP server over stdio instead of starting the HTTP listener. (origin R5)

**Tool surface**
- R2. The MCP server exposes one tool per existing metric capability — delivery flow, summary stats, percentile distributions, churn hotspots, author commit activity, repo breakdown, author breakdown — each accepting repo/author/date-range filter parameters equivalent to `storage.Filter`. (origin R1)
- R3. The MCP server exposes a new tool returning PR counts grouped by state (open/merged/declined/superseded), broken down by author and by repo. (origin R2)
- R4. Every tool has a name, description, and parameter schema sufficient for an agent to select it from `tools/list` without external documentation. (origin R3)

**Discoverability**
- R5. The MCP server defines curated MCP prompts, each bundling multiple tool calls into a named investigation workflow (feature/repo progress, team activity summary, PR health check). (origin R4)

**Scope and data freshness**
- R6. No tool triggers a live Bitbucket API call or new ingestion; all tools read from the already-synced SQLite store. (origin R5)

## Implementation Units

### U1. MCP server mode skeleton in cmd/server

**Goal:** Add the MCP SDK dependency and a `-mcp` flag that starts an empty MCP server over stdio (no tools yet), reusing existing config/store wiring.

**Requirements:** R1

**Dependencies:** None

**Files:**
- `go.mod`, `go.sum` (add `github.com/modelcontextprotocol/go-sdk`)
- `cmd/server/main.go` (add `-mcp` flag, branch before starting the HTTP listener)
- `cmd/server/mcpserver.go` (new: builds `*mcp.Server`, registers tools/prompts, runs `StdioTransport`)

**Approach:** Parse a `-mcp` bool flag alongside the existing `-config`/`-db` flags. When set, after `config.Load` and `storage.Open` succeed, call a new `runMCPServer(store *storage.Store) error` in `mcpserver.go` instead of building the HTTP mux, and return its result from `main` instead of starting `http.ListenAndServe`. `runMCPServer` constructs `mcp.NewServer(...)`, calls into the tool/prompt registration functions added in U3-U5 (stubbed as a no-op call in this unit), and runs it on `mcp.NewStdioTransport()` (or the SDK's equivalent constructor — confirm exact name against the installed module).

**Patterns to follow:** Existing flag parsing and `log.Fatalf` error handling in [cmd/server/main.go](cmd/server/main.go).

**Test scenarios:**
- Happy path: running the binary with `-mcp` and a valid config/db starts without error and the process responds to an MCP `initialize` request (verified via the SDK's in-memory transport or a short-lived subprocess test, matching whatever pattern the SDK's own examples use).
- Error path: invalid `-config` or `-db` path still produces the same `log.Fatalf` behavior as the HTTP path today (no new error handling needed, just confirm the existing flow runs before the mode branch).
- Test expectation for `main.go` flag wiring: none beyond a build check — `main` itself has no existing test coverage in this repo; cover `runMCPServer`'s server construction in `cmd/server/mcpserver_test.go` instead.

**Verification:** `go build ./...` succeeds; `go run ./cmd/server -mcp -config <test config> -db <temp db>` starts and accepts a stdio `initialize` handshake without exiting.

---

### U2. PR status breakdown metric

**Goal:** Add a new `internal/metrics` function computing PR counts grouped by `State`, broken down by author and by repo, for the PR-status MCP tool to wrap.

**Requirements:** R3

**Dependencies:** None

**Files:**
- `internal/metrics/metrics.go` (add `PRStatusByAuthor` and `PRStatusByRepository`, or a shared `PRStatusBreakdown` helper covering both — implementer's call once the shape of `BreakdownRow`-style grouping is in front of them)
- `internal/metrics/metrics_test.go`

**Approach:** Reuse `store.ListPullRequests(f)` (no new SQL). Group results by `AuthorID`/`RepoSlug` the same way `breakdownFromGroups` already does in [internal/metrics/metrics.go:249](internal/metrics/metrics.go), but tally by `State` value instead of computing timing aggregates — for each group, return counts per state (`OPEN`, `MERGED`, `DECLINED`, `SUPERSEDED`) rather than `AvgCycleTime` etc. Do not filter on `MergedAt` nullability the way `DeliveryFlow` does — every PR, regardless of state, counts here.

**Patterns to follow:** `BreakdownByAuthor`/`BreakdownByRepository` grouping and `store.ListAuthors()` display-name lookup in [internal/metrics/metrics.go:212-246](internal/metrics/metrics.go).

**Test scenarios:**
- Happy path: a mix of OPEN, MERGED, and DECLINED PRs across two authors and two repos produces correct per-state counts for each author and each repo.
- Edge case: an author or repo with PRs only in one state still reports zero, not an absent key, for the other states (so an agent doesn't have to special-case missing keys).
- Edge case: empty PR set for a filter returns an empty/zero result without error.
- Integration: filtering by `storage.Filter.RepoSlug`/`AuthorID`/date range narrows the grouped counts the same way it narrows `ListPullRequests` elsewhere.

**Verification:** `go test ./internal/metrics/...` passes with new cases covering all four states.

---

### U3. MCP tools for existing dashboard metrics

**Goal:** Register one MCP tool per existing dashboard metric function, with parameters mirroring `storage.Filter`.

**Requirements:** R2, R4

**Dependencies:** U1

**Files:**
- `internal/mcpserver/tools.go`
- `internal/mcpserver/tools_test.go`

**Approach:** One `mcp.AddTool` registration per metric function: `DeliveryFlow`, `SummaryStats`, `Distributions`, `ChurnHotspots`, `CommitsPerAuthor`, `BreakdownByRepository`, `BreakdownByAuthor`. Each tool's input struct has `repo`, `author`, `from`, `to` string fields (dates as `"2006-01-02"`, matching [internal/web/handlers.go](internal/web/handlers.go)'s `parseFilter`), converted to a `storage.Filter` before calling the wrapped function, with results marshaled to JSON in the tool response. Give each tool a description naming the dashboard it mirrors (e.g. "Returns PR cycle time, time-to-first-review, and time-in-review for merged pull requests, optionally filtered by repo/author/date range — same data as the Delivery Flow dashboard.") so R4's self-description requirement holds without external docs.

**Patterns to follow:** `parseFilter` in [internal/web/handlers.go:11](internal/web/handlers.go) for filter parsing; the metric function signatures themselves for what each tool wraps.

**Test scenarios:**
- Happy path: each tool, given valid filter params against a seeded test store, returns the same data its corresponding `internal/metrics` function returns directly.
- Edge case: omitted/empty filter params (no repo, no author, no dates) return unfiltered results, matching `parseFilter`'s behavior when query params are absent.
- Error path: an invalid date string in `from`/`to` returns a tool error rather than panicking, mirroring `parseFilter`'s `http.StatusBadRequest` behavior for the web handlers.
- Integration: tool registration on the constructed `*mcp.Server` is reachable via `tools/list` and each tool's schema validates the expected parameter shape.

**Verification:** `go test ./internal/mcpserver/...` passes; a manual `tools/list` call against a running `-mcp` process lists all seven tools with non-empty descriptions.

---

### U4. MCP tool for PR status breakdown

**Goal:** Register an MCP tool wrapping U2's PR-status metric function.

**Requirements:** R3, R4

**Dependencies:** U1, U2

**Files:**
- `internal/mcpserver/tools.go` (extend)
- `internal/mcpserver/tools_test.go` (extend)

**Approach:** Same registration pattern as U3 — `repo`/`author`/`from`/`to` filter params, calls into U2's metric function, JSON-marshaled response — with a description naming the per-state breakdown explicitly (open/merged/declined/superseded counts by author and repo) since this is new data not shown on any existing dashboard.

**Patterns to follow:** U3's tool-registration shape, for consistency.

**Test scenarios:**
- Happy path: tool returns matching per-state counts for a seeded mix of PR states, equivalent to calling U2's function directly.
- Edge case: filtering to an author/repo with no PRs returns a valid empty result, not an error.
- Integration: tool is listed in `tools/list` alongside the U3 tools with a description distinct enough that an agent can tell it apart from the timing-based breakdowns.

**Verification:** `go test ./internal/mcpserver/...` passes; manual `tools/list` shows the new tool.

---

### U5. Curated MCP prompts

**Goal:** Define MCP prompts bundling multiple tools into named investigation workflows.

**Requirements:** R5

**Dependencies:** U3, U4

**Files:**
- `internal/mcpserver/prompts.go`
- `internal/mcpserver/prompts_test.go`

**Approach:** Register prompts via the SDK's prompts primitive (confirm exact registration call, e.g. `server.AddPrompt`, against the installed module version). At minimum: `investigate_progress` (params: repo, optional author — guides the agent through PR status, delivery flow, and recent author activity for that scope), `team_activity_summary` (params: optional repo, date range — guides through author activity and breakdowns), `pr_health_check` (params: repo — guides through delivery flow summary, distributions, and PR status breakdown). Each prompt returns a message sequence directing the agent which tools to call and in what order, per the SDK's prompt-message shape — prompts orchestrate tool calls, they do not duplicate tool logic.

**Patterns to follow:** U3/U4 for how filter parameters are shaped; the origin document's "Discoverability" key decision for why prompts exist alongside self-describing tools.

**Test scenarios:**
- Happy path: each prompt, given valid params, returns a well-formed prompt response listing the tools it expects the agent to call.
- Edge case: omitted optional params (e.g. no author for `investigate_progress`) still produce a valid prompt response scoped to what was given.
- Integration: prompts are listed via `prompts/list` alongside the tools from U3/U4, distinguishable by name and description.

**Verification:** `go test ./internal/mcpserver/...` passes; manual `prompts/list` against a running `-mcp` process shows all three prompts.

---

## Scope Boundaries

### Deferred for later
- A generalized/flexible ad-hoc query tool beyond the fixed per-metric tool set. (origin)
- Live Bitbucket API fallback for questions the locally-synced data doesn't cover. (origin)

### Deferred to Follow-Up Work
- None identified — no tangential cleanup surfaced during planning.

## Open Questions

### Deferred to Implementation
- Exact SDK constructor/method names for stdio transport and prompt registration (`mcp.NewStdioTransport` vs. similar) — confirm against the installed `github.com/modelcontextprotocol/go-sdk` version once added to `go.mod`, per the docs/solutions guidance on verifying assumed APIs against real source.
- Whether `PRStatusByAuthor`/`PRStatusByRepository` are two functions or one shared helper — decide once the grouping code is in front of the implementer (U2).
- Whether the PR-status tool needs filter dimensions beyond repo/author/date (e.g. minimum age of an open PR) — origin left this open; no signal yet that it's needed for the target investigation scenario.

## Sources / Research

- [modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk) — official Go MCP SDK, maintained with Google, spec-complete for MCP 2025-11-25, supports stdio transport and prompts. Confirms KTD1.
- [docs/solutions/workflow-issues/verify-plan-assumed-apis-against-real-source.md](docs/solutions/workflow-issues/verify-plan-assumed-apis-against-real-source.md) — prior incident in this repo from assuming generic `Store`/`FilterParams` types instead of the real concrete `*storage.Store`/`storage.Filter`; directly informs KTD3 and KTD4 above.
