# Progress: MCP Data Interface Plan

Plan: docs/plans/2026-06-27-001-feat-mcp-data-interface-plan.md

Each implementing subagent appends/updates its own section below when it starts, halts, or finishes. Status values: `not started`, `in progress`, `halted`, `done`.

## U1. MCP server mode skeleton in cmd/server

- Status: done
- Dependencies: none
- Completed: Added `github.com/modelcontextprotocol/go-sdk v1.6.1` to go.mod. Added `-mcp` bool flag to cmd/server/main.go; when set, runs `runMCPServer(store)` instead of HTTP listener. Created cmd/server/mcpserver.go with `runMCPServer()` function that constructs `mcp.Server` with basic Implementation info and runs it on `mcp.StdioTransport()` over stdio. Added cmd/server/mcpserver_test.go to verify function construction. `go build ./...` and `go test ./...` both pass with all 11 packages.

## U2. PR status breakdown metric

- Status: done
- Dependencies: none
- Completed: Added `PRStatusByAuthor` and `PRStatusByRepository` functions to `internal/metrics/metrics.go`, along with shared helper `prStatusFromGroups`. All four PR states (OPEN, MERGED, DECLINED, SUPERSEDED) are reported per group with zeros for absent states. Added 11 comprehensive tests covering mixed states, single-state groups, empty results, and filtering by repo/author/date range. All tests pass; `go build ./internal/metrics` succeeds.

## U3. MCP tools for existing dashboard metrics

- Status: done
- Dependencies: U1
- Completed: Created `internal/mcpserver/tools.go` with `RegisterToolsDashboardMetrics` function registering 7 MCP tools wrapping the existing dashboard metrics (DeliveryFlow, SummaryStats, Distributions, ChurnHotspots, CommitsPerAuthor, BreakdownByRepository, BreakdownByAuthor). Added output wrapper types to satisfy MCP's requirement that output schemas be objects. Each tool accepts repo/author/from/to filter parameters (dates in "2006-01-02" format), parses them using the same logic as the web handlers, and returns results as JSON. Added comprehensive tests in `internal/mcpserver/tools_test.go` covering: registration success, individual tool execution against seeded test stores, empty/omitted filter params, and invalid date handling. Wired tool registration calls into `cmd/server/mcpserver.go` (already completed in U1). `go build ./...` and `go test ./...` both pass with all tests successful.

## U4. MCP tool for PR status breakdown

- Status: done
- Dependencies: U1, U2
- Completed: Extended `internal/mcpserver/tools.go` (created by U3) to add `RegisterToolsPRStatus` function registering two new MCP tools: `pr_status_by_author` and `pr_status_by_repo`. These tools wrap the `PRStatusByAuthor` and `PRStatusByRepository` metric functions from U2, accepting the same repo/author/from/to filter parameters and returning PR state counts (open, merged, declined, superseded) as JSON via output wrapper types. Added comprehensive tests in `internal/mcpserver/tools_test.go` covering: tool registration, seeded test data with mixed PR states, filtering by repo/author/date range, and edge cases (empty results, single-state groups, zero counts for absent states). Wired tool registration into `cmd/server/mcpserver.go`. `go build ./...` and `go test ./...` both pass with all tests successful.

## U5. Curated MCP prompts

- Status: done
- Dependencies: U3, U4
- Completed: Created `internal/mcpserver/prompts.go` with `RegisterPrompts` function registering three MCP prompts: `investigate_progress` (required params: repo; optional: author), `team_activity_summary` (optional params: repo, from, to), and `pr_health_check` (required param: repo). Each prompt returns a GetPromptResult with a message sequence directing the agent to call specific tools in order to complete the investigation workflow. Added comprehensive tests in `internal/mcpserver/prompts_test.go` covering: nil server error handling, each prompt with valid and omitted optional parameters, well-formed prompt message structures, and prompt descriptions. Wired RegisterPrompts call into `cmd/server/mcpserver.go`. `go build ./...` and `go test ./...` both pass with all 12 test packages successful.
